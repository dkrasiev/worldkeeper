//go:build unix

package lock

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// Locks are per process, so the "game" holding the lock is a child process:
// this test binary re-executed in helper mode.
func TestHelperHoldLock(t *testing.T) {
	path := os.Getenv("WK_HOLD_LOCK")
	if path == "" {
		t.Skip("helper process only")
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		os.Exit(2)
	}
	lk := unix.Flock_t{Type: unix.F_WRLCK, Whence: io.SeekStart}
	if err := unix.FcntlFlock(f.Fd(), unix.F_SETLK, &lk); err != nil {
		os.Exit(3)
	}
	os.Stdout.WriteString("locked\n")
	_, _ = io.Copy(io.Discard, os.Stdin) // hold until parent closes stdin
	os.Exit(0)
}

func TestInUse(t *testing.T) {
	dir := t.TempDir()
	if inUse, err := InUse(dir); err != nil || inUse {
		t.Fatalf("no session.lock: inUse=%v err=%v", inUse, err)
	}

	path := filepath.Join(dir, "session.lock")
	if err := os.WriteFile(path, []byte("☃"), 0o644); err != nil {
		t.Fatal(err)
	}
	if inUse, err := InUse(dir); err != nil || inUse {
		t.Fatalf("unlocked: inUse=%v err=%v", inUse, err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldLock$")
	cmd.Env = append(os.Environ(), "WK_HOLD_LOCK="+path)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if line, _ := bufio.NewReader(stdout).ReadString('\n'); line != "locked\n" {
		t.Fatalf("helper did not lock: %q", line)
	}

	if inUse, err := InUse(dir); err != nil || !inUse {
		t.Fatalf("locked by other process: inUse=%v err=%v", inUse, err)
	}

	stdin.Close()
	_ = cmd.Wait()
	if inUse, err := InUse(dir); err != nil || inUse {
		t.Fatalf("after release: inUse=%v err=%v", inUse, err)
	}
}
