package phase

import (
	"bufio"
	"io"

	"subwofer/internal/tui"
)

// verboseWriter returns an io.Writer that feeds stderr lines into the phase
// state log when verbose is true, or io.Discard when verbose is false.
// Call cleanup() after the command finishes to flush and close the pipe.
func verboseWriter(verbose bool, state *tui.PhaseState, toolName string) (w io.Writer, cleanup func()) {
	if !verbose {
		return io.Discard, func() {}
	}
	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			state.AddLog(toolName, sc.Text())
		}
	}()
	return pw, func() {
		_ = pw.Close()
		<-done
	}
}
