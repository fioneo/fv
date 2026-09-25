package main

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"os"
)

const usage = "fv [ directory ]"

type App struct {
	filepicker FilePicker
	statusbar  StatusBar
	KeyMap     AppKeyMap
	header     Header
	quitting   bool
	inputMode  bool
	WindowSize WindowSize
}

type WindowSize struct {
	Width  int
	Height int
}

func (a App) Init() tea.Cmd {
	return a.filepicker.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.WindowSize.Width = msg.Width
		a.WindowSize.Height = msg.Height

		a.filepicker.SetHeight(
			max(msg.Height-2, 1),
		)
	case filterMsg:
		if a.inputMode {
			a.inputMode = false
			var filepickerCmd tea.Cmd
			a.filepicker, filepickerCmd = a.filepicker.Update(msg)
			return a, tea.Batch(inputModeCmd(false), filepickerCmd)
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, a.KeyMap.Quit):
			a.quitting = true
			return a, tea.Quit

		case key.Matches(msg, a.KeyMap.FilterMode):
			if !a.inputMode {
				a.inputMode = true
				return a, inputModeCmd(true)
			}

		case key.Matches(msg, a.KeyMap.FilterExit):
			if a.inputMode {
				a.inputMode = false
				return a, inputModeCmd(false)
			}
		}
	}
	var cmd tea.Cmd
	if a.inputMode {
		a.header, cmd = a.header.Update(msg)
		return a, cmd
	}

	a.header, cmd = a.header.Update(msg)
	cmds = append(cmds, cmd)
	a.filepicker, cmd = a.filepicker.Update(msg)
	cmds = append(cmds, cmd)

	a.statusbar, cmd = a.statusbar.Update(msg)
	cmds = append(cmds, cmd)

	return a, tea.Batch(cmds...)
}

func (a App) View() tea.View {
	if a.quitting {
		return tea.NewView("")
	}

	header := a.header.View()
	statusbar := a.statusbar.View()

	filepickerHeight := max(a.WindowSize.Height-2, 1)

	filepicker := lipgloss.Place(
		a.WindowSize.Width,
		filepickerHeight,
		lipgloss.Left,
		lipgloss.Top,
		a.filepicker.View(),
	)

	str := lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		filepicker,
		statusbar,
	)

	v := tea.NewView(str)
	v.AltScreen = true
	return v
}

func main() {
	cfg, err := LoadConfig("/home/fioneo/.config/fv/config.json")
	if err != nil {
		fmt.Printf("ERROR loading config: %s\n", err.Error())
		os.Exit(1)
	}
	cfg.Settings.CurrentDir = "/"

	if len(os.Args) >= 2 {
		dir := os.Args[1]
		_, err := os.Stat(dir)
		if err != nil {
			fmt.Printf("invalid argument: %v\nusage: %s\n", os.Args[1], usage)
			os.Exit(1)
		}
		cfg.Settings.CurrentDir = dir
	}

	header := NewHeader(cfg)
	filepicker := NewFilePicker(cfg)
	statusbar := NewStatusBar(cfg)

	app := App{
		filepicker: filepicker,
		statusbar:  statusbar,
		header:     header,
		KeyMap:     cfg.KeyMap.AppKeyMap(),
	}

	_, err = tea.NewProgram(app).Run()
	if err != nil {
		fmt.Printf("ERROR starting program: %s\n", err.Error())
		os.Exit(1)
	}
}
