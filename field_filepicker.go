package huh

import (
	"cmp"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh/internal/accessibility"
	"github.com/charmbracelet/lipgloss"
	xstrings "github.com/charmbracelet/x/exp/strings"
)

// FilePicker is a form file file field.
type FilePicker struct {
	accessor Accessor[string]
	key      string
	picker   filepicker.Model

	// state
	focused bool
	picking bool

	// customization
	title       string
	description string

	// error handling
	validate func(string) error
	err      error

	// options
	width      int
	height     int
	accessible bool // Deprecated: use RunAccessible instead.
	theme      *Theme
	keymap     FilePickerKeyMap
}

// NewFilePicker returns a new file field.
func NewFilePicker() *FilePicker {
	fp := filepicker.New()
	fp.ShowSize = false

	if cmd := fp.Init(); cmd != nil {
		fp, _ = fp.Update(cmd())
	}

	return &FilePicker{
		accessor: &EmbeddedAccessor[string]{},
		validate: func(string) error { return nil },
		picker:   fp,
	}
}

// CurrentDirectory sets the directory of the file field.
func (f *FilePicker) CurrentDirectory(directory string) *FilePicker {
	f.picker.CurrentDirectory = directory
	if cmd := f.picker.Init(); cmd != nil {
		f.picker, _ = f.picker.Update(cmd())
	}
	return f
}

// Cursor sets the cursor of the file field.
func (f *FilePicker) Cursor(cursor string) *FilePicker {
	f.picker.Cursor = cursor
	return f
}

// Picking sets whether the file picker should be in the picking files state.
func (f *FilePicker) Picking(v bool) *FilePicker {
	f.setPicking(v)
	return f
}

// ShowHidden sets whether to show hidden files.
func (f *FilePicker) ShowHidden(v bool) *FilePicker {
	f.picker.ShowHidden = v
	return f
}

// ShowSize sets whether to show file sizes.
func (f *FilePicker) ShowSize(v bool) *FilePicker {
	f.picker.ShowSize = v
	return f
}

// ShowPermissions sets whether to show file permissions.
func (f *FilePicker) ShowPermissions(v bool) *FilePicker {
	f.picker.ShowPermissions = v
	return f
}

// FileAllowed sets whether to allow files to be selected.
func (f *FilePicker) FileAllowed(v bool) *FilePicker {
	f.picker.FileAllowed = v
	return f
}

// DirAllowed sets whether to allow files to be selected.
func (f *FilePicker) DirAllowed(v bool) *FilePicker {
	f.picker.DirAllowed = v
	return f
}

// Value sets the value of the file field.
func (f *FilePicker) Value(value *string) *FilePicker {
	return f.Accessor(NewPointerAccessor(value))
}

// Accessor sets the accessor of the file field.
func (f *FilePicker) Accessor(accessor Accessor[string]) *FilePicker {
	f.accessor = accessor
	return f
}

// Key sets the key of the file field which can be used to retrieve the value
// after submission.
func (f *FilePicker) Key(key string) *FilePicker {
	f.key = key
	return f
}

// Title sets the title of the file field.
func (f *FilePicker) Title(title string) *FilePicker {
	f.title = title
	return f
}

// Description sets the description of the file field.
func (f *FilePicker) Description(description string) *FilePicker {
	f.description = description
	return f
}

// AllowedTypes sets the allowed types of the file field. These will be the only
// valid file types accepted, other files will show as disabled.
func (f *FilePicker) AllowedTypes(types []string) *FilePicker {
	f.picker.AllowedTypes = types
	return f
}

// Height sets the height of the file field. If the number of options
// exceeds the height, the file field will become scrollable.
func (f *FilePicker) Height(height int) *FilePicker {
	f.WithHeight(height)
	return f
}

// Validate sets the validation function of the file field.
func (f *FilePicker) Validate(validate func(string) error) *FilePicker {
	f.validate = validate
	return f
}

// Error returns the error of the file field.
func (f *FilePicker) Error() error {
	return f.err
}

// Skip returns whether the file should be skipped or should be blocking.
func (*FilePicker) Skip() bool {
	return false
}

// Zoom returns whether the input should be zoomed.
func (f *FilePicker) Zoom() bool {
	return f.picking
}

// Focus focuses the file field.
func (f *FilePicker) Focus() tea.Cmd {
	f.focused = true
	return f.picker.Init()
}

// Blur blurs the file field.
func (f *FilePicker) Blur() tea.Cmd {
	f.focused = false
	f.setPicking(false)
	f.err = f.validate(f.accessor.Get())
	return nil
}

// KeyBinds returns the help keybindings for the file field.
func (f *FilePicker) KeyBinds() []key.Binding {
	return []key.Binding{f.keymap.Up, f.keymap.Down, f.keymap.Close, f.keymap.Open, f.keymap.Prev, f.keymap.Next, f.keymap.Submit}
}

// Init initializes the file field.
func (f *FilePicker) Init() tea.Cmd {
	return f.picker.Init()
}

// Update updates the file field.
func (f *FilePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	f.err = nil

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, f.keymap.Open):
			if f.picking {
				break
			}
			f.setPicking(true)
			return f, f.picker.Init()
		case key.Matches(msg, f.keymap.Close):
			f.setPicking(false)
			return f, NextField
		case key.Matches(msg, f.keymap.Next):
			f.setPicking(false)
			return f, NextField
		case key.Matches(msg, f.keymap.Prev):
			f.setPicking(false)
			return f, PrevField
		}
	}

	var cmd tea.Cmd
	f.picker, cmd = f.picker.Update(msg)
	didSelect, file := f.picker.DidSelectFile(msg)
	if didSelect {
		f.accessor.Set(file)
		f.setPicking(false)
		return f, NextField
	}
	didSelect, _ = f.picker.DidSelectDisabledFile(msg)
	if didSelect {
		f.err = errors.New(xstrings.EnglishJoin(f.picker.AllowedTypes, true) + " files only")
		return f, nil
	}

	return f, cmd
}

func (f *FilePicker) activeStyles() *FieldStyles {
	theme := f.theme
	if theme == nil {
		theme = ThemeCharm()
	}
	if f.focused {
		return &theme.Focused
	}
	return &theme.Blurred
}

func (f *FilePicker) renderTitle() string {
	styles := f.activeStyles()
	maxWidth := f.width - styles.Base.GetHorizontalFrameSize()
	return styles.Title.Render(wrap(f.title, maxWidth))
}

func (f FilePicker) renderDescription() string {
	styles := f.activeStyles()
	maxWidth := f.width - styles.Base.GetHorizontalFrameSize()
	return styles.Description.Render(wrap(f.description, maxWidth))
}

// View renders the file field.
func (f *FilePicker) View() string {
	styles := f.activeStyles()
	var parts []string
	if f.title != "" {
		parts = append(parts, f.renderTitle())
	}
	if f.description != "" {
		parts = append(parts, f.renderDescription())
	}
	parts = append(parts, f.pickerView())
	return styles.Base.Width(f.width).Height(f.height).
		Render(strings.Join(parts, "\n"))
}

func (f *FilePicker) pickerView() string {
	if f.picking {
		return f.picker.View()
	}
	styles := f.activeStyles()
	if f.accessor.Get() != "" {
		return styles.SelectedOption.Render(f.accessor.Get())
	}
	return styles.TextInput.Placeholder.Render("No file selected.")
}

func (f *FilePicker) setPicking(v bool) {
	f.picking = v

	f.keymap.Close.SetEnabled(v)
	f.keymap.Up.SetEnabled(v)
	f.keymap.Down.SetEnabled(v)
	f.keymap.Select.SetEnabled(v)
	f.keymap.Back.SetEnabled(v)

	f.picker.KeyMap.Up.SetEnabled(v)
	f.picker.KeyMap.Down.SetEnabled(v)
	f.picker.KeyMap.GoToTop.SetEnabled(v)
	f.picker.KeyMap.GoToLast.SetEnabled(v)
	f.picker.KeyMap.Select.SetEnabled(v)
	f.picker.KeyMap.Open.SetEnabled(v)
	f.picker.KeyMap.Back.SetEnabled(v)
}

// Run runs the file field.
func (f *FilePicker) Run() error {
	if f.accessible { // TODO: remove in a future release.
		return f.RunAccessible(os.Stdout, os.Stdin)
	}
	return Run(f)
}

// RunAccessible runs an accessible file field.
func (f *FilePicker) RunAccessible(w io.Writer, r io.Reader) error {
	styles := f.activeStyles()
	prompt := styles.Title.
		PaddingRight(1).
		Render(cmp.Or(f.title, "Choose a file:"))

	validateFile := func(s string) error {
		// is the string a file?
		if _, err := os.Open(s); err != nil {
			return errors.New("not a file")
		}

		// is it one of the allowed types?
		valid := len(f.picker.AllowedTypes) == 0
		for _, ext := range f.picker.AllowedTypes {
			if strings.HasSuffix(s, ext) {
				valid = true
				break
			}
		}
		if !valid {
			return errors.New("cannot select: " + s)
		}

		// does it pass user validation?
		return f.validate(s)
	}

	f.accessor.Set(accessibility.PromptString(
		w,
		r,
		prompt,
		f.GetValue().(string),
		validateFile,
	))
	return nil
}

// copied from bubbles' filepicker.
const (
	fileSizeWidth = 7
	paddingLeft   = 2
)

// WithTheme sets the theme of the file field.
func (f *FilePicker) WithTheme(theme *Theme) Field {
	if f.theme != nil || theme == nil {
		return f
	}
	f.theme = theme

	// XXX: add specific themes
	f.picker.Styles = filepicker.Styles{
		DisabledCursor:   lipgloss.Style{},
		Cursor:           theme.Focused.TextInput.Prompt,
		Symlink:          lipgloss.NewStyle(),
		Directory:        theme.Focused.Directory,
		File:             theme.Focused.File,
		DisabledFile:     theme.Focused.TextInput.Placeholder,
		Permission:       theme.Focused.TextInput.Placeholder,
		Selected:         theme.Focused.SelectedOption,
		DisabledSelected: theme.Focused.TextInput.Placeholder,
		FileSize:         theme.Focused.TextInput.Placeholder.Width(fileSizeWidth).Align(lipgloss.Right),
		EmptyDirectory:   theme.Focused.TextInput.Placeholder.PaddingLeft(paddingLeft).SetString("No files found."),
	}

	return f
}

// WithKeyMap sets the keymap on a file field.
func (f *FilePicker) WithKeyMap(k *KeyMap) Field {
	f.keymap = k.FilePicker
	f.picker.KeyMap = filepicker.KeyMap{
		GoToTop:  k.FilePicker.GotoTop,
		GoToLast: k.FilePicker.GotoBottom,
		Down:     k.FilePicker.Down,
		Up:       k.FilePicker.Up,
		PageUp:   k.FilePicker.PageUp,
		PageDown: k.FilePicker.PageDown,
		Back:     k.FilePicker.Back,
		Open:     k.FilePicker.Open,
		Select:   k.FilePicker.Select,
	}
	f.setPicking(f.picking)
	return f
}

// WithAccessible sets the accessible mode of the file field.
//
// Deprecated: you may now call [FilePicker.RunAccessible] directly to run the
// field in accessible mode.
func (f *FilePicker) WithAccessible(accessible bool) Field {
	f.accessible = accessible
	return f
}

// WithWidth sets the width of the file field.
func (f *FilePicker) WithWidth(width int) Field {
	f.width = width
	return f
}

// WithHeight sets the height of the file field.
func (f *FilePicker) WithHeight(height int) Field {
	if height == 0 {
		return f
	}
	adjust := 0
	if f.title != "" {
		adjust += lipgloss.Height(f.renderTitle())
	}
	if f.description != "" {
		adjust += lipgloss.Height(f.renderDescription())
	}
	adjust++ // picker's own help height
	f.picker.SetHeight(height - adjust)
	return f
}

// WithPosition sets the position of the file field.
func (f *FilePicker) WithPosition(p FieldPosition) Field {
	f.keymap.Prev.SetEnabled(!p.IsFirst())
	f.keymap.Next.SetEnabled(!p.IsLast())
	f.keymap.Submit.SetEnabled(p.IsLast())
	return f
}

// GetKey returns the key of the field.
func (f *FilePicker) GetKey() string {
	return f.key
}

// GetValue returns the value of the field.
func (f *FilePicker) GetValue() any {
	return f.accessor.Get()
}
