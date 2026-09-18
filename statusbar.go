package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
)

type StatusBar struct {
	status any
	Styles StatusBarStyles
}

type StatusBarStyles struct {
	ErrorMsg lipgloss.Style
}

func NewStatusBar(cfg Config) StatusBar {
	return StatusBar{
		Styles: cfg.Styles.StatusBarStyles(),
	}
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
	var v string
	if status, ok := b.status.(string); ok {
		v = fmt.Sprintf("  %s", status)
	}
	if err, ok := b.status.(error); ok {
		v = fmt.Sprintf("  %s", b.Styles.ErrorMsg.Render(err.Error()))
	}
	return v
}
