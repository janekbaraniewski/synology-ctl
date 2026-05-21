package views

import (
	"strings"
	"testing"
)

func TestSelectedLineOnlyMatchesColumnZeroCaret(t *testing.T) {
	lines := []string{
		"Shared folders, files, usage",
		"  ▸ collapsed root",
		"    ▸ nested collapsed folder",
		"\x1b[38;5;214m▸\x1b[0m  selected row",
	}

	if got := selectedLine(lines); got != 3 {
		t.Fatalf("selectedLine() = %d, want 3", got)
	}
}

func TestFitOrScrollFollowsSelectedLinePastTreeExpanders(t *testing.T) {
	lines := []string{
		"header",
		"  ▸ root-0",
		"  ▸ root-1",
		"  ▸ root-2",
		"  ▸ root-3",
		"  ▸ root-4",
		"  ▸ root-5",
		"  ▸ root-6",
		"  ▸ root-7",
		"  ▸ root-8",
		"\x1b[38;5;214m▸\x1b[0m  selected file",
		"tail-1",
		"tail-2",
	}

	got := fitOrScroll(strings.Join(lines, "\n"), 6)
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != 6 {
		t.Fatalf("fitOrScroll() returned %d lines, want 6", len(gotLines))
	}
	if !strings.Contains(got, "selected file") {
		t.Fatalf("fitOrScroll() did not include selected row:\n%s", got)
	}
	if strings.Contains(got, "root-0") {
		t.Fatalf("fitOrScroll() stayed anchored to a collapsed tree expander:\n%s", got)
	}
}
