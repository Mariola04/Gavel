//go:build !unix

package engine

import (
	"os/exec"
	"time"
)

// killProcessGroup falls back to killing only the process itself.
func killProcessGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = time.Second
}
