package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotAllowed  = errors.New("command is not allowlisted")
	ErrOutputLimit = errors.New("command output exceeded limit")
	ErrTimeout     = errors.New("command timed out")
)

type CommandSpec struct {
	Name        string
	Args        []string
	Env         []string
	Dir         string
	Timeout     time.Duration
	OutputLimit int64
}

type Result struct {
	Stdout    string
	Stderr    string
	ExitCode  int
	Duration  time.Duration
	TimedOut  bool
	Truncated bool
}

type Runner interface {
	Run(context.Context, CommandSpec) (Result, error)
}

type processController interface {
	Attach(*exec.Cmd) error
	Kill(*exec.Cmd) error
	Close() error
}

type Resolver struct {
	Allowed map[string]string
}

var procExePattern = regexp.MustCompile(`^/proc/[0-9]+/exe$`)

func NewResolver(names ...string) Resolver {
	r := Resolver{Allowed: make(map[string]string, len(names))}
	for _, name := range names {
		base := filepath.Base(name)
		path := ""
		if filepath.IsAbs(name) || filepath.Dir(name) != "." {
			path, _ = filepath.Abs(name)
		}
		r.Allowed[base] = path
	}
	return r
}

func (r Resolver) Resolve(name string) (string, error) {
	if filepath.IsAbs(name) && procExePattern.MatchString(filepath.ToSlash(name)) {
		target, err := os.Readlink(name)
		if err != nil {
			return "", err
		}
		base := filepath.Base(target)
		if !strings.HasPrefix(base, "python") {
			return "", fmt.Errorf("%w: proc interpreter %s", ErrNotAllowed, base)
		}
		return name, nil
	}
	base := filepath.Base(name)
	fixed, ok := r.Allowed[base]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNotAllowed, base)
	}
	if fixed != "" {
		return fixed, nil
	}
	p, err := exec.LookPath(base)
	if err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

type SafeRunner struct {
	Resolver Resolver
}

func (r SafeRunner) Run(parent context.Context, spec CommandSpec) (Result, error) {
	started := time.Now()
	limit := spec.OutputLimit
	if limit <= 0 {
		limit = 4 << 20
	}
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	path, err := r.Resolver.Resolve(spec.Name)
	if err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, spec.Args...)
	cmd.Env = sanitizedEnvironment(spec.Env)
	cmd.Dir = spec.Dir
	controller, controllerErr := newProcessController(cmd)
	if controllerErr != nil {
		return Result{}, controllerErr
	}
	defer controller.Close()
	cmd.Cancel = func() error { return controller.Kill(cmd) }
	cmd.WaitDelay = 250 * time.Millisecond
	stdout, stderr := &limitedBuffer{limit: limit}, &limitedBuffer{limit: limit}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err = cmd.Start(); err == nil {
		if attachErr := controller.Attach(cmd); attachErr != nil {
			_ = controller.Kill(cmd)
			_ = cmd.Wait()
			return Result{}, attachErr
		}
		err = cmd.Wait()
	}
	exitCode := 0
	if err != nil {
		exitCode = -1
	}
	res := Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: exitCode, Duration: time.Since(started), TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded), Truncated: stdout.truncated || stderr.truncated}
	if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
	}
	if res.Truncated && err == nil {
		err = ErrOutputLimit
	}
	if res.TimedOut {
		err = fmt.Errorf("%w: %v", ErrTimeout, err)
	}
	return res, err
}

var allowedEnvironment = map[string]bool{"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "LANG": true, "LC_ALL": true, "LC_CTYPE": true, "SYSTEMROOT": true, "WINDIR": true, "TEMP": true, "TMP": true, "TMPDIR": true, "DOCKER_HOST": true, "DOCKER_CONTEXT": true, "CUDA_VISIBLE_DEVICES": true}

func sanitizedEnvironment(explicit []string) []string {
	source := explicit
	if source == nil {
		source = os.Environ()
	}
	result := []string{}
	seen := map[string]bool{}
	for _, entry := range source {
		key, _, ok := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if !ok || !allowedEnvironment[upper] || seen[upper] {
			continue
		}
		seen[upper] = true
		result = append(result, entry)
	}
	sort.Strings(result)
	return result
}

type limitedBuffer struct {
	mu        sync.Mutex
	b         bytes.Buffer
	limit     int64
	written   int64
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := b.limit - b.written
	if remaining > 0 {
		write := int64(len(p))
		if write > remaining {
			write = remaining
		}
		_, _ = b.b.Write(p[:write])
		b.written += write
	}
	if int64(n) > remaining {
		b.truncated = true
	}
	return n, nil
}

func (b *limitedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

var _ io.Writer = (*limitedBuffer)(nil)
