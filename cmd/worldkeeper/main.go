// Command worldkeeper backs up Minecraft worlds automatically when the game
// closes them, and serves a local web UI to browse worlds and restore saves.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/api"
	"github.com/dkrasiev/worldkeeper/internal/app"
	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/discovery"
	"github.com/dkrasiev/worldkeeper/internal/secrets"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/tray"
	"github.com/dkrasiev/worldkeeper/web"
)

var version = "dev"

const usage = `worldkeeper %s — automatic Minecraft world backups

Usage:
  worldkeeper [flags]                 run the watcher, tray icon and web UI (default)
  worldkeeper list                    list discovered worlds
  worldkeeper backup <world-id>       save a manual snapshot now
  worldkeeper version

Flags:
`

func main() {
	attachConsole() // Windows release builds have no console of their own
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	defaultCfg, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfgPath := flag.String("config", defaultCfg, "path to config.json")
	noBrowser := flag.Bool("no-browser", false, "do not open the web UI on start")
	noTray := flag.Bool("no-tray", false, "run without the system tray icon")
	label := flag.String("label", "", "label for `backup`")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), usage, version)
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.Arg(0) == "version" {
		fmt.Println(version)
		return nil
	}

	_, statErr := os.Stat(*cfgPath)
	firstRun := errors.Is(statErr, os.ErrNotExist)
	cfg, err := config.Open(*cfgPath)
	if err != nil {
		return fmt.Errorf("config %s: %w", *cfgPath, err)
	}

	serving := flag.Arg(0) == "" || flag.Arg(0) == "serve"
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if serving {
		// Started from the tray or at login there is no terminal to read.
		logPath := filepath.Join(filepath.Dir(*cfgPath), "worldkeeper.log")
		if f, err := openLog(logPath); err == nil {
			defer f.Close()
			log = slog.New(slog.NewTextHandler(io.MultiWriter(os.Stderr, f), nil))
		}
	}
	a := app.New(cfg, secrets.Keyring{}, discovery.HostEnv(), log)

	switch flag.Arg(0) {
	case "", "serve":
		useTray := !*noTray && tray.Available && trayHostAvailable()
		// With a tray icon the UI is one click away; only open it unasked on
		// the very first start, so login autostart does not pop a browser.
		openUI := !*noBrowser && (!useTray || firstRun)
		return serve(a, log, openUI, useTray)
	case "list":
		return list(a)
	case "backup":
		if flag.NArg() < 2 {
			return errors.New("usage: worldkeeper backup <world-id>")
		}
		snap, err := a.Backup(flag.Arg(1), app.BackupOptions{Kind: snapshot.KindManual, Label: *label})
		if err != nil {
			return err
		}
		fmt.Println("saved", snap.ID)
		return nil
	default:
		flag.Usage()
		return fmt.Errorf("unknown command %q", flag.Arg(0))
	}
}

func list(a *app.App) error {
	ov, err := a.Overview()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tVERSION\tLAST PLAYED\tSNAPSHOTS\tSTATE")
	for _, w := range ov.Worlds {
		state := ""
		if w.InUse {
			state = "in game"
		} else if w.Changed {
			state = "not backed up"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\n", w.ID, w.Name, w.GameVersion,
			w.LastPlayed.Local().Format("2006-01-02 15:04"), w.Snapshots, state)
	}
	return tw.Flush()
}

func serve(a *app.App, log *slog.Logger, openUI, useTray bool) error {
	cfg := a.Config.Get()
	host, _, err := net.SplitHostPort(cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen address %q: %w", cfg.Listen, err)
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("refusing to listen on %q: only loopback addresses are allowed", cfg.Listen)
	}
	url := fmt.Sprintf("http://%s/?token=%s", cfg.Listen, cfg.Token)

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		// Most likely worldkeeper is already running: just show its UI.
		if alreadyRunning(cfg.Listen) {
			log.Info("worldkeeper is already running, opening UI")
			return openBrowser(url)
		}
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Handler:           api.New(a, web.FS()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		stop()
	}()
	go a.Watch(ctx)

	storage := cfg.StorageDir
	if cfg.Engine == config.EngineRestic {
		storage = "restic:" + cfg.Restic.Repo
	}
	log.Info("worldkeeper started", "version", version, "ui", url, "storage", storage)
	if openUI {
		if err := openBrowser(url); err != nil {
			log.Warn("could not open browser", "err", err)
		}
	}

	// The tray must own the main goroutine (macOS requires it); it returns
	// when the user picks Quit or ctx is cancelled.
	if useTray {
		ctrl := &trayController{app: a, url: url, log: log}
		if err := tray.Run(ctx, ctrl, tray.Locale(), log); err != nil {
			log.Warn("system tray unavailable, running without it", "err", err)
			<-ctx.Done()
		}
		stop()
	} else {
		<-ctx.Done()
	}

	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	select {
	case err := <-serveErr:
		return err
	default:
		log.Info("worldkeeper stopped")
		return nil
	}
}

// trayHostAvailable reports whether a tray can be shown: on Linux the icon
// lives on the D-Bus session bus, which headless systems do not have.
func trayHostAvailable() bool {
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" || runtime.GOOS == "netbsd" {
		return os.Getenv("DBUS_SESSION_BUS_ADDRESS") != ""
	}
	return true
}

// openLog appends to the log file, starting over once it grows past 5 MB.
func openLog(path string) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if fi, err := os.Stat(path); err == nil && fi.Size() > 5<<20 {
		flags |= os.O_TRUNC
	}
	return os.OpenFile(path, flags, 0o600)
}

type trayController struct {
	app *app.App
	url string
	log *slog.Logger
}

func (c *trayController) OpenUI() {
	if err := openBrowser(c.url); err != nil {
		c.log.Warn("could not open browser", "err", err)
	}
}

func (c *trayController) BackupNow() tray.BackupResult {
	saved, skipped, failed := c.app.BackupChanged()
	return tray.BackupResult{Saved: saved, Skipped: skipped, Failed: failed}
}

func (c *trayController) AutoBackup() bool { return c.app.Config.Get().AutoBackup }

func (c *trayController) SetAutoBackup(on bool) error {
	_, err := c.app.Config.Update(func(cfg *config.Config) error { cfg.AutoBackup = on; return nil })
	return err
}

func (c *trayController) Status() tray.Status {
	h := c.app.Health()
	return tray.Status{LastBackup: h.LastBackup, FailingWorld: h.FailingWorld, Error: h.Error}
}

func alreadyRunning(addr string) bool {
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://" + addr + "/api/overview")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusUnauthorized // our guard answered
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
