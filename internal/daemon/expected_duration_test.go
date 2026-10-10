package daemon

import (
	"path/filepath"
	"testing"
	"time"
)

func newEstimateTestManager(t *testing.T, store *Store) (*JobManager, *FakeProcessExecutor) {
	t.Helper()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(t.TempDir(), nil, executor, store)
	return jm, executor
}

func waitForJobStopped(t *testing.T, jm *JobManager, jobID string) JobResponse {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		resp, err := jm.GetJobResponse(jobID)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Status == "stopped" {
			return resp
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not stop")
	return JobResponse{}
}

func finishOnItsOwn(t *testing.T, jm *JobManager, executor *FakeProcessExecutor, jobID string) JobResponse {
	t.Helper()
	time.Sleep(5 * time.Millisecond)
	executor.LastHandle().Stop()
	return waitForJobStopped(t, jm, jobID)
}

func stopThroughGob(t *testing.T, jm *JobManager, executor *FakeProcessExecutor, jobID string) JobResponse {
	t.Helper()
	time.Sleep(5 * time.Millisecond)
	handle := executor.LastHandle()
	go func() {
		for {
			resp, _ := jm.GetJobResponse(jobID)
			if resp.Status == "stopping" {
				handle.Stop()
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	if err := jm.StopJob(jobID, false); err != nil {
		t.Fatal(err)
	}
	return waitForJobStopped(t, jm, jobID)
}

func TestStoppedRunExitingZeroIsNotASuccess(t *testing.T) {
	jm, executor := newEstimateTestManager(t, nil)
	job, _, _ := jm.AddJob([]string{"serve"}, "/workdir", "", false, nil)

	resp := stopThroughGob(t, jm, executor, job.ID)

	if resp.RunCount != 1 || resp.SuccessCount != 0 || resp.FailureCount != 0 {
		t.Fatalf("got runs=%d successes=%d failures=%d, want 1/0/0", resp.RunCount, resp.SuccessCount, resp.FailureCount)
	}
	if resp.ExpectedDurationMs != 0 || resp.AvgDurationMs != 0 {
		t.Fatalf("stopped run produced an estimate: expected=%d avg=%d", resp.ExpectedDurationMs, resp.AvgDurationMs)
	}
}

func TestRunFinishingOnItsOwnSetsExpectedDuration(t *testing.T) {
	jm, executor := newEstimateTestManager(t, nil)
	job, _, _ := jm.AddJob([]string{"build"}, "/workdir", "", false, nil)

	resp := finishOnItsOwn(t, jm, executor, job.ID)

	if resp.SuccessCount != 1 {
		t.Fatalf("SuccessCount = %d, want 1", resp.SuccessCount)
	}
	if resp.ExpectedDurationMs <= 0 || resp.ExpectedUpperDurationMs < resp.ExpectedDurationMs {
		t.Fatalf("got expected=%d upper=%d", resp.ExpectedDurationMs, resp.ExpectedUpperDurationMs)
	}
}

func TestServerStoppedAfterOneQuickExitLosesItsEstimate(t *testing.T) {
	jm, executor := newEstimateTestManager(t, nil)
	job, _, _ := jm.AddJob([]string{"serve"}, "/workdir", "", false, nil)
	finishOnItsOwn(t, jm, executor, job.ID)

	if err := jm.StartJob(job.ID, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	resp := stopThroughGob(t, jm, executor, job.ID)

	if resp.ExpectedDurationMs != 0 {
		t.Fatalf("ExpectedDurationMs = %d, want 0", resp.ExpectedDurationMs)
	}
}

func TestSignaledRunIsNotASuccess(t *testing.T) {
	jm, executor := newEstimateTestManager(t, nil)
	job, _, _ := jm.AddJob([]string{"serve"}, "/workdir", "", false, nil)

	if err := jm.Signal(job.ID, 0); err != nil {
		t.Fatal(err)
	}
	resp := finishOnItsOwn(t, jm, executor, job.ID)

	if resp.SuccessCount != 0 || resp.ExpectedDurationMs != 0 {
		t.Fatalf("got successes=%d expected=%d, want 0/0", resp.SuccessCount, resp.ExpectedDurationMs)
	}
}

func TestRemovingRunsUpdatesExpectedDuration(t *testing.T) {
	jm, executor := newEstimateTestManager(t, nil)
	job, _, _ := jm.AddJob([]string{"build"}, "/workdir", "", false, nil)
	finishOnItsOwn(t, jm, executor, job.ID)
	if err := jm.StartJob(job.ID, nil); err != nil {
		t.Fatal(err)
	}
	stopThroughGob(t, jm, executor, job.ID)

	runs, _ := jm.ListRunsForJob(job.ID)
	for _, run := range runs {
		if err := jm.RemoveRun(run.ID); err != nil {
			t.Fatal(err)
		}
	}

	resp, _ := jm.GetJobResponse(job.ID)
	if resp.RunCount != 0 || resp.SuccessCount != 0 || resp.ExpectedDurationMs != 0 {
		t.Fatalf("got runs=%d successes=%d expected=%d, want 0/0/0", resp.RunCount, resp.SuccessCount, resp.ExpectedDurationMs)
	}
}

func TestEstimateSurvivesRestartAndIgnoresOldExitZeroRuns(t *testing.T) {
	db, err := OpenDatabase(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)

	jm, executor := newEstimateTestManager(t, store)
	oldJob, _, _ := jm.AddJob([]string{"old"}, "/workdir", "", false, nil)
	finishOnItsOwn(t, jm, executor, oldJob.ID)
	newJob, _, _ := jm.AddJob([]string{"new"}, "/workdir", "", false, nil)
	finishOnItsOwn(t, jm, executor, newJob.ID)

	if _, err := db.Exec("UPDATE runs SET interrupted = NULL WHERE job_id = ?", oldJob.ID); err != nil {
		t.Fatal(err)
	}

	reloaded := NewJobManagerWithExecutor(t.TempDir(), nil, executor, store)
	if err := reloaded.LoadFromStore(); err != nil {
		t.Fatal(err)
	}

	oldResp, _ := reloaded.GetJobResponse(oldJob.ID)
	if oldResp.SuccessCount != 1 || oldResp.ExpectedDurationMs != 0 {
		t.Fatalf("old job: successes=%d expected=%d, want 1/0", oldResp.SuccessCount, oldResp.ExpectedDurationMs)
	}
	newResp, _ := reloaded.GetJobResponse(newJob.ID)
	if newResp.ExpectedDurationMs <= 0 {
		t.Fatalf("new job lost its estimate after reload: %d", newResp.ExpectedDurationMs)
	}
}
