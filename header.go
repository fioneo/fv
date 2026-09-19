package main

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"strings"
)

type Header struct {
	inputField      textinput.Model
	filePickerState filePickerStateMsg
	Styles          HeaderStyles
	screenWidth     int
	inputMode       bool
}

func NewHeader(cfg Config) Header {
	return Header{
		inputField: DefaultInputField(),
		Styles:     cfg.Styles.HeaderStyles(),
	}
}

func DefaultInputField() textinput.Model {
	ti := textinput.New()
	ti.SetVirtualCursor(true)
	ti.CharLimit = 25
	ti.Prompt = ""
	return ti
}

type inputModeMsg struct {
	inputMode bool
}

func inputModeCmd(mode bool) tea.Cmd {
	return func() tea.Msg {
		return inputModeMsg{inputMode: mode}
	}
}

type filterResetMsg struct{}

func filterResetCmd() tea.Cmd {
	return func() tea.Msg {
		return filterResetMsg{}
	}
}

type filterMsg struct {
	filter string
}

func filterCmd(filter string) tea.Cmd {
	return func() tea.Msg {
		return filterMsg{filter: filter}
	}
}

type HeaderStyles struct {
	CurrentDir lipgloss.Style
	Selected   lipgloss.Style
	FilesCount lipgloss.Style
	SortBy     lipgloss.Style
}

func (h Header) Init() tea.Cmd {
	return textinput.Blink
}

func (h Header) Update(msg tea.Msg) (Header, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.screenWidth = msg.Width

	case filterResetMsg:
		h.inputField.Reset()

	case filePickerStateMsg:
		h.filePickerState = msg

	case inputModeMsg:
		if msg.inputMode {
			h.inputMode = true
			h.inputField.Focus()
		} else {
			h.inputMode = false
			h.inputField.Blur()
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			return h, tea.Batch(
				filterCmd(h.inputField.Value()),
				filterResetCmd(),
			)
		}
	}

	var cmd tea.Cmd
	h.inputField, cmd = h.inputField.Update(msg)
	return h, cmd
}

func (h Header) View() string {
	state := h.filePickerState

	currIdx := state.selectedIdx

	if state.filesCount > 0 && currIdx < state.filesCount-1 {
		currIdx++
	}

	right := fmt.Sprintf(
		" %s  %s / %s",
		h.Styles.SortBy.Render(state.sortBy.String()),
		h.Styles.Selected.Render(fmt.Sprintf("%d", currIdx)),
		h.Styles.FilesCount.Render(fmt.Sprintf("%d", state.filesCount)),
	)

	var left string

	if h.inputMode {
		left = fmt.Sprintf(
			"  find: %s",
			h.inputField.View(),
		)
	} else {
		available := h.screenWidth - lipgloss.Width(right) - 2

		left = fmt.Sprintf(
			"  %s",
			h.Styles.CurrentDir.Render(
				truncatePath(state.currentDir, available),
			),
		)
	}

	gap := h.screenWidth - lipgloss.Width(left) - lipgloss.Width(right)

	if gap < 0 {
		gap = 0
	}

	return left + strings.Repeat(" ", gap) + right
}

func truncatePath(path string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	if lipgloss.Width(path) <= maxWidth {
		return path
	}

	parts := strings.Split(path, "/")

	for i := 0; i < len(parts); i++ {
		candidateParts := parts[i:]

		candidate := ".../" + strings.Join(candidateParts, "/")

		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}

	return path
}
