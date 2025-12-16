package telemetry

import "github.com/posthog/posthog-go"

var _ posthog.Logger = logger{}

// logger is a no-op implementation of posthog.Logger.
// We intentionally discard all log messages from PostHog because:
//  1. Telemetry failures (blocked by pihole, network issues, etc.) should never
//     affect user experience
//  2. Logging to stderr would corrupt the TUI display
//  3. Users shouldn't see errors about analytics they may have intentionally blocked
type logger struct{}

func (logger) Debugf(format string, args ...any) {}
func (logger) Logf(format string, args ...any)   {}
func (logger) Warnf(format string, args ...any)  {}
func (logger) Errorf(format string, args ...any) {}
