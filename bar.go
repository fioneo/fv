package main

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
)

type StatusBar struct {
	status string
}

func NewStatusBar() StatusBar {
	return StatusBar{}
}
func (b StatusBar) Init() tea.Cmd {
	return nil
}
func (b StatusBar) Update(msg tea.Msg) (StatusBar, tea.Cmd) {
	if msg, ok := msg.(statusMsg); ok {
		b.status = msg.status
	}
	return b, nil
}
func (b StatusBar) View() string {
	str := fmt.Sprintf("  Status: %s", b.status)
	return str
}
