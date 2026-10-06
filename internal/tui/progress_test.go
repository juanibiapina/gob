package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestProgressBar_BeforeTypical(t *testing.T) {
	bar := ansi.Strip(progressBar(45*time.Second, 70*time.Second, 180*time.Second, 60))
	if !strings.HasSuffix(bar, "45s / ~1m10s (up to 3m)") {
		t.Fatalf("unexpected text: %q", bar)
	}
	if !strings.Contains(bar, "░") {
		t.Fatalf("expected an extra part: %q", bar)
	}
	if ansi.StringWidth(bar) != 60 {
		t.Fatalf("width = %d, want 60", ansi.StringWidth(bar))
	}
}

func TestProgressBar_InExtraPart(t *testing.T) {
	bar := progressBar(100*time.Second, 70*time.Second, 180*time.Second, 60)
	if !strings.Contains(bar, progressBarExtraFillStyle.Render("█")) {
		t.Fatalf("expected the extra part to be filling: %q", ansi.Strip(bar))
	}
	if !strings.HasSuffix(ansi.Strip(bar), "1m40s / ~1m10s (up to 3m)") {
		t.Fatalf("unexpected text: %q", ansi.Strip(bar))
	}
}

func TestProgressBar_PastUpper(t *testing.T) {
	bar := ansi.Strip(progressBar(200*time.Second, 70*time.Second, 180*time.Second, 60))
	if !strings.HasSuffix(bar, "3m20s, longer than usual") {
		t.Fatalf("unexpected text: %q", bar)
	}
	if strings.ContainsAny(bar, "▒░") {
		t.Fatalf("bar should be full: %q", bar)
	}
}

func TestProgressBar_SinglePartWhenUpperEqualsTypical(t *testing.T) {
	bar := ansi.Strip(progressBar(30*time.Second, 60*time.Second, 60*time.Second, 60))
	if strings.Contains(bar, "░") || strings.Contains(bar, "up to") {
		t.Fatalf("expected a single part: %q", bar)
	}
	if !strings.HasSuffix(bar, "30s / ~1m") {
		t.Fatalf("unexpected text: %q", bar)
	}
}
