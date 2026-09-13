package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"os"
)

type App struct {
	filepicker FilePicker
	statusbar  StatusBar
	header     Header
	quitting   bool
	inputMode  bool
}

func (a App) Init() tea.Cmd {
	return a.filepicker.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case filterMsg:
		if !a.inputMode {
			break
		}
		a.inputMode = false
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			a.quitting = true
			return a, tea.Quit
		case "ctrl+f":
			if a.inputMode {
				break
			}
			a.inputMode = true
			return a, inputModeCmd(a.inputMode)
		case "esc":
			if !a.inputMode {
				break
			}
			a.inputMode = false
			return a, inputModeCmd(a.inputMode)
		}
	}
	if a.inputMode {
		var headerCmd tea.Cmd
		a.header, headerCmd = a.header.Update(msg)
		return a, headerCmd
	}
	var filepickerCmd tea.Cmd
	a.filepicker, filepickerCmd = a.filepicker.Update(msg)

	var statusbarCmd tea.Cmd
	a.statusbar, statusbarCmd = a.statusbar.Update(msg)

	var headerCmd tea.Cmd
	a.header, headerCmd = a.header.Update(msg)
	return a, tea.Batch(filepickerCmd, statusbarCmd, headerCmd)
}

func (a App) View() tea.View {
	if a.quitting {
		return tea.NewView("")
	}
	str := lipgloss.JoinVertical(
		lipgloss.Top,
		a.header.View(),
		a.filepicker.View(),
		a.statusbar.View(),
	)
	return tea.NewView(str)
}

func main() {
	header := NewHeader()
	filepicker := NewFilePicker()
	statusbar := NewStatusBar()

	app := App{
		filepicker: filepicker,
		statusbar:  statusbar,
		header:     header,
	}

	_, err := tea.NewProgram(app).Run()
	if err != nil {
		os.Exit(1)
	}
}
