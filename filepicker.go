package main

import (
	"bytes"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/dustin/go-humanize"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	fileSizeWidth  = 6
	permWidth      = 12
	ownershipWidth = 14
	marginLeft     = 2
)

type FilePicker struct {
	CurrentDir     string
	selectedIdx    int
	ShowHidden     bool
	Cursor         string
	files          []os.DirEntry
	viewportHeight int
	maxIdx         int
	minIdx         int
	KeyMap         KeyMap
	Styles         Styles
	extended       bool
}

func NewFilePicker() FilePicker {
	return FilePicker{
		CurrentDir:     "/",
		selectedIdx:    0,
		ShowHidden:     false,
		Cursor:         "> ",
		viewportHeight: 20,
		maxIdx:         0,
		minIdx:         0,
		KeyMap:         DefaultKeyMap(),
		Styles:         DefaultStyles(),
	}
}

type execMsg struct {
	path string
}

func execCmd(path string) tea.Cmd {
	return func() tea.Msg {
		return execMsg{path: path}
	}
}

type errorMsg struct {
	err error
}

func errorCmd(err error) tea.Cmd {
	return func() tea.Msg {
		return errorMsg{err: err}
	}
}

type readDirMsg struct {
	entries []os.DirEntry
}

type KeyMap struct {
	GoToTop      key.Binding
	GoToLast     key.Binding
	Down         key.Binding
	Up           key.Binding
	Back         key.Binding
	Open         key.Binding
	Find         key.Binding
	ToggleHidden key.Binding
	Extended     key.Binding
}

type Styles struct {
	DisabledCursor   lipgloss.Style
	Cursor           lipgloss.Style
	Symlink          lipgloss.Style
	Directory        lipgloss.Style
	File             lipgloss.Style
	Ownership        lipgloss.Style
	DisabledFile     lipgloss.Style
	Permission       lipgloss.Style
	Selected         lipgloss.Style
	DisabledSelected lipgloss.Style
	FileSize         lipgloss.Style
	EmptyDirectory   lipgloss.Style
}

func DefaultStyles() Styles {
	return Styles{
		DisabledCursor:   lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
		Cursor:           lipgloss.NewStyle().Foreground(lipgloss.Color("#A96AF0")),
		Symlink:          lipgloss.NewStyle().Foreground(lipgloss.Color("36")),
		Directory:        lipgloss.NewStyle().Foreground(lipgloss.Color("99")),
		File:             lipgloss.NewStyle(),
		Ownership:        lipgloss.NewStyle().Width(ownershipWidth),
		DisabledFile:     lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		DisabledSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
		Permission:       lipgloss.NewStyle().Foreground(lipgloss.Color("#06CBB4")).Width(permWidth),
		Selected:         lipgloss.NewStyle().Foreground(lipgloss.Color("#240641")).Bold(true),
		FileSize:         lipgloss.NewStyle().Foreground(lipgloss.Color("#7F849C")).Width(fileSizeWidth).Align(lipgloss.Right),
		EmptyDirectory:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")).SetString("No Files Found.\n").MarginLeft(marginLeft),
	}
}
func DefaultKeyMap() KeyMap {
	return KeyMap{
		GoToTop:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "first")),
		GoToLast:     key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "last")),
		Down:         key.NewBinding(key.WithKeys("j", "down", "ctrl+n"), key.WithHelp("j", "down")),
		Up:           key.NewBinding(key.WithKeys("k", "up", "ctrl+p"), key.WithHelp("k", "up")),
		Back:         key.NewBinding(key.WithKeys("h", "backspace", "left", "esc"), key.WithHelp("h", "back")),
		Open:         key.NewBinding(key.WithKeys("l", "right", "enter"), key.WithHelp("l", "open")),
		ToggleHidden: key.NewBinding(key.WithKeys("i", "ctrl+i"), key.WithHelp("i", "toggle hidden files")),
		Extended:     key.NewBinding(key.WithKeys("e", "ctrl+e"), key.WithHelp("e", "extended look on selected file")),
	}
}

func (fp *FilePicker) SetHeight(h int) {
	fp.viewportHeight = h

	if fp.maxIdx > fp.viewportHeight-1 {
		fp.maxIdx = fp.bottomIdx(fp.minIdx)
	}
}

func (fp FilePicker) Height() int {
	return fp.viewportHeight
}

func (fp FilePicker) bottomIdx(topIdx int) int {
	if fp.viewportHeight < 1 {
		return topIdx
	}
	return topIdx + fp.viewportHeight - 1
}

func (fp FilePicker) readDir(path string) tea.Cmd {
	return func() tea.Msg {
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			return errorMsg{err: err}
		}
		if fp.ShowHidden {
			return readDirMsg{entries: dirEntries}
		}

		var visibleEntries []os.DirEntry
		for _, dirEntry := range dirEntries {
			if isHidden(dirEntry.Name()) {
				continue
			}
			visibleEntries = append(visibleEntries, dirEntry)
		}
		return readDirMsg{entries: visibleEntries}
	}
}

func (fp FilePicker) Init() tea.Cmd {
	return fp.readDir(fp.CurrentDir)
}
func (fp *FilePicker) Reset() {
	fp.selectedIdx = 0
	fp.minIdx = 0
	fp.maxIdx = fp.bottomIdx(0)
}
func (fp FilePicker) Update(msg tea.Msg) (FilePicker, tea.Cmd) {
	switch msg := msg.(type) {
	case execMsg:
		c := exec.Command("micro", msg.path)
		return fp, tea.ExecProcess(c, func(err error) tea.Msg {
			return errorMsg{err}
		})
	case readDirMsg:
		fp.files = msg.entries
		fp.minIdx = 0
		fp.maxIdx = max(fp.maxIdx, fp.bottomIdx(fp.minIdx))
		return fp, tea.Batch(headerCmd(fp.CurrentDir, fp.selectedIdx, len(fp.files)), errorCmd(nil))
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, fp.KeyMap.GoToTop):
			if len(fp.files) > 0 {
				fp.selectedIdx = 0
				fp.minIdx = 0
				fp.maxIdx = fp.bottomIdx(0)
			}
			return fp, headerCmd(fp.CurrentDir, fp.selectedIdx, len(fp.files))
		case key.Matches(msg, fp.KeyMap.GoToLast):
			if len(fp.files) > 0 {
				fp.selectedIdx = len(fp.files) - 1
				fp.minIdx = max(len(fp.files)-fp.Height(), 0)
				fp.maxIdx = len(fp.files) - 1
			}
			return fp, headerCmd(fp.CurrentDir, fp.selectedIdx, len(fp.files))
		case key.Matches(msg, fp.KeyMap.Down):
			if len(fp.files) != 0 {
				fp.selectedIdx++
				if fp.selectedIdx >= len(fp.files) {
					fp.selectedIdx = len(fp.files) - 1
				}
				if fp.selectedIdx > fp.maxIdx {
					fp.maxIdx++
					fp.minIdx++
				}
			}
			return fp, headerCmd(fp.CurrentDir, fp.selectedIdx, len(fp.files))
		case key.Matches(msg, fp.KeyMap.Up):
			fp.selectedIdx--
			if fp.selectedIdx < 0 {
				fp.selectedIdx = 0
			}
			if fp.selectedIdx < fp.minIdx {
				fp.minIdx--
				fp.maxIdx--
			}
			return fp, headerCmd(fp.CurrentDir, fp.selectedIdx, len(fp.files))
		case key.Matches(msg, fp.KeyMap.Back):
			parentDir := filepath.Dir(fp.CurrentDir)
			if fp.CurrentDir == parentDir {
				break
			}
			fp.Reset()
			fp.CurrentDir = parentDir
			return fp, fp.readDir(fp.CurrentDir)
		case key.Matches(msg, fp.KeyMap.Open):
			if len(fp.files) == 0 {
				break
			}
			file := fp.files[fp.selectedIdx]
			info, err := file.Info()
			if err != nil {
				break
			}

			isSymlink := info.Mode()&os.ModeSymlink != 0
			isDir := file.IsDir()

			if isSymlink {
				symlinkPath, _ := filepath.EvalSymlinks(filepath.Join(fp.CurrentDir, file.Name()))

				stat, err := os.Stat(symlinkPath)
				if err != nil {
					break
				}
				isDir = stat.IsDir()
			}

			path := filepath.Join(fp.CurrentDir, file.Name())

			if !isDir {
				ok, err := canOpen(path)
				if err != nil {
					return fp, errorCmd(err)
				}
				if !ok {
					return fp, errorCmd(fmt.Errorf("Cannot open file: invalid format"))
				}
				return fp, execCmd(path)
			}
			if _, err := os.ReadDir(path); err != nil {
				return fp, errorCmd(err)
			}

			fp.CurrentDir = filepath.Join(fp.CurrentDir, file.Name())
			fp.Reset()
			return fp, fp.readDir(fp.CurrentDir)
		case key.Matches(msg, fp.KeyMap.ToggleHidden):

			fp.ShowHidden = !fp.ShowHidden
			fp.Reset()
			return fp, fp.readDir(fp.CurrentDir)
		case key.Matches(msg, fp.KeyMap.Extended):

			fp.extended = !fp.extended
			return fp, fp.readDir(fp.CurrentDir)
		}
	}
	return fp, nil
}

func (fp FilePicker) View() string {
	if len(fp.files) == 0 {
		return fp.Styles.EmptyDirectory.String()
	}
	var s strings.Builder

	for i, f := range fp.files {
		if i < fp.minIdx || i > fp.maxIdx {
			continue
		}
		info, err := f.Info()
		if err != nil {
			if fp.selectedIdx == i {
				s.WriteString(fp.Styles.Cursor.Render(fp.Cursor))
			} else {
				s.WriteString(fp.Styles.Cursor.Render("  "))
			}
			s.WriteString(fp.Styles.Permission.Render(f.Type().String()))
			s.WriteString(fp.Styles.Ownership.Render("? ?"))
			s.WriteString(fp.Styles.FileSize.Render("?"))
			s.WriteString(" " + f.Name())

			s.WriteRune('\n')
			continue
		}

		path := filepath.Join(fp.CurrentDir, f.Name())
		isSymlink := info.Mode()&os.ModeSymlink != 0
		symlinkPath := ""
		name := f.Name()
		size := strings.Replace(humanize.Bytes(uint64(info.Size())), " ", "", 1)

		if isSymlink {
			symlinkPath, err = os.Readlink(path)
			if err != nil {
				symlinkPath = "[permission denied]"
			}
		}

		var stat unix.Stat_t
		statErr := unix.Lstat(path, &stat)

		ownerName := "?"
		groupName := "?"

		if statErr == nil {
			owner, err := user.LookupId(
				strconv.FormatUint(uint64(stat.Uid), 10),
			)
			if err == nil {
				ownerName = owner.Username
			}

			group, err := user.LookupGroupId(
				strconv.FormatUint(uint64(stat.Gid), 10),
			)
			if err == nil {
				groupName = group.Name
			}
		}

		style := fp.Styles.File
		if f.IsDir() {
			style = fp.Styles.Directory
		} else if isSymlink {
			style = fp.Styles.Symlink
		}

		fileName := style.Render(name)
		if isSymlink {
			fileName += " -> " + symlinkPath
		}
		cursor := fp.Styles.Cursor
		if fp.selectedIdx == i {
			s.WriteString(cursor.Render(fp.Cursor))
		} else {
			s.WriteString(cursor.Render("  "))
		}
		s.WriteString(fp.Styles.Permission.Render(info.Mode().String()))
		s.WriteString(fp.Styles.Ownership.Render(ownerName + " " + groupName))
		s.WriteString(fp.Styles.FileSize.Render(size))
		s.WriteString(" " + fileName)

		s.WriteRune('\n')

		if fp.extended && fp.selectedIdx == i {
			s.WriteRune('\n')
			fmt.Fprintf(&s, "  Size: %s/%d\n  Uid: ( %d / %s ) Gid: ( %d / %s )\n\n", size, info.Size(), stat.Uid, ownerName, stat.Gid, groupName)
		}
	}

	return s.String()
}
func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}
func canOpen(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}
	data := buf[:n]

	if bytes.Contains(data, []byte{0}) {
		return false, nil
	}

	return utf8.Valid(data), nil
}
