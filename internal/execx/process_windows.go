//go:build windows

package execx

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	createJobObject          = kernel32.NewProc("CreateJobObjectW")
	setInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	assignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	terminateJobObject       = kernel32.NewProc("TerminateJobObject")
)

const jobObjectExtendedLimitInformation = 9
const jobObjectLimitKillOnJobClose = 0x00002000
const processSetQuota = 0x0100
const processTerminate = 0x0001

type basicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}
type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}
type extendedLimitInformation struct {
	BasicLimitInformation basicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}
type windowsProcessController struct{ job syscall.Handle }

func newProcessController(cmd *exec.Cmd) (processController, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200}
	handle, _, callErr := createJobObject.Call(0, 0)
	if handle == 0 {
		return nil, fmt.Errorf("create Windows job object: %w", callErr)
	}
	controller := &windowsProcessController{job: syscall.Handle(handle)}
	info := extendedLimitInformation{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	ok, _, callErr := setInformationJobObject.Call(handle, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if ok == 0 {
		_ = syscall.CloseHandle(controller.job)
		return nil, fmt.Errorf("configure Windows job object: %w", callErr)
	}
	return controller, nil
}
func (c *windowsProcessController) Attach(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return fmt.Errorf("attach Windows job: process not started")
	}
	process, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fmt.Errorf("open child process: %w", err)
	}
	defer syscall.CloseHandle(process)
	ok, _, callErr := assignProcessToJobObject.Call(uintptr(c.job), uintptr(process))
	if ok == 0 {
		return fmt.Errorf("assign Windows job object: %w", callErr)
	}
	return nil
}
func (c *windowsProcessController) Kill(cmd *exec.Cmd) error {
	if c.job != 0 {
		ok, _, callErr := terminateJobObject.Call(uintptr(c.job), 1)
		if ok != 0 {
			return nil
		}
		if cmd.Process == nil {
			return fmt.Errorf("terminate Windows job: %w", callErr)
		}
	}
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
func (c *windowsProcessController) Close() error {
	if c.job == 0 {
		return nil
	}
	err := syscall.CloseHandle(c.job)
	c.job = 0
	return err
}
