package daemon

import (
	"os"
	"testing"
)

func TestReusedPIDIsNotATrackedSurvivor(t *testing.T) {
	pid := os.Getpid()
	info, err := getProcessInfo(pid)
	if err != nil {
		t.Fatal(err)
	}
	survivors, err := filterRunningPIDs(map[int]int64{pid: info.StartTime.UnixMilli() - 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(survivors) != 0 {
		t.Fatalf("reused PID treated as original process: %v", survivors)
	}
}
