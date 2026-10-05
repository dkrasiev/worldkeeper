package lock

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// These tests lock session.lock from a real JVM, the way Minecraft does
// (testdata/HoldLock.java), so the probe is checked against Java's own
// locking on every OS: fcntl on Unix, mandatory LockFileEx on Windows.
// CI sets WK_REQUIRE_JAVA so a missing JVM fails instead of skipping.
func javaPath(t *testing.T) string {
	t.Helper()
	java, err := exec.LookPath("java")
	if err != nil {
		if os.Getenv("WK_REQUIRE_JAVA") != "" {
			t.Fatal("java not found but WK_REQUIRE_JAVA is set")
		}
		t.Skip("java not installed")
	}
	return java
}

func holdLock(t *testing.T, java string, args ...string) *exec.Cmd {
	t.Helper()
	src, _ := filepath.Abs(filepath.Join("testdata", "HoldLock.java"))
	return exec.Command(java, append([]string{src}, args...)...)
}

func TestDetectsLockHeldByJava(t *testing.T) {
	java := javaPath(t)
	dir := t.TempDir()

	cmd := holdLock(t, java, "hold", filepath.Join(dir, "session.lock"))
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if line, _ := bufio.NewReader(stdout).ReadString('\n'); strings.TrimSpace(line) != "locked" {
		t.Fatalf("java did not lock: %q", line)
	}

	if inUse, err := InUse(dir); err != nil || !inUse {
		t.Fatalf("while java holds the lock: inUse=%v err=%v", inUse, err)
	}

	stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if inUse, err := InUse(dir); err != nil || inUse {
		t.Fatalf("after java released the lock: inUse=%v err=%v", inUse, err)
	}
}

// The game must never fail to open a world because we probed it at the
// same moment. Java opens and locks the file in a tight loop while we probe
// in another tight loop; any refused lock fails the test.
func TestProbeNeverBlocksJava(t *testing.T) {
	java := javaPath(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "session.lock")
	if err := os.WriteFile(file, []byte("☃"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := holdLock(t, java, "cycle", file, "2000")
	out := &strings.Builder{}
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var probes, sawLocked atomic.Int64
	deadline := time.After(2 * time.Minute)
	for {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("java could not lock while being probed: %v, output %q", err, out.String())
			}
			t.Logf("%d probes, %d saw the lock held", probes.Load(), sawLocked.Load())
			if probes.Load() == 0 {
				t.Fatal("no probes ran")
			}
			return
		case <-deadline:
			cmd.Process.Kill()
			t.Fatal("timeout")
		default:
			inUse, err := InUse(dir)
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			probes.Add(1)
			if inUse {
				sawLocked.Add(1)
			}
		}
	}
}
