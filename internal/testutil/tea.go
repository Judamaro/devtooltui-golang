package testutil

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// CmdTimeout is how long RunCmds waits for each command. Tool results arrive
// in microseconds; slow ticks (cursor blink, debounce) are dropped.
const CmdTimeout = 30 * time.Millisecond

// RunCmds runs cmd and hands every message to update, which may return more
// commands. It expands tea.BatchMsg. Commands that take longer than
// CmdTimeout are abandoned.
func RunCmds(cmd tea.Cmd, update func(tea.Msg) tea.Cmd) {
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		ch := make(chan tea.Msg, 1)
		go func() { ch <- c() }()
		var msg tea.Msg
		select {
		case msg = <-ch:
		case <-time.After(CmdTimeout):
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if msg != nil {
			queue = append(queue, update(msg))
		}
	}
}
