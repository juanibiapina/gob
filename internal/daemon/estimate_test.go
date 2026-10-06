package daemon

import (
	"testing"
	"time"
)

func sample(seconds int, exitCode *int, interrupted *bool) runSample {
	return runSample{Duration: time.Duration(seconds) * time.Second, ExitCode: exitCode, Interrupted: interrupted}
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func clean(seconds int) runSample   { return sample(seconds, intPtr(0), boolPtr(false)) }
func stopped(seconds int) runSample { return sample(seconds, intPtr(0), boolPtr(true)) }
func killed(seconds int) runSample  { return sample(seconds, nil, boolPtr(false)) }
func failed(seconds int) runSample  { return sample(seconds, intPtr(2), boolPtr(false)) }

func legacy(seconds int, exitCode *int) runSample { return sample(seconds, exitCode, nil) }

func cleanRuns(seconds ...int) []runSample {
	var out []runSample
	for _, s := range seconds {
		out = append(out, clean(s))
	}
	return out
}

func repeat(n int, pattern ...runSample) []runSample {
	var out []runSample
	for i := 0; i < n; i++ {
		out = append(out, pattern...)
	}
	return out
}

func concat(parts ...[]runSample) []runSample {
	var out []runSample
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestEstimateDuration(t *testing.T) {
	hour := 3600
	lovelyGirlHouseServe := []runSample{
		legacy(503, nil), legacy(11246, nil), legacy(29, nil),
		legacy(257630, nil), legacy(5374, nil), legacy(90944, intPtr(143)),
	}
	dotfilesMakeOld := []runSample{
		legacy(36, intPtr(0)), legacy(133, intPtr(0)), legacy(33, intPtr(0)), legacy(36, intPtr(0)), legacy(41, intPtr(0)),
		legacy(32, intPtr(0)), legacy(39, intPtr(0)), legacy(84, intPtr(0)), legacy(216, intPtr(0)), legacy(48, intPtr(0)),
	}

	tests := []struct {
		name    string
		samples []runSample
		typical int
		upper   int
	}{
		{"no runs", nil, 0, 0},
		{"only failures", []runSample{failed(10), failed(12)}, 0, 0},
		{"only stopped runs", []runSample{stopped(2 * hour), stopped(40 * 60)}, 0, 0},
		{"single clean run", cleanRuns(8), 8, 8},
		{"consistent build", cleanRuns(60, 55, 70), 60, 70},
		{"build with early cancels", append(cleanRuns(60, 55, 70), stopped(10), stopped(20)), 60, 70},
		{"mixed fast and slow build", cleanRuns(36, 133, 33, 36, 41, 32, 39, 84, 216, 48), 41, 216},
		{"server with one quick clean exit", []runSample{clean(8), stopped(hour)}, 0, 0},
		{"server history plus one clean exit", append(lovelyGirlHouseServe, clean(8)), 0, 0},
		{"server, 27 stops and 3 clean exits", repeat(3, clean(5), stopped(hour), stopped(hour), stopped(hour), stopped(hour), stopped(hour), stopped(hour), stopped(hour), stopped(hour), stopped(hour)), 0, 0},
		{"server, 300 stops and 100 clean exits", repeat(100, clean(5), stopped(hour), stopped(hour), stopped(hour)), 0, 0},
		{"build, 300 clean runs and 100 stops", repeat(100, stopped(hour), clean(60), clean(60), clean(60)), 60, 60},
		{"old exit-0 runs are left out", append(dotfilesMakeOld, clean(28)), 28, 28},
		{"old killed and exit-143 runs count as stopped", append(lovelyGirlHouseServe, clean(8), clean(9)), 0, 0},
		{"killed outside gob counts as stopped", []runSample{clean(8), killed(hour)}, 0, 0},
		{"recent stops outweigh older clean runs", concat(repeat(50, clean(60)), repeat(30, stopped(hour))), 0, 0},
		{"recent clean runs outweigh older stops", concat(repeat(30, stopped(hour)), repeat(50, clean(60))), 60, 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typical, upper, ok := estimateDuration(tt.samples)
			wantTypical := time.Duration(tt.typical) * time.Second
			wantUpper := time.Duration(tt.upper) * time.Second
			if ok != (tt.typical != 0) || typical != wantTypical || upper != wantUpper {
				t.Fatalf("got typical=%v upper=%v ok=%v, want %v/%v", typical, upper, ok, wantTypical, wantUpper)
			}
		})
	}
}

func TestEstimateDuration_SameMixAtTenTimesTheCount(t *testing.T) {
	small := concat(repeat(1, stopped(3600), stopped(3600)), cleanRuns(50, 60, 70))
	large := repeat(10, small...)
	_, _, smallOK := estimateDuration(small)
	_, _, largeOK := estimateDuration(large)
	if smallOK != largeOK {
		t.Fatalf("show/hide differs with scale: small=%v large=%v", smallOK, largeOK)
	}
}

func TestEstimateDuration_CapLimitsInput(t *testing.T) {
	recent := repeat(estimateMaxRuns, clean(30))
	withOldStops := concat(repeat(1000, stopped(3600)), recent)
	gotTypical, gotUpper, _ := estimateDuration(withOldStops)
	wantTypical, wantUpper, _ := estimateDuration(recent)
	if gotTypical != wantTypical || gotUpper != wantUpper {
		t.Fatalf("runs older than the cap changed the result: got %v/%v, want %v/%v", gotTypical, gotUpper, wantTypical, wantUpper)
	}
}
