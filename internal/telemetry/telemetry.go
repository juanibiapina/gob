package telemetry

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"

	"github.com/juanibiapina/gob/internal/version"
	"github.com/posthog/posthog-go"
)

const (
	endpoint = "https://eu.i.posthog.com"
	key      = "phc_LYz5yMmLW6BCBf4XaZ4P5g6bDDjraFALiJbTJBU5nkb"
)

var (
	client posthog.Client

	baseProps = posthog.NewProperties().
			Set("goos", runtime.GOOS).
			Set("goarch", runtime.GOARCH).
			Set("term", os.Getenv("TERM")).
			Set("shell", filepath.Base(os.Getenv("SHELL"))).
			Set("version", version.Version).
			Set("go_version", runtime.Version())
)

func Init() {
	if isDisabled() {
		return
	}
	c, err := posthog.NewWithConfig(key, posthog.Config{
		Endpoint: endpoint,
		Logger:   logger{},
	})
	if err != nil {
		slog.Error("Failed to initialize PostHog client", "error", err)
	}
	client = c
	distinctId = getDistinctId()
}

func isDisabled() bool {
	if v, _ := strconv.ParseBool(os.Getenv("GOB_TELEMETRY_DISABLED")); v {
		return true
	}
	if v, _ := strconv.ParseBool(os.Getenv("DO_NOT_TRACK")); v {
		return true
	}
	return false
}

func send(event string, props ...any) {
	if client == nil {
		return
	}
	err := client.Enqueue(posthog.Capture{
		DistinctId: distinctId,
		Event:      event,
		Properties: pairsToProps(props...).Merge(baseProps),
	})
	if err != nil {
		slog.Error("Failed to enqueue PostHog event", "event", event, "props", props, "error", err)
		return
	}
}

func Error(err any, props ...any) {
	if client == nil {
		return
	}
	props = append(
		[]any{
			"$exception_list",
			[]map[string]string{
				{"type": reflect.TypeOf(err).String(), "value": fmt.Sprintf("%v", err)},
			},
		},
		props...,
	)
	send("$exception", props...)
}

func Flush() {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil {
		slog.Error("Failed to flush PostHog events", "error", err)
	}
}

func pairsToProps(props ...any) posthog.Properties {
	p := posthog.NewProperties()

	if !isEven(len(props)) {
		slog.Error("Event properties must be provided as key-value pairs", "props", props)
		return p
	}

	for i := 0; i < len(props); i += 2 {
		key := props[i].(string)
		value := props[i+1]
		p = p.Set(key, value)
	}
	return p
}

func isEven(n int) bool {
	return n%2 == 0
}
