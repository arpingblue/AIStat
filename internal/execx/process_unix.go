//go:build !windows

package execx

import (
	"os/exec"
	"syscall"
)

type unixProcessController struct{}

func newProcessController(cmd *exec.Cmd) (processController, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return unixProcessController{}, nil
}
func (unixProcessController) Attach(*exec.Cmd) error { return nil }
func (unixProcessController) Kill(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
func (unixProcessController) Close() error { return nil }
