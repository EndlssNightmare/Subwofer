package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"subwofer/internal/config"
	"subwofer/pkg/executil"
)

// CmdTool wraps an external binary tool.
type CmdTool struct {
	Name       string
	CheckAvail func(cfg *config.Config) (skipReason string, err error)
	BuildCmds  func(ctx context.Context, cfg *config.Config) ([]*exec.Cmd, error)
	Pipe       bool // use PipeCommands instead of RunAll
}

func (t *CmdTool) GetName() string { return t.Name }

func (t *CmdTool) IsAvailable(cfg *config.Config) (string, error) {
	if t.CheckAvail != nil {
		return t.CheckAvail(cfg)
	}
	return "", nil
}

func (t *CmdTool) Run(ctx context.Context, cfg *config.Config, outPath string, logCh chan<- string) (int, error) {
	cmds, err := t.BuildCmds(ctx, cfg)
	if err != nil {
		return 0, err
	}
	if len(cmds) == 0 {
		return 0, nil
	}

	var stderrDst io.Writer = io.Discard
	if logCh != nil && cfg.Verbose {
		pr, pw := io.Pipe()
		stderrDst = pw

		go func() {
			scanner := bufio.NewScanner(pr)
			for scanner.Scan() {
				line := scanner.Text()
				select {
				case logCh <- fmt.Sprintf("[%s] %s", t.Name, line):
				default:
				}
			}
		}()

		defer func() { _ = pw.Close() }()
	}

	if t.Pipe {
		return executil.PipeCommands(ctx, cmds, outPath, stderrDst)
	}
	return executil.RunAll(ctx, cmds, outPath, stderrDst)
}

// HTTPTool handles API-based sources that use Go HTTP directly.
type HTTPTool struct {
	Name       string
	CheckAvail func(cfg *config.Config) (skipReason string, err error)
	Fetch      func(ctx context.Context, cfg *config.Config) ([]string, error)
}

func (t *HTTPTool) GetName() string { return t.Name }

func (t *HTTPTool) IsAvailable(cfg *config.Config) (string, error) {
	if t.CheckAvail != nil {
		return t.CheckAvail(cfg)
	}
	return "", nil
}

func (t *HTTPTool) Run(ctx context.Context, cfg *config.Config, outPath string, logCh chan<- string) (int, error) {
	lines, err := t.Fetch(ctx, cfg)
	if err != nil {
		return 0, err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	count := 0
	for _, l := range lines {
		if l == "" {
			continue
		}
		if _, err := fmt.Fprintln(w, l); err != nil {
			return count, err
		}
		count++
	}
	return count, w.Flush()
}

// HasCmd reports whether name is available on PATH.
func HasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
