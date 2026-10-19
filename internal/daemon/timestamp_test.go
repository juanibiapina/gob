package daemon

import (
	"testing"
	"time"
)

func TestJobResponseReportsRunStartToTheMillisecond(t *testing.T) {
	jm, _ := newEstimateTestManager(t, nil)
	before := time.Now().Truncate(time.Millisecond)
	job, _, _ := jm.AddJob([]string{"build"}, "/workdir", "", false, nil)
	after := time.Now()

	resp, err := jm.GetJobResponse(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	startedAt, err := time.Parse(time.RFC3339, resp.StartedAt)
	if err != nil {
		t.Fatal(err)
	}

	if startedAt.Before(before) || startedAt.After(after) {
		t.Fatalf("StartedAt = %s, want between %s and %s", resp.StartedAt, before.Format(time.RFC3339Nano), after.Format(time.RFC3339Nano))
	}
}
