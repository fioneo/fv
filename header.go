package main

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
)

type filterResetMsg struct{}

func filterResetCmd() tea.Cmd {
	return func() tea.Msg {
		return filterResetMsg{}
	}
}

type Header struct {
	inputField      textinput.Model
	filePickerState filePickerStateMsg
	Styles          HeaderStyles
}

func NewHeader() Header {
	return Header{
		inputField: DefaultInputField(),
		Styles:     DefaultHeaderStyles(),
	}
}
func DefaultInputField() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.SetVirtualCursor(true)
	ti.CharLimit = 20
	ti.SetWidth(12)
	return ti
}

type filterMsg struct {
	filter string
}

func filterCmd(filter string) tea.Cmd {
	return func() tea.Msg {
		return filterMsg{filter: filter}
	}
}

type inputModeMsg struct {
	inputMode bool
}

func inputModeCmd(mode bool) tea.Cmd {
	return func() tea.Msg {
		return inputModeMsg{inputMode: mode}
	}
}

type HeaderStyles struct {
	CurrentDir lipgloss.Style
	Selected   lipgloss.Style
	FilesCount lipgloss.Style
	SortBy     lipgloss.Style
}

func DefaultHeaderStyles() HeaderStyles {
	return HeaderStyles{
		CurrentDir: lipgloss.NewStyle(),
		Selected:   lipgloss.NewStyle().Foreground(lipgloss.Color("#BFB09F")),
		FilesCount: lipgloss.NewStyle().Foreground(lipgloss.Color("#BFA09F")),
		SortBy:     lipgloss.NewStyle().Foreground(lipgloss.Color("#AEDD6C")),
	}
}

func (h Header) Init() tea.Cmd {
	return textinput.Blink
}

func (h Header) Update(msg tea.Msg) (Header, tea.Cmd) {

	switch msg := msg.(type) {
	case filterResetMsg:
		h.inputField.Reset()

	case filePickerStateMsg:
		h.filePickerState = msg
	case inputModeMsg:
		if msg.inputMode {
			h.inputField.Focus()
		} else {
			h.inputField.Blur()
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			return h, filterCmd(h.inputField.Value())
		}
	}

	var cmd tea.Cmd
	h.inputField, cmd = h.inputField.Update(msg)

	return h, cmd
}

func (h Header) View() string {
	state := h.filePickerState
	curr := state.selectedIdx
	if len(state.files) > 0 {
		curr++
	}

	selected := fmt.Sprintf("%d", curr)
	filescount := fmt.Sprintf("%d", len(state.files))
	return fmt.Sprintf(
		"  %s  %s ( %s / %s ) [ %s ]",
		h.Styles.CurrentDir.Render(state.currentDir),
		h.inputField.View(), h.Styles.Selected.Render(selected),
		h.Styles.FilesCount.Render(filescount),
		h.Styles.SortBy.Render(state.sortBy.String()))

}
