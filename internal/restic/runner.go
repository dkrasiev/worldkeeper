// Package restic stores world snapshots in a restic repository by running
// the restic command-line tool, which must be installed separately.
package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// MinVersion is the oldest supported restic: 0.17 added the snapshot summary
// in JSON output and the dedicated exit codes we rely on.
var MinVersion = [3]int{0, 17, 0}

var (
	ErrNotInstalled  = errors.New("restic is not installed or not found in PATH")
	ErrRepoMissing   = errors.New("restic repository does not exist")
	ErrLocked        = errors.New("restic repository is locked")
	ErrWrongPassword = errors.New("wrong restic repository password")
	ErrNoPassword    = errors.New("restic repository password is not set")
	ErrIncomplete    = errors.New("snapshot created, but some files could not be read")
)

// Runner invokes the restic binary against one repository.
type Runner struct {
	Binary       string // path or name in PATH; empty means "restic"
	Repo         string
	PasswordFile string                 // used instead of Password when set
	Password     func() (string, error) // e.g. read from the OS keychain
}

// command builds a restic invocation. On Windows the release build has no
// console of its own, so without hideConsole every restic run would pop up
// a terminal window.
func (r *Runner) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, r.binary(), args...)
	hideConsole(cmd)
	return cmd
}

func (r *Runner) binary() string {
	if r.Binary == "" {
		return "restic"
	}
	return r.Binary
}

// Version returns the installed restic version, e.g. "0.18.1".
func (r *Runner) Version(ctx context.Context) (string, error) {
	out, err := r.command(ctx, "version").Output()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) || errors.Is(err, os.ErrNotExist) {
			return "", ErrNotInstalled
		}
		return "", fmt.Errorf("restic version: %w", err)
	}
	m := versionRe.FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("unexpected `restic version` output: %q", out)
	}
	if !atLeast(m[1], MinVersion) {
		return m[1], fmt.Errorf("restic %s is too old, need %d.%d.%d or newer", m[1], MinVersion[0], MinVersion[1], MinVersion[2])
	}
	return m[1], nil
}

// errorMessage extracts the human message from stderr. With --json, restic
// reports fatal errors as {"message_type":"exit_error","message":"..."}.
func errorMessage(stderr string) string {
	for _, line := range strings.Split(stderr, "\n") {
		var m struct {
			Type    string `json:"message_type"`
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(line), &m) == nil && m.Type == "exit_error" && m.Message != "" {
			return strings.TrimSpace(strings.TrimPrefix(m.Message, "Fatal: "))
		}
	}
	return strings.TrimSpace(stderr)
}

var versionRe = regexp.MustCompile(`restic (\d+\.\d+\.\d+)`)

func atLeast(v string, min [3]int) bool {
	parts := strings.SplitN(v, ".", 3)
	for i := 0; i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		if n != min[i] {
			return n > min[i]
		}
	}
	return true
}

// Run executes restic with args in dir and returns stdout. Exit codes are
// mapped to the package errors; stderr is included in other errors.
func (r *Runner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if r.Repo == "" {
		return nil, errors.New("restic repository is not configured")
	}
	env, err := r.env()
	if err != nil {
		return nil, err
	}
	cmd := r.command(ctx, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return stdout.Bytes(), nil
	case errors.As(err, &exit):
		return stdout.Bytes(), exitError(exit.ExitCode(), stderr.String())
	case errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist):
		return nil, ErrNotInstalled
	default:
		return nil, err
	}
}

// env passes the repository and password through the environment, so the
// password never shows up in the process list. RESTIC_* variables from the
// parent are dropped; others (e.g. AWS_* for S3) are kept.
func (r *Runner) env() ([]string, error) {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "RESTIC_") {
			env = append(env, kv)
		}
	}
	env = append(env, "RESTIC_REPOSITORY="+r.Repo)
	switch {
	case r.PasswordFile != "":
		env = append(env, "RESTIC_PASSWORD_FILE="+r.PasswordFile)
	case r.Password != nil:
		pw, err := r.Password()
		if err != nil {
			return nil, err
		}
		if pw == "" {
			return nil, ErrNoPassword
		}
		env = append(env, "RESTIC_PASSWORD="+pw)
	default:
		return nil, ErrNoPassword
	}
	return env, nil
}

func exitError(code int, stderr string) error {
	msg := errorMessage(stderr)
	if len(msg) > 2000 {
		msg = "…" + msg[len(msg)-2000:]
	}
	var base error
	switch code {
	case 3:
		base = ErrIncomplete
	case 10:
		base = ErrRepoMissing
	case 11:
		base = ErrLocked
	case 12:
		base = ErrWrongPassword
	default:
		return fmt.Errorf("restic failed (exit %d): %s", code, msg)
	}
	if msg == "" {
		return base
	}
	return &exitErr{base: base, msg: msg}
}

// exitErr shows restic's own message, which is more specific than base,
// while still matching base with errors.Is.
type exitErr struct {
	base error
	msg  string
}

func (e *exitErr) Error() string { return "restic: " + e.msg }
func (e *exitErr) Unwrap() error { return e.base }
