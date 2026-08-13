package execx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	marker := ""
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			marker = os.Args[i+1]
			break
		}
	}
	if marker == "" {
		return
	}
	switch marker {
	case "success":
		fmt.Print("ok")
		os.Exit(0)
	case "failure":
		fmt.Fprint(os.Stderr, "bad")
		os.Exit(7)
	case "wait":
		time.Sleep(5 * time.Second)
	case "large":
		fmt.Print(strings.Repeat("x", 4096))
	case "tree":
		// Give the parent runner time to attach this process to its process
		// group/job before creating a descendant.
		time.Sleep(100 * time.Millisecond)
		child := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "heartbeat", os.Args[len(os.Args)-1])
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		_ = child.Wait()
	case "heartbeat":
		f, err := os.OpenFile(os.Args[len(os.Args)-1], os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			os.Exit(2)
		}
		defer f.Close()
		for {
			_, _ = f.Write([]byte("x"))
			time.Sleep(20 * time.Millisecond)
		}
	}
	os.Exit(0)
}
func helperSpec(mode string) CommandSpec {
	return CommandSpec{Name: filepath.Base(os.Args[0]), Args: []string{"-test.run=TestHelperProcess", "--", mode}, Env: os.Environ(), Timeout: 2 * time.Second, OutputLimit: 1024}
}
func TestSanitizedEnvironment(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("LANG", "C")
	got := sanitizedEnvironment(nil)
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "AWS_SECRET_ACCESS_KEY") || !strings.Contains(joined, "LANG=C") {
		t.Fatalf("unexpected environment: %v", got)
	}
}
func helperRunner() SafeRunner { return SafeRunner{Resolver: NewResolver(os.Args[0])} }
func TestSafeRunnerSuccess(t *testing.T) {
	result, err := helperRunner().Run(context.Background(), helperSpec("success"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "ok" || result.ExitCode != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}
func TestSafeRunnerExitCode(t *testing.T) {
	result, err := helperRunner().Run(context.Background(), helperSpec("failure"))
	if err == nil || result.ExitCode != 7 {
		t.Fatalf("expected exit 7: %#v %v", result, err)
	}
}
func TestSafeRunnerTimeout(t *testing.T) {
	spec := helperSpec("wait")
	spec.Timeout = 50 * time.Millisecond
	result, err := helperRunner().Run(context.Background(), spec)
	if !errors.Is(err, ErrTimeout) || !result.TimedOut {
		t.Fatalf("expected timeout: %#v %v", result, err)
	}
}

func TestSafeRunnerTimeoutKillsProcessTree(t *testing.T) {
	heartbeat := filepath.Join(t.TempDir(), "heartbeat")
	spec := helperSpec("tree")
	spec.Args = append(spec.Args, heartbeat)
	spec.Timeout = 350 * time.Millisecond
	result, err := helperRunner().Run(context.Background(), spec)
	if !errors.Is(err, ErrTimeout) || !result.TimedOut {
		t.Fatalf("expected timeout: %#v %v", result, err)
	}
	before, statErr := os.Stat(heartbeat)
	if statErr != nil || before.Size() == 0 {
		t.Fatalf("descendant did not write heartbeat: %v", statErr)
	}
	time.Sleep(200 * time.Millisecond)
	after, statErr := os.Stat(heartbeat)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if after.Size() != before.Size() {
		t.Fatalf("descendant survived timeout: size grew from %d to %d", before.Size(), after.Size())
	}
}
func TestSafeRunnerOutputLimit(t *testing.T) {
	result, err := helperRunner().Run(context.Background(), helperSpec("large"))
	if !errors.Is(err, ErrOutputLimit) || !result.Truncated || len(result.Stdout) != 1024 {
		t.Fatalf("expected capped output: len=%d result=%#v err=%v", len(result.Stdout), result, err)
	}
}
func TestSafeRunnerAllowlist(t *testing.T) {
	_, err := helperRunner().Run(context.Background(), CommandSpec{Name: "definitely-not-allowed"})
	if !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("unexpected error %v", err)
	}
}
