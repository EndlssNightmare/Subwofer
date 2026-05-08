package tui

import (
	"fmt"
	"sync"
)

// ToolStatus represents the lifecycle state of a single tool.
type ToolStatus int

const (
	StatusPending ToolStatus = iota
	StatusRunning
	StatusDone
	StatusSkipped
	StatusFailed
)

// ToolState holds the display state for one tool.
type ToolState struct {
	Name    string
	Status  ToolStatus
	Count   int
	Message string
}

// PhaseState tracks all tools in a phase and recent log lines.
type PhaseState struct {
	mu       sync.Mutex
	Tools    []*ToolState
	LogLines []string // verbose log entries, capped at 200
}

// NewPhaseState initialises a PhaseState with all tools in Pending status.
func NewPhaseState(names []string) *PhaseState {
	ps := &PhaseState{}
	for _, n := range names {
		ps.Tools = append(ps.Tools, &ToolState{Name: n, Status: StatusPending})
	}
	return ps
}

// Update finds the tool by name and applies fn to it while holding the mutex.
func (p *PhaseState) Update(name string, fn func(*ToolState)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, t := range p.Tools {
		if t.Name == name {
			fn(t)
			return
		}
	}
}

// Snapshot returns a copy of all tool states and log lines.
func (p *PhaseState) Snapshot() ([]ToolState, []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	tools := make([]ToolState, len(p.Tools))
	for i, t := range p.Tools {
		tools[i] = *t
	}

	logs := make([]string, len(p.LogLines))
	copy(logs, p.LogLines)

	return tools, logs
}

// AddLog appends a tagged log line; keeps only the last 200 entries.
func (p *PhaseState) AddLog(toolName, line string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry := fmt.Sprintf("[%s] %s", toolName, line)
	p.LogLines = append(p.LogLines, entry)
	if len(p.LogLines) > 200 {
		p.LogLines = p.LogLines[len(p.LogLines)-200:]
	}
}
