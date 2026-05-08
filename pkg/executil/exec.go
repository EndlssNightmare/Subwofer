package executil

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
)

// lineCountWriter wraps an io.Writer and counts newlines written through it.
type lineCountWriter struct {
	w     io.Writer
	lines int
}

func (lc *lineCountWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if b == '\n' {
			lc.lines++
		}
	}
	if lc.w != nil {
		return lc.w.Write(p)
	}
	return len(p), nil
}

// filteredWriter is an io.Writer that drops lines containing a given substring.
type filteredWriter struct {
	w      io.Writer
	filter string
	buf    []byte
}

func (fw *filteredWriter) Write(p []byte) (int, error) {
	fw.buf = append(fw.buf, p...)
	for {
		idx := strings.IndexByte(string(fw.buf), '\n')
		if idx < 0 {
			break
		}
		line := string(fw.buf[:idx+1])
		fw.buf = fw.buf[idx+1:]
		if !strings.Contains(line, fw.filter) {
			if _, err := io.WriteString(fw.w, line); err != nil {
				return 0, err
			}
		}
	}
	return len(p), nil
}

// RunAll runs cmds sequentially, writing all stdout to outPath.
// Non-zero exits are ignored. Returns the number of lines written and any
// file I/O error.
func RunAll(ctx context.Context, cmds []*exec.Cmd, outPath string, stderr io.Writer) (int, error) {
	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	lcw := &lineCountWriter{w: f}

	if stderr == nil {
		stderr = io.Discard
	}

	for _, cmd := range cmds {
		cmd.Stdout = lcw
		cmd.Stderr = stderr
		// ignore non-zero exits
		_ = cmd.Run()

		// Ensure context cancellation propagates immediately.
		if ctx.Err() != nil {
			break
		}
	}

	return lcw.lines, nil
}

// PipeCommands chains cmds via io.Pipe so each cmd's stdout feeds the next.
// The final cmd writes to outPath. Returns lines written and any error.
func PipeCommands(ctx context.Context, cmds []*exec.Cmd, outPath string, stderr io.Writer) (int, error) {
	if len(cmds) == 0 {
		return 0, nil
	}

	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	lcw := &lineCountWriter{w: f}

	if stderr == nil {
		stderr = io.Discard
	}

	if len(cmds) == 1 {
		cmds[0].Stdout = lcw
		cmds[0].Stderr = stderr
		_ = cmds[0].Run()
		return lcw.lines, nil
	}

	// Wire pipes: cmd[i].Stdout -> pipe -> cmd[i+1].Stdin
	pipes := make([]*io.PipeWriter, len(cmds)-1)
	for i := 0; i < len(cmds)-1; i++ {
		pr, pw := io.Pipe()
		pipes[i] = pw
		cmds[i].Stdout = pw
		cmds[i].Stderr = stderr
		cmds[i+1].Stdin = pr
	}
	cmds[len(cmds)-1].Stdout = lcw
	cmds[len(cmds)-1].Stderr = stderr

	// Start all commands.
	for _, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			// Close remaining pipes on failure.
			for _, pw := range pipes {
				_ = pw.Close()
			}
			return 0, err
		}
		if ctx.Err() != nil {
			break
		}
	}

	// Wait sequentially, closing write-end of pipe after each cmd finishes
	// so the next cmd sees EOF.
	errs := make([]error, len(cmds))
	for i, cmd := range cmds {
		errs[i] = cmd.Wait()
		if i < len(pipes) {
			_ = pipes[i].Close()
		}
	}

	// Drain any remaining input for last cmd.
	_ = errs[len(errs)-1]

	return lcw.lines, nil
}

// NewFilteredWriter returns a writer that drops lines containing filter and
// forwards the rest to w.
func NewFilteredWriter(w io.Writer, filter string) io.Writer {
	return &filteredWriter{w: w, filter: filter}
}

// ScanLines reads all lines from r and sends them on ch.
// Closes ch when r is exhausted or returns an error.
func ScanLines(r io.Reader, ch chan<- string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		ch <- scanner.Text()
	}
}
