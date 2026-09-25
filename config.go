package main

import (
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"encoding/json"
	"os"
)

type Config struct {
	Settings SettingsConfig `json:"settings"`
	Styles   ColorConfig    `json:"styles"`
	KeyMap   KeyMapConfig   `json:"keymap"`
}
type SettingsConfig struct {
	CurrentDir string
	ShowHidden bool   `json:"show_hidden"`
	Cursor     string `json:"cursor"`
	ExecTool   string `json:"exec_tool"`
}

type ColorConfig struct {
	ErrorMsg         string `json:"error_msg"`
	DisabledCursor   string `json:"disabled_cursor"`
	Cursor           string `json:"cursor"`
	Symlink          string `json:"symlink"`
	LinkDest         string `json:"link_dest"`
	BrokenSymlink    string `json:"broken_symlink"`
	FileNotExist     string `json:"file_not_exist"`
	Pipe             string `json:"pipe"`
	Socket           string `json:"socket"`
	IrregularFile    string `json:"irregular_file"`
	Device           string `json:"device"`
	Directory        string `json:"directory"`
	File             string `json:"file"`
	Ownership        string `json:"ownership"`
	DisabledFile     string `json:"disabled_file"`
	Permission       string `json:"permission"`
	Selected         string `json:"selected"`
	DisabledSelected string `json:"disabled_selected"`
	FileSize         string `json:"file_size"`
	EmptyDirectory   string `json:"empty_directory"`
	Nlink            string `json:"nlink"`
	Mtime            string `json:"mtime"`
	HeaderSelected   string `json:"header_selected"`
	HeaderFilesCount string `json:"header_files_count"`
	HeaderSortBy     string `json:"header_sort_by"`
}

type KeyBindingConfig struct {
	Keys []string `json:"keys"`
	Help string   `json:"help"`
}

func (k KeyBindingConfig) ToBinding() key.Binding {
	if len(k.Keys) == 0 {
		return key.Binding{}
	}
	return key.NewBinding(
		key.WithKeys(k.Keys...),
		key.WithHelp(k.Keys[0], k.Help),
	)
}

type AppKeyMap struct {
	Quit       key.Binding
	FilterMode key.Binding
	FilterExit key.Binding
}

type KeyMapConfig struct {
	Quit            KeyBindingConfig `json:"quit"`
	FilterMode      KeyBindingConfig `json:"filter_mode"`
	FilterExit      KeyBindingConfig `json:"filter_exit"`
	GoToTop         KeyBindingConfig `json:"go_to_top"`
	GoToLast        KeyBindingConfig `json:"go_to_last"`
	Down            KeyBindingConfig `json:"down"`
	Up              KeyBindingConfig `json:"up"`
	Back            KeyBindingConfig `json:"back"`
	Open            KeyBindingConfig `json:"open"`
	ToggleHidden    KeyBindingConfig `json:"toggle_hidden"`
	ToggleSortField KeyBindingConfig `json:"toggle_sort_field"`
	ToggleSortOrder KeyBindingConfig `json:"toggle_sort_order"`
	Less            KeyBindingConfig `json:"less"`
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func (c ColorConfig) FilePickerStyles() Styles {
	return Styles{
		DisabledCursor:   lipgloss.NewStyle().Foreground(lipgloss.Color(c.DisabledCursor)),
		Cursor:           lipgloss.NewStyle().Foreground(lipgloss.Color(c.Cursor)),
		Symlink:          lipgloss.NewStyle().Foreground(lipgloss.Color(c.Symlink)),
		LinkDest:         lipgloss.NewStyle().Foreground(lipgloss.Color(c.LinkDest)),
		BrokenSymlink:    lipgloss.NewStyle().Foreground(lipgloss.Color(c.BrokenSymlink)),
		FileNotExist:     lipgloss.NewStyle().Foreground(lipgloss.Color(c.FileNotExist)),
		Pipe:             lipgloss.NewStyle().Foreground(lipgloss.Color(c.Pipe)),
		Socket:           lipgloss.NewStyle().Foreground(lipgloss.Color(c.Socket)),
		IrregularFile:    lipgloss.NewStyle().Foreground(lipgloss.Color(c.IrregularFile)),
		Device:           lipgloss.NewStyle().Foreground(lipgloss.Color(c.Device)),
		Directory:        lipgloss.NewStyle().Foreground(lipgloss.Color(c.Directory)),
		File:             lipgloss.NewStyle().Foreground(lipgloss.Color(c.File)),
		Ownership:        lipgloss.NewStyle().Foreground(lipgloss.Color(c.Ownership)),
		DisabledFile:     lipgloss.NewStyle().Foreground(lipgloss.Color(c.DisabledFile)),
		Permission:       lipgloss.NewStyle().Foreground(lipgloss.Color(c.Permission)),
		Selected:         lipgloss.NewStyle().Foreground(lipgloss.Color(c.Selected)),
		DisabledSelected: lipgloss.NewStyle().Foreground(lipgloss.Color(c.DisabledSelected)),
		FileSize:         lipgloss.NewStyle().Foreground(lipgloss.Color(c.FileSize)),
		EmptyDirectory:   lipgloss.NewStyle().Foreground(lipgloss.Color(c.EmptyDirectory)).SetString("No Files Found.\n").MarginLeft(marginLeft),
		Nlink:            lipgloss.NewStyle().Foreground(lipgloss.Color(c.Nlink)),
		Mtime:            lipgloss.NewStyle().Foreground(lipgloss.Color(c.Mtime)),
	}
}

func (c ColorConfig) HeaderStyles() HeaderStyles {
	return HeaderStyles{
		CurrentDir: lipgloss.NewStyle(),
		Selected:   lipgloss.NewStyle().Foreground(lipgloss.Color(c.HeaderSelected)),
		FilesCount: lipgloss.NewStyle().Foreground(lipgloss.Color(c.HeaderFilesCount)),
		SortBy:     lipgloss.NewStyle().Foreground(lipgloss.Color(c.HeaderSortBy)),
	}
}
func (c ColorConfig) StatusBarStyles() StatusBarStyles {
	return StatusBarStyles{
		ErrorMsg: lipgloss.NewStyle().Foreground(lipgloss.Color(c.ErrorMsg)),
	}
}

func (k KeyMapConfig) AppKeyMap() AppKeyMap {
	return AppKeyMap{
		Quit:       k.Quit.ToBinding(),
		FilterMode: k.FilterMode.ToBinding(),
		FilterExit: k.FilterExit.ToBinding(),
	}
}

func (k KeyMapConfig) FilePickerKeyMap() KeyMap {
	return KeyMap{
		GoToTop:         k.GoToTop.ToBinding(),
		GoToLast:        k.GoToLast.ToBinding(),
		Down:            k.Down.ToBinding(),
		Up:              k.Up.ToBinding(),
		Back:            k.Back.ToBinding(),
		Open:            k.Open.ToBinding(),
		ToggleHidden:    k.ToggleHidden.ToBinding(),
		ToggleSortField: k.ToggleSortField.ToBinding(),
		ToggleSortOrder: k.ToggleSortOrder.ToBinding(),
		Less:            k.Less.ToBinding(),
	}
}
