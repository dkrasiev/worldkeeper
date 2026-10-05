// Command worldkeeper backs up Minecraft worlds automatically when the game
// closes them, and serves a local web UI to browse worlds and restore saves.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/api"
	"github.com/dkrasiev/worldkeeper/internal/app"
	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/discovery"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/web"
)

var version = "dev"

const usage = `worldkeeper %s — automatic Minecraft world backups

Usage:
  worldkeeper [flags]                 run the watcher and web UI (default)
  worldkeeper list                    list discovered worlds
  worldkeeper backup <world-id>       save a manual snapshot now
  worldkeeper version

Flags:
`

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	defaultCfg, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfgPath := flag.String("config", defaultCfg, "path to config.json")
	noBrowser := flag.Bool("no-browser", false, "do not open the web UI on start")
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

	cfg, err := config.Open(*cfgPath)
	if err != nil {
		return fmt.Errorf("config %s: %w", *cfgPath, err)
	}
	a := app.New(cfg, discovery.HostEnv(), log)

	switch flag.Arg(0) {
	case "", "serve":
		return serve(a, log, !*noBrowser)
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

func serve(a *app.App, log *slog.Logger, openUI bool) error {
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
		if openUI && alreadyRunning(cfg.Listen) {
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
	go a.Watch(ctx)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	log.Info("worldkeeper started", "version", version, "ui", url, "storage", cfg.StorageDir)
	if openUI {
		if err := openBrowser(url); err != nil {
			log.Warn("could not open browser", "err", err)
		}
	}
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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
