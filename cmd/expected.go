package cmd

import (
	"fmt"
	"time"

	"github.com/juanibiapina/gob/internal/daemon"
)

func formatExpectedDuration(job daemon.JobResponse) string {
	if job.ExpectedDurationMs <= 0 {
		return ""
	}
	typical := time.Duration(job.ExpectedDurationMs) * time.Millisecond
	upper := time.Duration(job.ExpectedUpperDurationMs) * time.Millisecond
	if upper > typical {
		return fmt.Sprintf("~%s (up to %s)", formatDuration(typical), formatDuration(upper))
	}
	return fmt.Sprintf("~%s", formatDuration(typical))
}

func runningStatus(job daemon.JobResponse, elapsed time.Duration) string {
	if job.ExpectedDurationMs <= 0 {
		return "running"
	}
	typical := time.Duration(job.ExpectedDurationMs) * time.Millisecond
	upper := time.Duration(job.ExpectedUpperDurationMs) * time.Millisecond
	switch {
	case elapsed < typical:
		return fmt.Sprintf("running (%.0f%%)", float64(elapsed)/float64(typical)*100)
	case elapsed < upper:
		return fmt.Sprintf("running (past typical, up to %s)", formatDuration(upper))
	default:
		return "running (longer than usual)"
	}
}
