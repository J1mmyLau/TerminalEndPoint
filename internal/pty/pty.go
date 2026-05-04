package pty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

// Terminal wraps a pseudo-terminal with its associated command.
type Terminal struct {
	file    *os.File
	cmd     *exec.Cmd
	done    chan struct{}
	exitErr error
	exitCode int
	mu       sync.Mutex
}

// Options configures a new terminal session.
type Options struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	Rows    uint16
	Cols    uint16
}

// New creates a new PTY-backed terminal running the specified command.
func New(opts Options) (*Terminal, error) {
	if opts.Command == "" {
		return nil, fmt.Errorf("pty: command is required")
	}
	if opts.Rows == 0 {
		opts.Rows = 24
	}
	if opts.Cols == 0 {
		opts.Cols = 80
	}

	cmd := exec.Command(opts.Command, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Env = opts.Env

	winSize := &pty.Winsize{
		Rows: opts.Rows,
		Cols: opts.Cols,
	}

	file, err := pty.StartWithSize(cmd, winSize)
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}

	t := &Terminal{
		file:     file,
		cmd:      cmd,
		done:     make(chan struct{}),
		exitCode: -1,
	}

	go t.waitForExit()

	return t, nil
}

// waitForExit waits for the command to finish and captures the exit status.
func (t *Terminal) waitForExit() {
	defer close(t.done)

	err := t.cmd.Wait()

	t.mu.Lock()
	defer t.mu.Unlock()

	t.exitErr = err
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				t.exitCode = status.ExitStatus()
			} else {
				t.exitCode = 1
			}
		} else {
			t.exitCode = 1
		}
	} else {
		t.exitCode = 0
	}
}

// Read reads from the PTY master. It's safe for concurrent use as the
// underlying os.File handles concurrent reads at the kernel level.
func (t *Terminal) Read(p []byte) (int, error) {
	return t.file.Read(p)
}

// Write writes to the PTY master (sends input to the slave).
func (t *Terminal) Write(p []byte) (int, error) {
	return t.file.Write(p)
}

// Resize changes the terminal window size.
func (t *Terminal) Resize(rows, cols uint16) error {
	winSize := &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}
	return pty.Setsize(t.file, winSize)
}

// Signal sends a signal to the terminal's process group.
func (t *Terminal) Signal(sig os.Signal) error {
	if t.cmd.Process == nil {
		return fmt.Errorf("pty: process not started")
	}
	return t.cmd.Process.Signal(sig)
}

// Close closes the PTY and ensures the process is terminated.
func (t *Terminal) Close() error {
	var errs []error

	// Try graceful termination first
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Close the PTY master — this sends SIGHUP to the slave
	if err := t.file.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close pty: %w", err))
	}

	// Wait with a timeout, then force kill
	select {
	case <-t.done:
		// exited normally
	default:
		if t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
		}
		<-t.done
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Done returns a channel that closes when the terminal process exits.
func (t *Terminal) Done() <-chan struct{} {
	return t.done
}

// ExitCode returns the exit code of the process, or -1 if still running.
func (t *Terminal) ExitCode() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exitCode
}

// ExitErr returns the error from the process exit, if any.
func (t *Terminal) ExitErr() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exitErr
}

// Alive returns true if the terminal process is still running.
func (t *Terminal) Alive() bool {
	select {
	case <-t.done:
		return false
	default:
		return true
	}
}

// Fd returns the file descriptor of the PTY master for use with io.Copy, etc.
func (t *Terminal) Fd() uintptr {
	return t.file.Fd()
}

// Reader returns an io.Reader for the PTY output.
func (t *Terminal) Reader() io.Reader {
	return t.file
}

// Writer returns an io.Writer for the PTY input.
func (t *Terminal) Writer() io.Writer {
	return t.file
}
