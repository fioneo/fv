package main

import (
	"bytes"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"cmp"
	"fmt"
	"github.com/dustin/go-humanize"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type sortField int
type sortOrder int

const (
	sortByName sortField = iota
	sortBySize
	sortByDate
)
const (
	sortAsc sortOrder = iota
	sortDesc
)

var (
	marginLeft = 2
	cacheTTL   = 5 * time.Second
)

type SortConfig struct {
	field sortField
	order sortOrder
}

func (c SortConfig) String() string {
	order := "⬆"
	if c.order == sortDesc {
		order = "⬇"

	}
	switch c.field {
	case sortByName:
		return "Name " + order
	case sortBySize:
		return "Size " + order
	case sortByDate:
		return "Time " + order
	default:
		return ""
	}
}
func (o sortOrder) Reverse() sortOrder {
	if o == sortAsc {
		return sortDesc
	}
	return sortAsc
}

type SafeCache struct {
	mu    sync.RWMutex
	items map[uint32]string
}

func NewSafeCache() *SafeCache {
	return &SafeCache{items: make(map[uint32]string)}
}

func (c *SafeCache) Get(id uint32) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.items[id]
	return val, ok
}

func (c *SafeCache) Set(id uint32, val string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[id] = val
}

type FilePicker struct {
	CurrentDir     string
	selectedIdx    int
	ShowHidden     bool
	Cursor         string
	files          []File
	viewportHeight int
	maxIdx         int
	minIdx         int
	userCache      *SafeCache
	groupCache     *SafeCache
	filesCache     map[string]filesCacheEntry
	filter         string
	sortBy         SortConfig
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
		userCache:      NewSafeCache(),
		groupCache:     NewSafeCache(),
		filesCache:     map[string]filesCacheEntry{},
		sortBy:         SortConfig{},
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

type statusMsg struct {
	status string
}

func statusCmd(status string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{status: status}
	}
}

type filePickerStateMsg struct {
	currentDir  string
	files       []File
	selectedIdx int
	sortBy      SortConfig
}

func filePickerStateCmd(state FilePicker) tea.Cmd {
	return func() tea.Msg {
		return filePickerStateMsg{
			currentDir:  state.CurrentDir,
			files:       state.files,
			selectedIdx: state.selectedIdx,
			sortBy:      state.sortBy,
		}
	}
}

type readCurrentDirMsg struct {
	dir   string
	entry filesCacheEntry
}

type filesCacheEntry struct {
	files     []File
	updatedAt time.Time
	status    string
}

type File struct {
	stat unix.Stat_t
	info os.FileInfo
}

type KeyMap struct {
	GoToTop         key.Binding
	GoToLast        key.Binding
	Down            key.Binding
	Up              key.Binding
	Back            key.Binding
	Open            key.Binding
	ToggleHidden    key.Binding
	Extended        key.Binding
	ToggleSortField key.Binding
	ToggleSortOrder key.Binding
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
		GoToTop:         key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "first")),
		GoToLast:        key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "last")),
		Down:            key.NewBinding(key.WithKeys("j", "down", "ctrl+n"), key.WithHelp("j", "down")),
		Up:              key.NewBinding(key.WithKeys("k", "up", "ctrl+p"), key.WithHelp("k", "up")),
		Back:            key.NewBinding(key.WithKeys("h", "backspace", "left"), key.WithHelp("h", "back")),
		Open:            key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "open")),
		ToggleHidden:    key.NewBinding(key.WithKeys("i", "ctrl+i"), key.WithHelp("i", "toggle hidden files")),
		ToggleSortField: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "toggle sort field")),
		ToggleSortOrder: key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "toggle sort order")),
		Extended:        key.NewBinding(key.WithKeys("e", "ctrl+e"), key.WithHelp("e", "extended look on selected file")),
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

func (fp FilePicker) readCurrentDir() tea.Cmd {
	return func() tea.Msg {
		dir := fp.CurrentDir
		if cachedFiles, ok := fp.filesCache[dir]; ok && time.Since(cachedFiles.updatedAt) < cacheTTL {
			return readCurrentDirMsg{dir: dir, entry: cachedFiles}
		}

		dirEntries, err := os.ReadDir(dir)
		if err != nil {
			return statusMsg{status: err.Error()}
		}

		var files []File
		skipped := 0
		for _, f := range dirEntries {
			var stat unix.Stat_t
			path := filepath.Join(dir, f.Name())
			skipped++
			if err := unix.Lstat(path, &stat); err != nil {
				continue
			}
			info, err := os.Lstat(path)
			if err != nil {
				continue
			}
			skipped--
			files = append(files, File{
				stat: stat,
				info: info,
			})
		}

		filesEntry := filesCacheEntry{
			files:     files,
			updatedAt: time.Now(),
			status:    fmt.Sprintf("! skipped %d files", skipped),
		}

		return readCurrentDirMsg{dir: dir, entry: filesEntry}
	}
}

func (fp FilePicker) filterFiles(files []File, filter string) []File {
	result := make([]File, 0, len(files))
	trim := strings.TrimSpace(filter)

	for _, f := range files {
		if !fp.ShowHidden && isHidden(f.info.Name()) {
			continue
		}

		if trim != "" && !strings.Contains(strings.ToLower(f.info.Name()), trim) {
			continue
		}

		result = append(result, f)
	}

	return result
}

func (fp *FilePicker) sortFiles() {
	slices.SortFunc(fp.files, func(a File, b File) int {
		var res int

		switch fp.sortBy.field {
		case sortByName:
			res = strings.Compare(strings.ToLower(a.info.Name()), strings.ToLower(b.info.Name()))
		case sortBySize:
			res = cmp.Compare(a.info.Size(), b.info.Size())
		case sortByDate:
			res = cmp.Compare(a.stat.Mtim.Sec, b.stat.Mtim.Sec)
		}

		if res == 0 {
			res = strings.Compare(strings.ToLower(a.info.Name()), strings.ToLower(b.info.Name()))
		}

		if fp.sortBy.order == sortDesc {
			return -res
		}

		return res
	})
}

func (fp FilePicker) Init() tea.Cmd {
	return fp.readCurrentDir()
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
		return fp, fp.readCurrentDir()
	case filterMsg:
		fp.Reset()
		fp.filter = msg.filter
		return fp, fp.readCurrentDir()
	case execMsg:
		c := exec.Command("micro", msg.path)
		return fp, tea.ExecProcess(c, func(err error) tea.Msg {
			return statusMsg{status: "program exit"}
		})
	case readCurrentDirMsg:
		fp.filesCache[msg.dir] = msg.entry
		if msg.dir == fp.CurrentDir {
			fp.files = fp.filterFiles(msg.entry.files, fp.filter)
			fp.sortFiles()
			fp.minIdx = 0
			fp.maxIdx = max(fp.maxIdx, fp.bottomIdx(fp.minIdx))
		}
		return fp, tea.Batch(filePickerStateCmd(fp), statusCmd(msg.entry.status))
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, fp.KeyMap.GoToTop):
			if len(fp.files) > 0 {
				fp.selectedIdx = 0
				fp.minIdx = 0
				fp.maxIdx = fp.bottomIdx(0)
			}
			return fp, filePickerStateCmd(fp)
		case key.Matches(msg, fp.KeyMap.GoToLast):
			if len(fp.files) > 0 {
				fp.selectedIdx = len(fp.files) - 1
				fp.minIdx = max(len(fp.files)-fp.Height(), 0)
				fp.maxIdx = len(fp.files) - 1
			}
			return fp, filePickerStateCmd(fp)
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
			return fp, filePickerStateCmd(fp)
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
			return fp, filePickerStateCmd(fp)
		case key.Matches(msg, fp.KeyMap.Back):
			parentDir := filepath.Dir(fp.CurrentDir)
			if fp.CurrentDir == parentDir {
				break
			}
			fp.Reset()
			fp.CurrentDir = parentDir
			return fp, tea.Batch(fp.readCurrentDir(), filterResetCmd())
		case key.Matches(msg, fp.KeyMap.Open):
			if len(fp.files) == 0 {
				break
			}
			file := fp.files[fp.selectedIdx]

			isSymlink := file.info.Mode()&os.ModeSymlink != 0
			isDir := file.info.IsDir()

			if isSymlink {
				symlinkPath, err := filepath.EvalSymlinks(filepath.Join(fp.CurrentDir, file.info.Name()))
				if err != nil {
					return fp, statusCmd(err.Error())
				}

				stat, err := os.Stat(symlinkPath)
				if err != nil {
					return fp, statusCmd(err.Error())
				}
				isDir = stat.IsDir()
			}

			path := filepath.Join(fp.CurrentDir, file.info.Name())

			if !isDir {
				ok, err := canOpen(path)
				if err != nil {
					return fp, statusCmd(err.Error())
				}
				if !ok {
					return fp, statusCmd("Cannot open file: invalid format")
				}
				return fp, execCmd(path)
			}

			if _, err := os.ReadDir(path); err != nil {
				return fp, statusCmd(err.Error())
			}

			fp.CurrentDir = filepath.Join(fp.CurrentDir, file.info.Name())
			fp.Reset()
			return fp, tea.Batch(fp.readCurrentDir(), filterResetCmd())
		case key.Matches(msg, fp.KeyMap.ToggleHidden):
			fp.ShowHidden = !fp.ShowHidden
			fp.Reset()
			if cached, ok := fp.filesCache[fp.CurrentDir]; ok {
				fp.files = fp.filterFiles(cached.files, fp.filter)
			}
			return fp, filePickerStateCmd(fp)
		case key.Matches(msg, fp.KeyMap.ToggleSortField):
			switch fp.sortBy.field {
			case sortByName:
				fp.sortBy.field = sortBySize
			case sortBySize:
				fp.sortBy.field = sortByDate
			case sortByDate:
				fp.sortBy.field = sortByName
			}
			fp.sortFiles()
			return fp, filePickerStateCmd(fp)
		case key.Matches(msg, fp.KeyMap.ToggleSortOrder):
			fp.sortBy.order = fp.sortBy.order.Reverse()
			fp.sortFiles()
			return fp, filePickerStateCmd(fp)
		case key.Matches(msg, fp.KeyMap.Extended):
			fp.extended = !fp.extended
			return fp, nil
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

		ownerName := fp.LookupUser(f.stat.Uid)
		groupName := fp.LookupGroup(f.stat.Gid)

		ownerStrLen := utf8.RuneCountInString(ownerName + " " + groupName)
		if ownerStrLen > maxOwnerLen {
			maxOwnerLen = ownerStrLen
		}
		nlinkLen := utf8.RuneCountInString(strconv.FormatUint(f.stat.Nlink, 10))
		if nlinkLen > maxNlinkLen {
			maxNlinkLen = nlinkLen
		}

		sizeStr := strings.Replace(humanize.Bytes(uint64(f.info.Size())), " ", "", 1)
		if len(sizeStr) > maxSizeLen {
			maxSizeLen = len(sizeStr)
		}
		permStrLen := utf8.RuneCountInString(f.info.Mode().String())
		if permStrLen > maxPermLen {
			maxPermLen = permStrLen
		}
	}

	var s strings.Builder
	for i, f := range fp.files {
		if i < fp.minIdx || i > fp.maxIdx {
			continue
		}
		info := f.info
		stat := f.stat

		name := info.Name()
		size := strings.Replace(humanize.Bytes(uint64(info.Size())), " ", "", 1)

		ownerName := fp.LookupUser(stat.Uid)
		groupName := fp.LookupGroup(stat.Gid)

		name = fp.RenderByFileType(stat, name)

		cursor := fp.Styles.Cursor
		if fp.selectedIdx == i {
			s.WriteString(cursor.Render(fp.Cursor))
		} else {
			s.WriteString(cursor.Render("  "))
		}

		rawOwnership := ownerName + " " + groupName
		rawPerm := info.Mode().String()
		rawNLink := strconv.FormatUint(f.stat.Nlink, 10)

		date := fp.Styles.Date.Render(
			time.Unix(f.stat.Mtim.Sec, f.stat.Mtim.Nsec).Format("Jan _2 15:04"),
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
	if u, ok := fp.userCache.Get(uid); ok {
		return u
	}
	usr := strconv.FormatUint(uint64(uid), 10)
	if u, err := user.LookupId(usr); err == nil {
		usr = u.Username
	}
	fp.userCache.Set(uid, usr)
	return usr
}

func (fp FilePicker) LookupGroup(gid uint32) string {
	if g, ok := fp.groupCache.Get(gid); ok {
		return g
	}
	group := strconv.FormatUint(uint64(gid), 10)
	if g, err := user.LookupGroupId(group); err == nil {
		group = g.Name
	}
	fp.groupCache.Set(gid, group)
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
