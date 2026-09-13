package main

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"strings"
)

type filterResetMsg struct{}

func filterResetCmd() tea.Cmd {
	return func() tea.Msg {
		return filterResetMsg{}
	}
}

type Header struct {
	inputField textinput.Model
	CurrDir    string
	Selected   int
	FilesCount int
	Styles     HeaderStyles
}

func NewHeader() Header {
	return Header{
		inputField: DefaultInputField(),
		CurrDir:    "/",
		Selected:   0,
		FilesCount: 0,
		Styles:     DefaultHeaderStyles(),
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

type filterMsg struct {
	filter string
}

func filterCmd(filter string) tea.Cmd {
	return func() tea.Msg {
		return filterMsg{filter: filter}
	}
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
		CurrDir:    lipgloss.NewStyle(),
		Selected:   lipgloss.NewStyle().Foreground(lipgloss.Color("#BFB09F")),
		FilesCount: lipgloss.NewStyle().Foreground(lipgloss.Color("#BFA09F")),
	}
}

func (h Header) Init() tea.Cmd {
	return textinput.Blink
}

func (h Header) Update(msg tea.Msg) (Header, tea.Cmd) {
	switch msg := msg.(type) {
	case filterResetMsg:
		h.inputField.SetValue("")
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
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			return h, tea.Batch(inputModeCmd(false), filterCmd(h.inputField.Value()))
		}
	}
	var cmd tea.Cmd
	h.inputField, cmd = h.inputField.Update(msg)
	return h, cmd
}
func (h Header) View() string {
	var s strings.Builder
	curr := h.Selected
	if h.FilesCount > 0 {
		curr++
	}
	selected := fmt.Sprintf("%d", curr)
	filescount := fmt.Sprintf("%d", h.FilesCount)
	fmt.Fprintf(&s, "  %s  %s %s / %s", h.Styles.CurrDir.Render(h.CurrDir), h.inputField.View(), h.Styles.Selected.Render(selected), h.Styles.FilesCount.Render(filescount))

	return s.String()
}
