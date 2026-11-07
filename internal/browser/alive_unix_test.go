//go:build !windows

package browser

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

func processAlive(pid int) bool {
	// A killed grandchild may briefly remain a zombie until container init reaps it.
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil && strings.Contains(string(data), ") Z ") {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
