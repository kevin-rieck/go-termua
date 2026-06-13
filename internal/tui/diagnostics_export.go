package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"termua/internal/config"
)

type DiagnosticsExporter interface {
	ExportDiagnostics(markdown string) (string, error)
}

type filesystemDiagnosticsExporter struct {
	baseDir string
	now     func() time.Time
}

func newFilesystemDiagnosticsExporter(paths config.Paths) DiagnosticsExporter {
	baseDir := paths.CacheDir
	if baseDir == "" {
		baseDir = "."
	}
	return filesystemDiagnosticsExporter{baseDir: baseDir, now: time.Now}
}

func (e filesystemDiagnosticsExporter) ExportDiagnostics(markdown string) (string, error) {
	exportDir := filepath.Join(e.baseDir, "exports")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return "", err
	}

	stamp := e.now().Format("20060102-150405")
	for attempt := 0; attempt < 1000; attempt++ {
		path := filepath.Join(exportDir, diagnosticsFilename(stamp, attempt))
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		_, writeErr := file.WriteString(markdown)
		closeErr := file.Close()
		if writeErr != nil {
			_ = os.Remove(path)
			return "", writeErr
		}
		if closeErr != nil {
			_ = os.Remove(path)
			return "", closeErr
		}
		return path, nil
	}
	return "", fmt.Errorf("could not create unique Diagnostics Bundle filename for timestamp %s", stamp)
}

func diagnosticsFilename(stamp string, attempt int) string {
	if attempt == 0 {
		return "diagnostics-" + stamp + ".md"
	}
	return fmt.Sprintf("diagnostics-%s-%03d.md", stamp, attempt)
}
