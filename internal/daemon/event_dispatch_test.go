package daemon

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestSlowSubscriberDoesNotBlockListOrOtherSubscribers(t *testing.T) {
	jm := NewJobManagerWithExecutor(t.TempDir(), nil, NewFakeProcessExecutor(), nil)
	job, _, err := jm.AddJob([]string{"echo"}, "/workdir", "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	slowReader, slowWriter := net.Pipe()
	defer slowReader.Close()
	defer slowWriter.Close()
	fastReader, fastWriter := net.Pipe()
	defer fastReader.Close()
	defer fastWriter.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slow := &Subscriber{conn: slowWriter, encoder: json.NewEncoder(slowWriter), events: make(chan Event, 1), done: make(chan struct{})}
	fast := &Subscriber{conn: fastWriter, encoder: json.NewEncoder(fastWriter), events: make(chan Event, 1), done: make(chan struct{})}
	d := &Daemon{ctx: ctx, events: make(chan Event, 2), jobManager: jm, subscribers: []*Subscriber{slow, fast}}
	go d.dispatchEvents()
	go d.deliverSubscriberEvents(slow)
	go d.deliverSubscriberEvents(fast)
	d.handleEvent(Event{Type: EventTypeJobStarted, JobID: job.ID, Job: jm.ListJobResponses("/workdir")[0]})
	received := make(chan Event, 1)
	go func() { var event Event; _ = json.NewDecoder(fastReader).Decode(&event); received <- event }()
	select {
	case event := <-received:
		if event.JobID != job.ID {
			t.Fatalf("wrong subscriber event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("slow subscriber delayed another subscriber")
	}
	listed := make(chan *Response, 1)
	go func() { listed <- d.handleList(&Request{Payload: map[string]any{"workdir": "/workdir"}}) }()
	select {
	case resp := <-listed:
		if !resp.Success || len(resp.Data["jobs"].([]JobResponse)) != 1 {
			t.Fatalf("bad list: %+v", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("list blocked by subscriber")
	}
}
