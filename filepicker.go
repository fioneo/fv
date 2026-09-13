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
	"time"
	"unicode/utf8"
)

var (
	marginLeft = 2
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
	userCache      map[uint32]string
	groupCache     map[uint32]string
	filter         string
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
		userCache:      map[uint32]string{},
		groupCache:     map[uint32]string{},
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
	ToggleHidden key.Binding
	Extended     key.Binding
}

type Styles struct {
	DisabledCursor   lipgloss.Style
	Cursor           lipgloss.Style
	Symlink          lipgloss.Style
	LinkDest         lipgloss.Style
	BrokenSymlink    lipgloss.Style
	Pipe             lipgloss.Style
	Socket           lipgloss.Style
	IrregularFile    lipgloss.Style
	Device           lipgloss.Style
	Directory        lipgloss.Style
	File             lipgloss.Style
	Ownership        lipgloss.Style
	DisabledFile     lipgloss.Style
	Permission       lipgloss.Style
	Selected         lipgloss.Style
	DisabledSelected lipgloss.Style
	FileSize         lipgloss.Style
	EmptyDirectory   lipgloss.Style
	Nlink            lipgloss.Style
	Date             lipgloss.Style
}

func DefaultStyles() Styles {
	return Styles{
		DisabledCursor:   lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
		Cursor:           lipgloss.NewStyle().Foreground(lipgloss.Color("#D5CFEB")),
		Symlink:          lipgloss.NewStyle().Foreground(lipgloss.Color("36")),
		LinkDest:         lipgloss.NewStyle().Foreground(lipgloss.Color("#D5CFEB")),
		BrokenSymlink:    lipgloss.NewStyle().Foreground(lipgloss.Color("#9E190D")),
		Pipe:             lipgloss.NewStyle().Foreground(lipgloss.Color("#E0BB7A")),
		Socket:           lipgloss.NewStyle().Foreground(lipgloss.Color("#F3C2EA")),
		IrregularFile:    lipgloss.NewStyle().Foreground(lipgloss.Color("#D1448B")),
		Device:           lipgloss.NewStyle().Foreground(lipgloss.Color("#DEFB3D")),
		Directory:        lipgloss.NewStyle().Foreground(lipgloss.Color("#2472B5")),
		File:             lipgloss.NewStyle(),
		Ownership:        lipgloss.NewStyle().Foreground(lipgloss.Color("#B5B1FB")),
		DisabledFile:     lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		DisabledSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
		Permission:       lipgloss.NewStyle().Foreground(lipgloss.Color("#B1D2FB")),
		Selected:         lipgloss.NewStyle().Background(lipgloss.Color("#240641")).Bold(true),
		FileSize:         lipgloss.NewStyle().Foreground(lipgloss.Color("#C2D2F9")),
		EmptyDirectory:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")).SetString("No Files Found.\n").MarginLeft(marginLeft),
		Nlink:            lipgloss.NewStyle().Foreground(lipgloss.Color("#CFE5EB")),
		Date:             lipgloss.NewStyle().Foreground(lipgloss.Color("#CFE5EB")),
	}
}
func DefaultKeyMap() KeyMap {
	return KeyMap{
		GoToTop:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "first")),
		GoToLast:     key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "last")),
		Down:         key.NewBinding(key.WithKeys("j", "down", "ctrl+n"), key.WithHelp("j", "down")),
		Up:           key.NewBinding(key.WithKeys("k", "up", "ctrl+p"), key.WithHelp("k", "up")),
		Back:         key.NewBinding(key.WithKeys("h", "backspace", "left"), key.WithHelp("h", "back")),
		Open:         key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "open")),
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
		trim := strings.TrimSpace(fp.filter)
		var visibleEntries []os.DirEntry
		for _, dirEntry := range dirEntries {
			if trim != "" {
				if !strings.Contains(strings.ToLower(dirEntry.Name()), strings.ToLower(trim)) {
					continue
				}
			}
			if !fp.ShowHidden && isHidden(dirEntry.Name()) {
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
	case filterResetMsg:
		fp.filter = ""
		return fp, fp.readDir(fp.CurrentDir)
	case filterMsg:
		fp.Reset()
		fp.filter = msg.filter
		return fp, fp.readDir(fp.CurrentDir)
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
			if len(fp.files) == 0 {
				break
			}
			fp.selectedIdx++
			if fp.selectedIdx >= len(fp.files) {
				fp.selectedIdx = len(fp.files) - 1
			}
			if fp.selectedIdx > fp.maxIdx {
				fp.maxIdx++
				fp.minIdx++
			}
			return fp, headerCmd(fp.CurrentDir, fp.selectedIdx, len(fp.files))
		case key.Matches(msg, fp.KeyMap.Up):
			if len(fp.files) == 0 {
				break
			}
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
			return fp, tea.Batch(fp.readDir(fp.CurrentDir), filterResetCmd())
		case key.Matches(msg, fp.KeyMap.Open):
			if len(fp.files) == 0 {
				break
			}
			file := fp.files[fp.selectedIdx]
			info, err := file.Info()
			if err != nil {
				return fp, errorCmd(err)
			}

			isSymlink := info.Mode()&os.ModeSymlink != 0
			isDir := file.IsDir()

			if isSymlink {
				symlinkPath, err := filepath.EvalSymlinks(filepath.Join(fp.CurrentDir, file.Name()))
				if err != nil {
					return fp, errorCmd(err)
				}

				stat, err := os.Stat(symlinkPath)
				if err != nil {
					return fp, errorCmd(err)
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
			return fp, tea.Batch(fp.readDir(fp.CurrentDir), filterResetCmd())
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

	maxOwnerLen := 0
	maxSizeLen := 0
	maxPermLen := 0
	maxNlinkLen := 0

	for i := fp.minIdx; i <= fp.maxIdx && i < len(fp.files); i++ {
		f := fp.files[i]
		path := filepath.Join(fp.CurrentDir, f.Name())

		var stat unix.Stat_t
		if err := unix.Lstat(path, &stat); err == nil {
			ownerName := fp.LookupUser(stat.Uid)
			groupName := fp.LookupGroup(stat.Gid)

			ownerStrLen := utf8.RuneCountInString(ownerName + " " + groupName)
			if ownerStrLen > maxOwnerLen {
				maxOwnerLen = ownerStrLen
			}
			nlinkLen := utf8.RuneCountInString(strconv.FormatUint(stat.Nlink, 10))
			if nlinkLen > maxNlinkLen {
				maxNlinkLen = nlinkLen
			}
		}

		if info, err := f.Info(); err == nil {
			sizeStr := strings.Replace(humanize.Bytes(uint64(info.Size())), " ", "", 1)
			if len(sizeStr) > maxSizeLen {
				maxSizeLen = len(sizeStr)
			}
			permStrLen := utf8.RuneCountInString(info.Mode().String())
			if permStrLen > maxPermLen {
				maxPermLen = permStrLen
			}
		}
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
		name := f.Name()
		size := strings.Replace(humanize.Bytes(uint64(info.Size())), " ", "", 1)

		var stat unix.Stat_t

		ownerName := "?"
		groupName := "?"

		if err := unix.Lstat(path, &stat); err == nil {
			ownerName = fp.LookupUser(stat.Uid)
			groupName = fp.LookupGroup(stat.Gid)
		}

		name = fp.RenderByFileType(stat, name)

		cursor := fp.Styles.Cursor
		if fp.selectedIdx == i {
			s.WriteString(cursor.Render(fp.Cursor))
		} else {
			s.WriteString(cursor.Render("  "))
		}

		rawOwnership := ownerName + " " + groupName
		rawPerm := info.Mode().String()
		rawNLink := strconv.FormatUint(stat.Nlink, 10)

		date := fp.Styles.Date.Render(
			time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec).Format("Jan _2 15:04"),
		)
		ownershipRendered := fp.Styles.Ownership.Render(
			fmt.Sprintf("%-*s", maxOwnerLen, rawOwnership),
		)
		sizeRendered := fp.Styles.FileSize.Render(
			fmt.Sprintf("%*s", maxSizeLen, size),
		)
		permRendered := fp.Styles.Permission.Render(
			fmt.Sprintf("%-*s", maxPermLen, rawPerm),
		)
		nlinkRendered := fp.Styles.Nlink.Render(
			fmt.Sprintf("%*s", maxNlinkLen, rawNLink),
		)

		WriteRow(
			&s,
			permRendered,
			nlinkRendered,
			ownershipRendered,
			sizeRendered,
			date,
			name,
		)
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
func (fp FilePicker) LookupUser(uid uint32) string {
	if user, ok := fp.userCache[uid]; ok {
		return user
	}
	usr := strconv.FormatUint(uint64(uid), 10)
	if u, err := user.LookupId(usr); err == nil {
		usr = u.Username
	}
	fp.userCache[uid] = usr
	return usr
}

func (fp FilePicker) LookupGroup(gid uint32) string {
	if group, ok := fp.groupCache[gid]; ok {
		return group
	}
	group := strconv.FormatUint(uint64(gid), 10)
	if g, err := user.LookupGroupId(group); err == nil {
		group = g.Name
	}
	fp.groupCache[gid] = group
	return group
}

func WriteRow(w io.Writer, perm, nlink, ownership, size, date, name string) {
	fmt.Fprintf(w, "%s %s %s %s %s %s", perm, nlink, ownership, size, date, name)
}

func (fp FilePicker) RenderByFileType(stat unix.Stat_t, name string) string {
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		return fp.Styles.File.Render(name)

	case unix.S_IFDIR:
		return fp.Styles.Directory.Render(name)

	case unix.S_IFLNK:
		symlinkPath, err := os.Readlink(filepath.Join(fp.CurrentDir, name))
		if err != nil {
			return fp.Styles.BrokenSymlink.Render(name)
		}
		target := " -> " + fp.Styles.LinkDest.Render(symlinkPath)
		return fp.Styles.Symlink.Render(name) + target

	case unix.S_IFIFO:
		return fp.Styles.Pipe.Render(name)

	case unix.S_IFSOCK:
		return fp.Styles.Socket.Render(name)

	case unix.S_IFCHR, unix.S_IFBLK:
		return fp.Styles.Device.Render(name)
	default:
		return fp.Styles.IrregularFile.Render(name)
	}
}
