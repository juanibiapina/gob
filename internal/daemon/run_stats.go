package daemon

import (
	"sort"
)

type runOutcome int

const (
	outcomeUncounted runOutcome = iota
	outcomeSuccess
	outcomeFailure
)

func outcomeOf(run *Run) runOutcome {
	if run.Interrupted != nil && *run.Interrupted {
		return outcomeUncounted
	}
	if run.ExitCode == nil {
		return outcomeUncounted
	}
	if *run.ExitCode == 0 {
		return outcomeSuccess
	}
	return outcomeFailure
}

func (j *Job) applyRunStats(run *Run) {
	durationMs := run.StoppedAt.Sub(run.StartedAt).Milliseconds()
	j.RunCount++
	switch outcomeOf(run) {
	case outcomeSuccess:
		j.SuccessCount++
		j.SuccessTotalDurationMs += durationMs
	case outcomeFailure:
		j.FailureCount++
		j.FailureTotalDurationMs += durationMs
	}
}

func (j *Job) revertRunStats(run *Run) {
	durationMs := run.StoppedAt.Sub(run.StartedAt).Milliseconds()
	j.RunCount--
	switch outcomeOf(run) {
	case outcomeSuccess:
		j.SuccessCount--
		j.SuccessTotalDurationMs -= durationMs
	case outcomeFailure:
		j.FailureCount--
		j.FailureTotalDurationMs -= durationMs
	}
}

func (j *Job) setEstimateFromRuns(runs []*Run) {
	finished := make([]*Run, 0, len(runs))
	for _, run := range runs {
		if run.StoppedAt != nil {
			finished = append(finished, run)
		}
	}
	sort.Slice(finished, func(a, b int) bool { return finished[a].StartedAt.Before(finished[b].StartedAt) })

	samples := make([]runSample, len(finished))
	for i, run := range finished {
		samples[i] = runSample{Duration: run.StoppedAt.Sub(run.StartedAt), ExitCode: run.ExitCode, Interrupted: run.Interrupted}
	}
	typical, upper, ok := estimateDuration(samples)
	if !ok {
		j.ExpectedDurationMs, j.ExpectedUpperDurationMs = 0, 0
		return
	}
	j.ExpectedDurationMs = max(typical.Milliseconds(), 1)
	j.ExpectedUpperDurationMs = max(upper.Milliseconds(), j.ExpectedDurationMs)
}

func (jm *JobManager) markInterrupted(run *Run) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	interrupted := true
	run.Interrupted = &interrupted
}

func (jm *JobManager) refreshEstimateLocked(job *Job) {
	var runs []*Run
	for _, run := range jm.runs {
		if run.JobID == job.ID {
			runs = append(runs, run)
		}
	}
	job.setEstimateFromRuns(runs)
}
