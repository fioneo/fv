package main

import (
	"bytes"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"cmp"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/fvbommel/sortorder"
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

type Cache[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

func NewCache[K comparable, V any]() *Cache[K, V] {
	return &Cache[K, V]{items: make(map[K]V)}
}
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.items[key]
	return val, ok
}

func (c *Cache[K, V]) Set(key K, val V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = val
}

type FilePicker struct {
	Config         Config
	CurrentDir     string
	selectedIdx    int
	ShowHidden     bool
	Cursor         string
	files          []File
	viewportHeight int
	maxIdx         int
	minIdx         int
	userCache      *Cache[uint32, string]
	groupCache     *Cache[uint32, string]
	filesCache     *Cache[string, filesCacheEntry]
	filter         string
	sortBy         SortConfig
	less           bool
	KeyMap         KeyMap
	Styles         Styles
	extended       bool
}

func NewFilePicker(cfg Config) FilePicker {
	return FilePicker{
		CurrentDir:     cfg.Settings.CurrentDir,
		selectedIdx:    0,
		ShowHidden:     cfg.Settings.ShowHidden,
		Cursor:         cfg.Settings.Cursor,
		viewportHeight: 0,
		maxIdx:         0,
		minIdx:         0,
		userCache:      NewCache[uint32, string](),
		groupCache:     NewCache[uint32, string](),
		filesCache:     NewCache[string, filesCacheEntry](),
		sortBy:         SortConfig{},
		KeyMap:         cfg.KeyMap.FilePickerKeyMap(),
		Styles:         cfg.Styles.FilePickerStyles(),
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
	status any
}

func statusCmd(status any) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{status: status}
	}
}

type filePickerStateMsg struct {
	currentDir  string
	filesCount  int
	selectedIdx int
	sortBy      SortConfig
}

func filePickerStateCmd(state FilePicker) tea.Cmd {
	return func() tea.Msg {
		return filePickerStateMsg{
			currentDir:  state.CurrentDir,
			filesCount:  len(state.files),
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
	Less            key.Binding
}

type Styles struct {
	DisabledCursor   lipgloss.Style
	Cursor           lipgloss.Style
	Symlink          lipgloss.Style
	LinkDest         lipgloss.Style
	BrokenSymlink    lipgloss.Style
	FileNotExist     lipgloss.Style
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
	Mtime            lipgloss.Style
}

func (fp *FilePicker) SetHeight(h int) {
	fp.viewportHeight = max(h, 1)

	if len(fp.files) == 0 {
		fp.minIdx = 0
		fp.maxIdx = 0
		return
	}

	fp.selectedIdx = min(max(fp.selectedIdx, 0), len(fp.files)-1)

	if fp.selectedIdx < fp.minIdx {
		fp.minIdx = fp.selectedIdx
	}
	if fp.selectedIdx > fp.bottomIdx(fp.minIdx) {
		fp.minIdx = fp.selectedIdx - fp.viewportHeight + 1
	}

	maxMinIdx := max(len(fp.files)-fp.viewportHeight, 0)
	fp.minIdx = min(max(fp.minIdx, 0), maxMinIdx)
	fp.maxIdx = min(fp.bottomIdx(fp.minIdx), len(fp.files)-1)
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
		if cachedFiles, ok := fp.filesCache.Get(dir); ok && time.Since(cachedFiles.updatedAt) < cacheTTL {
			return readCurrentDirMsg{dir: dir, entry: cachedFiles}
		}

		dirEntries, err := os.ReadDir(dir)
		if err != nil {
			return statusMsg{status: err.Error()}
		}

		var files []File
		for _, f := range dirEntries {
			var stat unix.Stat_t
			path := filepath.Join(dir, f.Name())
			if err := unix.Lstat(path, &stat); err != nil {
				continue
			}
			info, err := os.Lstat(path)
			if err != nil {
				continue
			}
			files = append(files, File{
				stat: stat,
				info: info,
			})
		}

		filesEntry := filesCacheEntry{
			files:     files,
			updatedAt: time.Now(),
		}
		fp.filesCache.Set(dir, filesEntry)
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

		if a.info.IsDir() != b.info.IsDir() {
			if a.info.IsDir() {
				return -1
			}
			return 1

		}
		switch fp.sortBy.field {
		case sortByName:
			if sortorder.NaturalLess(strings.ToLower(a.info.Name()), strings.ToLower(b.info.Name())) {
				res = -1
			} else if sortorder.NaturalLess(strings.ToLower(b.info.Name()), strings.ToLower(a.info.Name())) {
				res = 1
			} else {
				res = 0
			}
		case sortBySize:
			res = cmp.Compare(a.info.Size(), b.info.Size())
		case sortByDate:
			res = cmp.Compare(a.stat.Mtim.Sec, b.stat.Mtim.Sec)
		}

		if res == 0 {
			if sortorder.NaturalLess(strings.ToLower(a.info.Name()), strings.ToLower(b.info.Name())) {
				res = -1
			} else if sortorder.NaturalLess(strings.ToLower(b.info.Name()), strings.ToLower(a.info.Name())) {
				res = 1
			} else {
				res = 0
			}
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
			return statusCmd(err)
		})
	case readCurrentDirMsg:
		if msg.dir == fp.CurrentDir {
			fp.files = fp.filterFiles(msg.entry.files, fp.filter)
			fp.sortFiles()

			fp.minIdx = 0
			fp.maxIdx = min(
				fp.bottomIdx(fp.minIdx),
				len(fp.files)-1,
			)
		}
		return fp, tea.Batch(filePickerStateCmd(fp), statusCmd(""))
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, fp.KeyMap.GoToTop):
			if len(fp.files) > 0 {
				fp.selectedIdx = 0
				fp.minIdx = 0
				fp.maxIdx = fp.bottomIdx(0)
			}
			return fp, tea.Batch(filePickerStateCmd(fp), statusCmd(""))
		case key.Matches(msg, fp.KeyMap.GoToLast):
			if len(fp.files) > 0 {
				fp.selectedIdx = len(fp.files) - 1
				fp.minIdx = max(len(fp.files)-fp.Height(), 0)
				fp.maxIdx = len(fp.files) - 1
			}
			return fp, tea.Batch(filePickerStateCmd(fp), statusCmd(""))
		case key.Matches(msg, fp.KeyMap.Down):
			if len(fp.files) == 0 {
				break
			}

			if fp.selectedIdx < len(fp.files)-1 {
				fp.selectedIdx++
			}

			if fp.selectedIdx > fp.maxIdx {
				fp.minIdx++
				fp.maxIdx++
			}

			fp.maxIdx = min(fp.maxIdx, len(fp.files)-1)

			return fp, tea.Batch(filePickerStateCmd(fp), statusCmd(""))
		case key.Matches(msg, fp.KeyMap.Up):
			if len(fp.files) == 0 {
				break
			}

			if fp.selectedIdx > 0 {
				fp.selectedIdx--
			}

			if fp.selectedIdx < fp.minIdx {
				fp.minIdx--
				fp.maxIdx--
			}

			fp.minIdx = max(fp.minIdx, 0)
			fp.maxIdx = min(fp.maxIdx, len(fp.files)-1)

			return fp, tea.Batch(filePickerStateCmd(fp), statusCmd(""))
		case key.Matches(msg, fp.KeyMap.Back):
			parentDir := filepath.Dir(fp.CurrentDir)
			if fp.CurrentDir == parentDir {
				break
			}
			fp.Reset()
			fp.CurrentDir = parentDir
			return fp, tea.Batch(fp.readCurrentDir(), filterResetCmd(), statusCmd(""))
		case key.Matches(msg, fp.KeyMap.Open):
			if len(fp.files) == 0 {
				break
			}
			file := fp.files[fp.selectedIdx]

			isSymlink := file.info.Mode()&os.ModeSymlink != 0
			isDir := file.info.IsDir()
			path := filepath.Join(fp.CurrentDir, file.info.Name())
			if isSymlink {
				symlinkPath, err := filepath.EvalSymlinks(filepath.Join(fp.CurrentDir, file.info.Name()))
				if err != nil {
					return fp, statusCmd(err)
				}

				stat, err := os.Stat(symlinkPath)
				if err != nil {
					return fp, statusCmd(err)
				}
				path = symlinkPath
				isDir = stat.IsDir()
			}

			if !isDir {
				ok, err := canOpen(path)
				if err != nil {
					return fp, statusCmd(err.Error())
				}
				if !ok {
					return fp, statusCmd(fmt.Errorf("Cannot open file: invalid format"))
				}
				return fp, execCmd(path)
			}

			if _, err := os.ReadDir(path); err != nil {
				return fp, statusCmd(err)
			}

			fp.CurrentDir = filepath.Join(fp.CurrentDir, file.info.Name())
			fp.Reset()
			return fp, tea.Batch(fp.readCurrentDir(), filterResetCmd(), statusCmd(""))
		case key.Matches(msg, fp.KeyMap.ToggleHidden):
			fp.ShowHidden = !fp.ShowHidden
			fp.Reset()
			if cached, ok := fp.filesCache.Get(fp.CurrentDir); ok {
				fp.files = fp.filterFiles(cached.files, fp.filter)
			}
			return fp, tea.Batch(filePickerStateCmd(fp), statusCmd(""))
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
			return fp, tea.Batch(filePickerStateCmd(fp), statusCmd(""))
		case key.Matches(msg, fp.KeyMap.ToggleSortOrder):
			fp.sortBy.order = fp.sortBy.order.Reverse()
			fp.sortFiles()
			return fp, tea.Batch(filePickerStateCmd(fp), statusCmd(""))
		case key.Matches(msg, fp.KeyMap.Extended):
			fp.extended = !fp.extended
			return fp, nil
		case key.Matches(msg, fp.KeyMap.Less):
			fp.less = !fp.less
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

	if !fp.less {

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

	}

	var s strings.Builder
	for i, f := range fp.files {
		if i < fp.minIdx || i > fp.maxIdx {
			continue
		}

		info := f.info
		stat := f.stat

		cursor := fp.Styles.Cursor
		if fp.selectedIdx == i {
			s.WriteString(cursor.Render(fp.Cursor))
		} else {
			s.WriteString(cursor.Render("  "))
		}

		name := info.Name()
		symName := name
		symlinkPath, symErr := os.Readlink(filepath.Join(fp.CurrentDir, name))
		if symErr == nil {
			symName += " -> " + symlinkPath
		}

		size := strings.Replace(humanize.Bytes(uint64(info.Size())), " ", "", 1)

		ownerName := fp.LookupUser(stat.Uid)
		groupName := fp.LookupGroup(stat.Gid)

		rawSize := fmt.Sprintf("%*s", maxSizeLen, size)
		rawOwnership := fmt.Sprintf("%-*s", maxOwnerLen, ownerName+" "+groupName)
		rawPerm := fmt.Sprintf("%-*s", maxPermLen, info.Mode().String())
		rawNlink := fmt.Sprintf("%*s", maxNlinkLen, strconv.FormatUint(f.stat.Nlink, 10))
		mtime := time.Unix(f.stat.Mtim.Sec, f.stat.Mtim.Nsec).Format("Jan _2 15:04")

		mtimeRendered := fp.Styles.Mtime.Render(
			mtime,
		)
		ownershipRendered := fp.Styles.Ownership.Render(
			rawOwnership,
		)
		sizeRendered := fp.Styles.FileSize.Render(
			rawSize,
		)
		permRendered := fp.Styles.Permission.Render(
			rawPerm,
		)
		nlinkRendered := fp.Styles.Nlink.Render(
			rawNlink,
		)
		nameRendered := fp.RenderByFileType(stat, name)

		if fp.selectedIdx == i {
			row := fmt.Sprintf("%s %s %s %s %s %s", rawPerm, rawNlink, rawOwnership, rawSize, mtime, symName)
			s.WriteString(fp.Styles.Selected.Render(row))
		} else if fp.less {
			row := fp.RenderByFileType(stat, name)
			if fp.selectedIdx == i {
				row = fp.Styles.Selected.Render(symName)
			}
			s.WriteString(row)
		} else {
			WriteRow(
				&s,
				permRendered,
				nlinkRendered,
				ownershipRendered,
				sizeRendered,
				mtimeRendered,
				nameRendered,
			)
		}

		if i != len(fp.files)-1 && i < fp.maxIdx {
			s.WriteRune('\n')
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

func (fp *FilePicker) LookupUser(uid uint32) string {
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

func (fp *FilePicker) LookupGroup(gid uint32) string {
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
		symlinkPath, symErr := os.Readlink(filepath.Join(fp.CurrentDir, name))
		if symErr != nil {
			return fp.Styles.BrokenSymlink.Render(name)
		}
		arrow := " -> "
		if err := unix.Lstat(filepath.Join(fp.CurrentDir, symlinkPath), &unix.Stat_t{}); err != nil {
			return fp.Styles.BrokenSymlink.Render(name) + arrow + fp.Styles.FileNotExist.Render(symlinkPath)
		}
		return fp.Styles.Symlink.Render(name) + arrow + fp.Styles.LinkDest.Render(symlinkPath)
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
