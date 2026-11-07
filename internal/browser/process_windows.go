package browser

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

// A kill-on-close Job Object owns only this helper's process tree.
var jobs sync.Map

func prepareProcess(cmd *exec.Cmd) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	jobs.Store(cmd, job)
	var once sync.Once
	closeJob := func() { once.Do(func() { jobs.Delete(cmd); windows.CloseHandle(job) }) }
	cmd.Cancel = func() error {
		closeJob()
		if cmd.Process != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	return closeJob, nil
}
func attachProcess(cmd *exec.Cmd) error {
	value, ok := jobs.Load(cmd)
	if !ok {
		return windows.ERROR_CANCELLED
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	return windows.AssignProcessToJobObject(value.(windows.Handle), process)
}
