package daemon

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestSubscribeChanWaitsForAcknowledgment(t *testing.T) {
	server, connection := net.Pipe()
	defer server.Close()
	defer connection.Close()
	client := &Client{conn: connection}
	ready := make(chan struct{})
	deliver := make(chan struct{})
	go func() {
		var req Request
		if json.NewDecoder(server).Decode(&req) != nil {
			return
		}
		close(ready)
		<-deliver
		_ = json.NewEncoder(server).Encode(NewSuccessResponse())
		_ = json.NewEncoder(server).Encode(Event{Type: EventTypeJobStopping, JobID: "abc"})
	}()
	type result struct {
		events <-chan Event
		errors <-chan error
	}
	resultCh := make(chan result, 1)
	go func() { events, errors := client.SubscribeChan("/workdir"); resultCh <- result{events, errors} }()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("request was not sent")
	}
	select {
	case <-resultCh:
		t.Fatal("returned before subscribe acknowledgment")
	default:
	}
	close(deliver)
	select {
	case result := <-resultCh:
		select {
		case event := <-result.events:
			if event.JobID != "abc" {
				t.Fatalf("wrong event: %+v", event)
			}
		case <-time.After(time.Second):
			t.Fatal("event not delivered")
		}
	case <-time.After(time.Second):
		t.Fatal("acknowledged subscription did not return")
	}
}
