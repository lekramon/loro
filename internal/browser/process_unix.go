//go:build !windows

package browser

import (
	"os/exec"
	"syscall"
)

func prepareProcess(cmd *exec.Cmd) (func(), error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	kill := func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.Cancel = kill
	return func() { _ = kill() }, nil
}
func attachProcess(cmd *exec.Cmd) error { return nil }
