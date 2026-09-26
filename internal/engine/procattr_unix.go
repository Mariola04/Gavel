//go:build unix

package engine

import (
	"os/exec"
	"syscall"
	"time"
)

// killProcessGroup runs cmd in its own process group and, when its context
// is cancelled, kills the whole group so no child survives.
func killProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = time.Second
}
