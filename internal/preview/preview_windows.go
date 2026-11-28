//go:build windows

package preview

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runProcessTree runs command with cmd.exe /d /s /c inside a job object and
// ends the job when the shell exits or ctx ends. The shell starts suspended
// and resumes only once it is in the job, so every process it starts joins
// the job too. Closing the job also kills anything left in it.
func runProcessTree(ctx context.Context, command string, env []string, stdout, stderr io.Writer) error {
	job, err := newKillOnCloseJob()
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(job) }()
	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = "cmd.exe"
	}
	//nolint:gosec // preview commands are user-configured shell snippets by design.
	c := exec.CommandContext(ctx, shell)
	// /s keeps the command between the outer quotes exactly as written.
	c.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       syscall.EscapeArg(shell) + ` /d /s /c "` + command + `"`,
		CreationFlags: windows.CREATE_SUSPENDED,
	}
	c.Cancel = func() error { return windows.TerminateJobObject(job, 1) }
	c.WaitDelay = 250 * time.Millisecond
	if env != nil {
		c.Env = append(os.Environ(), env...)
	}
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Start(); err != nil {
		return err
	}
	//nolint:gosec // a Windows process ID is a DWORD.
	if err := adopt(job, uint32(c.Process.Pid)); err != nil {
		_ = c.Process.Kill()
		_ = c.Wait()
		return err
	}
	err = c.Wait()
	killErr := windows.TerminateJobObject(job, 1)
	if err != nil {
		return err
	}
	return killErr
}

func newKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	//nolint:gosec // SetInformationJobObject takes the struct by pointer.
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

// adopt puts the suspended process pid in job, then resumes it.
func adopt(job windows.Handle, pid uint32) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(process) }()
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		return err
	}
	return resumeThreads(pid)
}

func resumeThreads(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	resumed := false
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(thread)
		_ = windows.CloseHandle(thread)
		if err != nil {
			return err
		}
		resumed = true
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return err
	}
	if !resumed {
		return errors.New("preview shell has no thread to resume")
	}
	return nil
}
