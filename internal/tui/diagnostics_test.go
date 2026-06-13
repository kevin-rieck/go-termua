package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiagnosticLogRetainsRecentEventsWithinCapacity(t *testing.T) {
	log := NewDiagnosticLog(2)
	current := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	log.now = func() time.Time {
		current = current.Add(time.Second)
		return current
	}

	log.Add("first")
	log.Add("second")
	log.Add("third")

	events := log.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %#v", events)
	}
	if events[0].Message != "second" || events[1].Message != "third" {
		t.Fatalf("expected newest events to be retained, got %#v", events)
	}
}

func TestFilesystemDiagnosticsExporterDoesNotOverwriteRepeatedExports(t *testing.T) {
	exportDir := t.TempDir()
	exporter := filesystemDiagnosticsExporter{
		baseDir: exportDir,
		now:     func() time.Time { return time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC) },
	}

	first, err := exporter.ExportDiagnostics("first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := exporter.ExportDiagnostics("second")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("expected unique Diagnostics Bundle paths, got %q", first)
	}

	files, err := filepath.Glob(filepath.Join(exportDir, "exports", "diagnostics-*.md"))
	if err != nil || len(files) != 2 {
		t.Fatalf("expected two Diagnostics Bundle files, files=%#v err=%v", files, err)
	}
	firstContent, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondContent, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstContent) != "first" || string(secondContent) != "second" {
		t.Fatalf("unexpected Diagnostics Bundle contents: first=%q second=%q", firstContent, secondContent)
	}
}
