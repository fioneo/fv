package main

import (
	tea "charm.land/bubbletea/v2"
)

type StatusBar struct {
	err error
}

func NewStatusBar() StatusBar {
	return StatusBar{}
}
func (b StatusBar) Init() tea.Cmd {
	return nil
}
func (b StatusBar) Update(msg tea.Msg) (StatusBar, tea.Cmd) {
	if msg, ok := msg.(errorMsg); ok {
		b.err = msg.err
	}
	return b, nil
}
func (b StatusBar) View() string {
	str := "  Status: "
	if b.err != nil {
		str += b.err.Error()
	}
	return str
}
