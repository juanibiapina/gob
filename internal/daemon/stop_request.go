package daemon

import (
	"fmt"
)

type stopAttempt struct {
	runID  string
	pid    int
	done   chan struct{}
	force  chan struct{}
	forced bool  // guarded by JobManager.mu
	err    error // published by closing done
}

// RequestStop acknowledges a stop attempt before process discovery or signal waiting.
func (jm *JobManager) RequestStop(jobID string, force bool) (int, <-chan struct{}, error) {
	op, pid, err := jm.requestStop(jobID, force)
	if err != nil {
		return 0, nil, err
	}
	if op == nil {
		done := make(chan struct{})
		close(done)
		return pid, done, nil
	}
	return pid, op.done, nil
}

func (jm *JobManager) requestStop(jobID string, force bool) (*stopAttempt, int, error) {
	jm.mu.Lock()
	job, ok := jm.jobs[jobID]
	if !ok {
		jm.mu.Unlock()
		return nil, 0, fmt.Errorf("job not found: %s", jobID)
	}
	if job.CurrentRunID == nil {
		pid := 0
		if run := jm.latestRun[jobID]; run != nil {
			pid = run.PID
		}
		jm.mu.Unlock()
		return nil, pid, nil
	}
	run := jm.runs[*job.CurrentRunID]
	if op := jm.stops[jobID]; op != nil && op.runID == run.ID {
		select {
		case <-op.done:
			if op.err == nil {
				jm.mu.Unlock()
				return op, op.pid, nil
			}
		default:
			if force && !op.forced {
				op.forced = true
				close(op.force)
			}
			jm.mu.Unlock()
			return op, op.pid, nil
		}
	}
	op := &stopAttempt{runID: run.ID, pid: run.PID, done: make(chan struct{}), force: make(chan struct{}), forced: force}
	jm.stops[jobID] = op
	job.Stopping = true
	job.StopError = ""
	run.Status = "stopping"
	interrupted := true
	run.Interrupted = &interrupted
	if run.stopVerified == nil {
		run.stopVerified = make(chan struct{})
	}
	event := Event{Type: EventTypeJobStopping, JobID: jobID, Job: jm.jobToResponse(job), Run: ptrRunResponse(run), JobCount: len(jm.jobs), RunningJobCount: jm.countRunningJobsLocked()}
	jm.publishListLocked()
	jm.mu.Unlock()
	jm.emitEvent(event)
	go func() {
		err := jm.stopJobProcess(jobID, force, op.force)
		jm.mu.Lock()
		op.err = err
		if err == nil {
			close(run.stopVerified)
			jm.mu.Unlock()
			if run.finalized != nil {
				<-run.finalized
			}
			jm.mu.Lock()
		}
		if err != nil { // Retain stopping until a retry verifies termination.
			job.StopError = err.Error()
			jm.publishListLocked()
			event := Event{Type: EventTypeJobStopFailed, JobID: jobID, Job: jm.jobToResponse(job), Run: ptrRunResponse(run), JobCount: len(jm.jobs), RunningJobCount: jm.countRunningJobsLocked()}
			jm.mu.Unlock()
			jm.emitEvent(event)
			jm.mu.Lock()
		}
		close(op.done)
		jm.mu.Unlock()
	}()
	return op, op.pid, nil
}

func ptrRunResponse(run *Run) *RunResponse { response := runToResponse(run); return &response }

// StopJob waits until the accepted stop has completed and reports verification errors.
func (jm *JobManager) StopJob(jobID string, force bool) error {
	op, _, err := jm.requestStop(jobID, force)
	if err != nil {
		return err
	}
	if op == nil {
		return nil
	}
	<-op.done
	return op.err
}
