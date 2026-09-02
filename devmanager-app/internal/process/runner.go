// Package process porta app/process/runner.py a os/exec.
//
// CONCURRENCIA: OnStdout/OnStderr se disparan desde goroutines lectoras de
// pipes. El consumidor debe serializar acceso a su propio estado (mutex).
package process

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type RunnerCallbacks struct {
	OnStdout   func(line string)
	OnStderr   func(line string)
	OnStarted  func()
	OnFinished func(exitCode int, status string) // status: "NormalExit" | "CrashExit"
	OnError    func(desc string)
}

type Runner struct {
	mu       sync.Mutex
	cb       RunnerCallbacks
	cmd      *exec.Cmd
	starting bool
	waitDone chan struct{}
}

func NewRunner(cb RunnerCallbacks) *Runner {
	return &Runner{cb: cb}
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cmd != nil && r.cmd.Process != nil && r.cmd.ProcessState == nil
}

func (r *Runner) PID() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil && r.cmd.Process != nil {
		return r.cmd.Process.Pid
	}
	return 0
}

// Start ejecuta command via platform shell (cmd.exe on Windows, /bin/sh on Unix).
func (r *Runner) Start(command, workingDir string, extraEnv map[string]string) error {
	r.mu.Lock()
	if r.starting || (r.cmd != nil && r.cmd.Process != nil && r.cmd.ProcessState == nil) {
		r.mu.Unlock()
		return errors.New("process is already running")
	}
	r.starting = true
	cmd := shellCommand(command)
	cmd.Dir = workingDir
	if extraEnv != nil {
		env := os.Environ()
		for k, v := range extraEnv {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.starting = false
		r.mu.Unlock()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		r.starting = false
		r.mu.Unlock()
		return fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		r.starting = false
		r.mu.Unlock()
		if r.cb.OnError != nil {
			r.cb.OnError("Failed to start: " + err.Error())
		}
		return fmt.Errorf("start: %w", err)
	}
	r.cmd = cmd
	r.waitDone = make(chan struct{})
	r.mu.Unlock()

	if r.cb.OnStarted != nil {
		r.cb.OnStarted()
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scanLines(&wg, stdout, r.cb.OnStdout)
	go scanLines(&wg, stderr, r.cb.OnStderr)

	go func() {
		wg.Wait()
		waitErr := cmd.Wait()
		status := "NormalExit"
		code := 0
		if waitErr != nil {
			status = "CrashExit"
			var ee *exec.ExitError
			if errors.As(waitErr, &ee) {
				code = ee.ExitCode()
			} else {
				code = -1
			}
		}
		r.mu.Lock()
		r.cmd = nil
		r.starting = false
		close(r.waitDone)
		r.waitDone = nil
		r.mu.Unlock()
		if r.cb.OnFinished != nil {
			r.cb.OnFinished(code, status)
		}
	}()
	return nil
}

func scanLines(wg *sync.WaitGroup, f interface{ Read([]byte) (int, error) }, emit func(string)) {
	defer wg.Done()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := StripANSI(strings.ToValidUTF8(scanner.Text(), "\uFFFD"))
		if line != "" && emit != nil {
			emit(line)
		}
	}
}

// Stop replica ProcessRunner.stop(): kills the process tree and waits
// up to 5 seconds for clean exit.
func (r *Runner) Stop() {
	r.mu.Lock()
	if r.cmd == nil || r.cmd.Process == nil || r.cmd.ProcessState != nil {
		r.mu.Unlock()
		return
	}
	pid := r.cmd.Process.Pid
	done := r.waitDone
	r.mu.Unlock()

	killProcessTree(pid)

	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}
