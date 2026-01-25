package daemon

import (
	"fmt"
	"sync"
	"syscall"
)

// FakeProcessHandle implements ProcessHandle for testing
type FakeProcessHandle struct {
	pid       int
	running   bool
	waitCh    chan struct{}
	waitErr   error
	mu        sync.Mutex
	signalLog []syscall.Signal
}

func (h *FakeProcessHandle) Pid() int {
	return h.pid
}

func (h *FakeProcessHandle) Wait() error {
	<-h.waitCh
	return h.waitErr
}

func (h *FakeProcessHandle) Signal(sig syscall.Signal) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.signalLog = append(h.signalLog, sig)
	return nil
}

func (h *FakeProcessHandle) IsRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// Stop simulates the process stopping (unblocks Wait)
func (h *FakeProcessHandle) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		h.running = false
		close(h.waitCh)
	}
}

// StopWithExitCode simulates the process stopping with a specific exit code
// Note: This sets the waitErr which will be processed by waitForProcessExit,
// but since it's not a real exec.ExitError, the exit code won't be extracted.
// Use SetExitCode on the run directly for testing specific exit codes.
func (h *FakeProcessHandle) StopWithExitCode(exitCode int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		h.running = false
		// Store exit code for testing purposes (caller should set on run)
		close(h.waitCh)
	}
}

// IsRunningFake returns the fake running state
func (h *FakeProcessHandle) IsRunningFake() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// SignalLog returns the signals received
func (h *FakeProcessHandle) SignalLog() []syscall.Signal {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]syscall.Signal{}, h.signalLog...)
}

// FakeProcessExecutor implements ProcessExecutor for testing
type FakeProcessExecutor struct {
	mu          sync.Mutex
	nextPID     int
	handles     []*FakeProcessHandle
	startErr    error
	startCalled int
}

// NewFakeProcessExecutor creates a new fake executor
func NewFakeProcessExecutor() *FakeProcessExecutor {
	return &FakeProcessExecutor{
		nextPID: 1000,
	}
}

// Start creates a fake process
func (e *FakeProcessExecutor) Start(command []string, workdir string, env []string, stdoutPath, stderrPath string) (ProcessHandle, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.startCalled++

	if e.startErr != nil {
		return nil, e.startErr
	}

	if len(command) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	handle := &FakeProcessHandle{
		pid:     e.nextPID,
		running: true,
		waitCh:  make(chan struct{}),
	}
	e.nextPID++
	e.handles = append(e.handles, handle)

	return handle, nil
}

// SetStartError sets an error to return on next Start call
func (e *FakeProcessExecutor) SetStartError(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.startErr = err
}

// StartCount returns number of times Start was called
func (e *FakeProcessExecutor) StartCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.startCalled
}

// Handles returns all created handles
func (e *FakeProcessExecutor) Handles() []*FakeProcessHandle {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]*FakeProcessHandle{}, e.handles...)
}

// LastHandle returns the most recently created handle
func (e *FakeProcessExecutor) LastHandle() *FakeProcessHandle {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.handles) == 0 {
		return nil
	}
	return e.handles[len(e.handles)-1]
}

// StopAll stops all fake processes
func (e *FakeProcessExecutor) StopAll() {
	e.mu.Lock()
	handles := append([]*FakeProcessHandle{}, e.handles...)
	e.mu.Unlock()

	for _, h := range handles {
		h.Stop()
	}
}
