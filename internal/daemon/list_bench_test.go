package daemon

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkListJobResponses(b *testing.B) {
	for _, jobs := range []int{1, 100, 1000} {
		for _, history := range []int{0, 100000, 1000000} {
			b.Run(fmt.Sprintf("jobs=%d/runs=%d", jobs, history), func(b *testing.B) {
				jm := NewJobManager(b.TempDir(), nil, nil)
				now := time.Now()
				for i := 0; i < jobs; i++ {
					id := fmt.Sprintf("j%d", i)
					jm.jobs[id] = &Job{ID: id, Workdir: "/workdir", CreatedAt: now, Command: []string{"echo"}}
				}
				for i := 0; i < history; i++ {
					id := fmt.Sprintf("j%d", i%jobs)
					run := &Run{ID: fmt.Sprintf("r%d", i), JobID: id, StartedAt: now.Add(time.Duration(i) * time.Second)}
					jm.runs[run.ID] = run
					jm.recordLatestRunLocked(run)
				}
				jm.publishListLocked()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = jm.ListJobResponses("/workdir")
				}
			})
		}
	}
}
