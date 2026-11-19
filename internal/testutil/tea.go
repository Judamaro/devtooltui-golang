package testutil

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// CmdTimeout es lo que RunCmds espera a cada comando. Los resultados de las
// herramientas llegan en microsegundos; los ticks lentos (parpadeo del
// cursor, debounce) se descartan.
const CmdTimeout = 30 * time.Millisecond

// RunCmds ejecuta cmd y entrega cada mensaje a update, que puede devolver
// más comandos. Expande tea.BatchMsg. Los comandos que tardan más de
// CmdTimeout se abandonan.
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
