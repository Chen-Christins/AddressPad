package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPanelBounds(t *testing.T) {
	body := make([][]rowPart, 30)
	for i := range body {
		body[i] = []rowPart{{"row content that may be long", styles{}.text}}
	}
	for _, width := range []int{16, 24, 58} {
		for _, height := range []int{1, 3, 5, 8, 20} {
			panel := panelLines(width, height, "A title that may be long", body, "a hint that may be long", styles{})
			if len(panel) > height {
				t.Fatalf("panel %dx%d has %d lines", width, height, len(panel))
			}
			for _, line := range panel {
				if got := lipgloss.Width(line); got > max(16, width) {
					t.Fatalf("panel %dx%d line too wide: %q (%d)", width, height, line, got)
				}
			}
		}
	}
}

func TestOverlayBounds(t *testing.T) {
	base := []string{strings.Repeat("x", 10), strings.Repeat("y", 10)}
	panel := []string{"╭──╮", "│hi│", "╰──╯"}
	for _, size := range [][2]int{{20, 2}, {10, 8}, {24, 3}} {
		lines := overlayLines(base, panel, size[0], size[1])
		if len(lines) != size[1] {
			t.Fatalf("overlay %v has %d lines", size, len(lines))
		}
		for _, line := range lines {
			if got := lipgloss.Width(line); got > size[0] {
				t.Fatalf("overlay %v line too wide: %q (%d)", size, line, got)
			}
		}
	}
}

func TestWrapText(t *testing.T) {
	text := "alpha beta gamma delta epsilon"
	lines := wrapText(text, 12)
	if len(lines) < 2 {
		t.Fatalf("text was not wrapped: %q", lines)
	}
	if got := strings.Join(lines, " "); got != text {
		t.Fatalf("wrap changed text: %q", got)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > 12 {
			t.Fatalf("line too wide: %q (%d)", line, got)
		}
	}
	if got := wrapText("中文备注没有空格但是需要换行", 6); len(got) < 2 {
		t.Fatalf("CJK text was not wrapped: %q", got)
	}
}
