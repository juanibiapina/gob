package daemon

import (
	"math"
	"sort"
	"time"
)

const (
	estimateHalfLifeRuns = 10
	estimateMaxRuns      = 500
	estimateTypicalBelow = 0.5
	estimateUpperAtMost  = 0.1
)

type runSample struct {
	Duration    time.Duration
	ExitCode    *int
	Interrupted *bool
}

type sampleKind int

const (
	sampleIgnored sampleKind = iota
	sampleFinished
	sampleCensored
)

func classifySample(s runSample) sampleKind {
	if s.Interrupted != nil && *s.Interrupted {
		return sampleCensored
	}
	if s.ExitCode == nil {
		return sampleCensored
	}
	switch *s.ExitCode {
	case 129, 130, 137, 143:
		return sampleCensored
	case 0:
		if s.Interrupted == nil {
			return sampleIgnored
		}
		return sampleFinished
	}
	return sampleIgnored
}

func estimateDuration(samples []runSample) (typical, upper time.Duration, ok bool) {
	if len(samples) > estimateMaxRuns {
		samples = samples[len(samples)-estimateMaxRuns:]
	}

	type observation struct {
		duration time.Duration
		finished bool
		weight   float64
	}

	observations := make([]observation, 0, len(samples))
	atRisk := 0.0
	for i, s := range samples {
		kind := classifySample(s)
		if kind == sampleIgnored {
			continue
		}
		age := len(samples) - 1 - i
		weight := math.Pow(0.5, float64(age)/estimateHalfLifeRuns)
		observations = append(observations, observation{duration: s.Duration, finished: kind == sampleFinished, weight: weight})
		atRisk += weight
	}

	sort.Slice(observations, func(a, b int) bool { return observations[a].duration < observations[b].duration })

	survival := 1.0
	var lastFinish time.Duration
	foundTypical, foundUpper := false, false
	for i := 0; i < len(observations); {
		duration := observations[i].duration
		finished, leaving := 0.0, 0.0
		for ; i < len(observations) && observations[i].duration == duration; i++ {
			if observations[i].finished {
				finished += observations[i].weight
			}
			leaving += observations[i].weight
		}
		if finished > 0 {
			survival *= 1 - finished/atRisk
			lastFinish = duration
			if !foundTypical && survival < estimateTypicalBelow {
				typical, foundTypical = duration, true
			}
			if !foundUpper && survival <= estimateUpperAtMost+1e-9 {
				upper, foundUpper = duration, true
			}
		}
		atRisk -= leaving
	}

	if !foundTypical {
		return 0, 0, false
	}
	if !foundUpper {
		upper = lastFinish
	}
	return typical, upper, true
}
