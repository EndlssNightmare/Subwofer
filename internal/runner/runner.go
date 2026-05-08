package runner

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"

	"subwofer/internal/config"
)

// Runnable is the interface every passive tool must satisfy.
type Runnable interface {
	GetName() string
	IsAvailable(cfg *config.Config) (skipReason string, err error)
	Run(ctx context.Context, cfg *config.Config, outPath string,
		logCh chan<- string) (int, error)
}

// AggregateSourceFiles reads every file in cfg.SourcesDir and appends its
// contents to rawPath, then returns any error encountered.
func AggregateSourceFiles(cfg *config.Config, rawPath string) error {
	entries, err := os.ReadDir(cfg.SourcesDir)
	if err != nil {
		return err
	}

	dst, err := os.OpenFile(rawPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()

	w := bufio.NewWriter(dst)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(cfg.SourcesDir, entry.Name())
		if err := appendFile(w, srcPath); err != nil {
			// Non-fatal: skip broken source files.
			continue
		}
	}

	return w.Flush()
}

func appendFile(w io.Writer, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	return err
}
