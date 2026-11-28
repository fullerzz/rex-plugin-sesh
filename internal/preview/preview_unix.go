//go:build unix

package preview

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// runProcessTree runs command with sh -lc in a process group of its own and
// kills the group when the shell exits or ctx ends. A stopped sentinel
// process leads the group, so its ID can't be reused while the command runs.
func runProcessTree(ctx context.Context, command string, stdout, stderr io.Writer) error {
	sentinel := exec.Command("sh", "-c", `kill -s STOP "$$"`)
	sentinel.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := sentinel.Start(); err != nil {
		return err
	}
	processGroupID := sentinel.Process.Pid
	killProcessGroup := func() error {
		err := syscall.Kill(-processGroupID, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	//nolint:gosec // preview commands are user-configured shell snippets by design.
	c := exec.CommandContext(ctx, "sh", "-lc", command)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: processGroupID}
	c.Cancel = killProcessGroup
	c.WaitDelay = 250 * time.Millisecond
	c.Stdout = stdout
	c.Stderr = stderr
	err := c.Run()
	killErr := killProcessGroup()
	if killErr != nil {
		_ = sentinel.Process.Kill()
	}
	_ = sentinel.Wait()
	if err != nil {
		return err
	}
	return killErr
}
