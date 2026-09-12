package main

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"strings"
)

type Header struct {
	inputField textinput.Model
	CurrDir    string
	Selected   int
	FilesCount int
}

func NewHeader() Header {
	return Header{
		inputField: DefaultInputField(),
		CurrDir:    "/",
		Selected:   0,
		FilesCount: 0,
	}
}
func DefaultInputField() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.SetVirtualCursor(true)
	ti.CharLimit = 30
	ti.SetWidth(20)
	return ti
}

type headerMsg struct {
	dir        string
	selected   int
	filesCount int
}

func headerCmd(dir string, selected, filesCount int) tea.Cmd {
	return func() tea.Msg {
		return headerMsg{dir: dir, selected: selected, filesCount: filesCount}
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
	CurrDir    lipgloss.Style
	Selected   lipgloss.Style
	FilesCount lipgloss.Style
}

func DefaultHeaderStyles() HeaderStyles {
	return HeaderStyles{
		CurrDir:    lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
		Selected:   lipgloss.NewStyle().Foreground(lipgloss.Color("#A96AF0")),
		FilesCount: lipgloss.NewStyle().Foreground(lipgloss.Color("36")),
	}
}

func (h Header) Init() tea.Cmd {
	return textinput.Blink
}

func (h Header) Update(msg tea.Msg) (Header, tea.Cmd) {
	switch msg := msg.(type) {
	case headerMsg:
		h.CurrDir = msg.dir
		h.Selected = msg.selected
		h.FilesCount = msg.filesCount
	case inputModeMsg:
		if msg.inputMode {
			h.inputField.Focus()
			break
		}
		h.inputField.Blur()
	}

	var cmd tea.Cmd
	h.inputField, cmd = h.inputField.Update(msg)
	return h, cmd
}
func (h Header) View() string {
	var s strings.Builder
	selected := h.Selected
	if h.FilesCount > 0 {
		selected++
	}
	fmt.Fprintf(&s, "  %s  %s %d / %d ", h.CurrDir, h.inputField.View(), selected, h.FilesCount)

	return s.String()
}
