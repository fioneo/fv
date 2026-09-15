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
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case filterMsg:
		if a.inputMode {
			a.inputMode = false
			var filepickerCmd tea.Cmd
			a.filepicker, filepickerCmd = a.filepicker.Update(msg)
			return a, tea.Batch(inputModeCmd(false), filepickerCmd)
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			a.quitting = true
			return a, tea.Quit

		case "ctrl+f":
			if !a.inputMode {
				a.inputMode = true
				return a, inputModeCmd(true)
			}

		case "esc":
			if a.inputMode {
				a.inputMode = false
				return a, inputModeCmd(false)
			}
		}
	}
	if a.inputMode {
		var headerCmd tea.Cmd
		a.header, headerCmd = a.header.Update(msg)
		return a, headerCmd
	}
	var headerCmd tea.Cmd
	a.header, headerCmd = a.header.Update(msg)
	cmds = append(cmds, headerCmd)
	var filepickerCmd tea.Cmd
	a.filepicker, filepickerCmd = a.filepicker.Update(msg)
	cmds = append(cmds, filepickerCmd)
	var statusbarCmd tea.Cmd
	a.statusbar, statusbarCmd = a.statusbar.Update(msg)
	cmds = append(cmds, statusbarCmd)

	return a, tea.Batch(cmds...)
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
