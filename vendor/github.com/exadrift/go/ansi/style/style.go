package style

import (
	"fmt"
	"regexp"
	"strings"
)

var finder = regexp.MustCompile("\n")

var ansiCodeRemover = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-A]`)

type Color uint32

const (
	Black Color = iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
	BlackHi
	RedHi
	GreenHi
	YellowHi
	BlueHi
	MagentaHi
	CyanHi
	WhiteHi
)

var color256Palette = build256ColorPalette()

type StyleType int

const (
	StyleTypeFgColor StyleType = iota
	StyleTypeBgColor
	StyleTypeLineBreak
	StyleTypeReset
)

type Style struct {
	Ansi      string
	StyleType StyleType
}

type Styles []Style

// Ansi writes the ANSI codes for an array of styles
func (s Styles) Ansi() string {
	b := strings.Builder{}
	for _, st := range s {
		b.WriteString(st.Ansi)
	}
	return b.String()
}

type Text struct {
	text   []any
	length int
}

var (
	Break = Style{"", StyleTypeLineBreak}
)

var (
	StyleReset = Style{"\x1b[0m", StyleTypeReset}
)

func toRgb(color Color) (uint32, uint32, uint32) {
	c := uint32(color)
	return c & 0xFF000000 >> 24, c & 0x00FF0000 >> 16, c & 0x0000FF00 >> 8
}

func fgColor(color Color) uint32 {
	var intensity Color
	if color < 8 {
		intensity = 30
	} else {
		intensity = 90
	}

	return uint32(intensity + color)
}

func bgColor(color Color) uint32 {
	var intensity Color
	if color < 8 {
		intensity = 40
	} else {
		intensity = 100
	}

	return uint32(intensity + color)
}

// Fg return an ANSI foreground style representation of the color
func (c Color) Fg() Style {
	// palette color
	if c < 256 {
		return Style{fmt.Sprintf("\x1b[%dm", fgColor(c)), StyleTypeFgColor}
	}

	red, green, blue := toRgb(c)
	return Style{fmt.Sprintf("\x1b[38;2;%d;%d;%dm", red, green, blue), StyleTypeBgColor}
}

// Bg return an ANSI background style representation of the color
func (c Color) Bg() Style {
	// palette color
	if c < 256 {
		return Style{fmt.Sprintf("\x1b[%dm", bgColor(c)), StyleTypeBgColor}
	}

	red, green, blue := toRgb(c)
	return Style{fmt.Sprintf("\x1b[48;2;%d;%d;%dm", red, green, blue), StyleTypeBgColor}
}

// Construct a color from an RGB tri
func FromRgb(red uint32, green uint32, blue uint32) Color {
	return Color(red<<24 + green<<16 + blue<<8)
}

// Generates a styled Text object from a collection of strings and/or style cues
func T(text ...any) *Text {
	length := 0
	var cat []any
	for _, t := range text {
		switch ty := t.(type) {
		case string:
			prevStart := 0
			indexList := finder.FindAllStringIndex(ty, -1)
			for _, indexes := range indexList {
				start := indexes[0]
				end := indexes[1]

				if start > prevStart {
					fragment := []rune(ty[prevStart:start])
					length += len(fragment)
					cat = append(cat, fragment)
				}
				cat = append(cat, Break)
				prevStart = end
			}
			if prevStart < len(ty) {
				fragment := []rune(ty[prevStart:])
				length += len(fragment)
				cat = append(cat, fragment)
			}
		case Style:
			cat = append(cat, ty)
		case Styles:
			for _, sty := range ty {
				cat = append(cat, sty)
			}
		case *Text:
			cat = append(cat, ty.text...)
			length += ty.length
		case []rune:
			cat = append(cat, ty)
			length += len(ty)
		default:
			panic("unknown type in text")
		}
	}

	return &Text{
		text:   cat,
		length: length,
	}
}

type RenderOption struct {
	Width         int
	MinRows       int
	DefaultStyles []Style
}

type RenderOptionFunc func(*RenderOption)

func WithWidthConstraint(width int) RenderOptionFunc {
	return func(opt *RenderOption) {
		opt.Width = width
	}
}

func WithMinRows(minRows int) RenderOptionFunc {
	return func(opt *RenderOption) {
		opt.MinRows = minRows
	}
}

func WithDefaultStyles(styles ...Style) RenderOptionFunc {
	return func(opt *RenderOption) {
		opt.DefaultStyles = styles
	}
}

func (t *Text) Len() int {
	return t.length
}

func (t *Text) Extend(add ...any) *Text {
	newText := &Text{
		text:   t.text[:],
		length: t.length,
	}
	for _, item := range add {
		switch ty := item.(type) {
		case string:
			fragment := []rune(ty)
			newText.text = append(newText.text, fragment)
			newText.length += len(fragment)
		case []rune:
			newText.text = append(newText.text, ty)
			newText.length += len(ty)
		case Style:
			newText.text = append(newText.text, ty)
		case *Text:
			newText.text = append(newText.text, ty.text...)
			newText.length += ty.length
		default:
			panic("unknown item being added")
		}
	}

	return newText
}

func (t *Text) RequiresScroll(width int, height int) bool {
	totalRows := 0
	leftOver := 0
	for _, token := range t.text {
		switch ty := token.(type) {
		case []rune:
			l := len(ty)
			rows := l / width
			leftOver += l % width
			if leftOver > width {
				rows += leftOver / width
				leftOver = leftOver % width
			}

			totalRows += rows
		case Style:
			if ty.StyleType == StyleTypeLineBreak {
				leftOver = 0
				totalRows++
			}
		}
		if totalRows > height {
			return true
		}
	}
	if leftOver > 0 {
		totalRows++
	}

	return totalRows > height
}

// Wrap takes an existing text object and wraps it into an array of text objects, taking
// into account and removing newlines in the process.  Empty lines will result in empty
// Text objects in the array (not null, just empty)
func (t *Text) Wrap(width int) []*Text {
	var rows []*Text
	var curRow []any
	curRowLen := 0

	for _, token := range t.text {
		switch tType := token.(type) {
		case Style:
			switch tType.StyleType {
			case StyleTypeLineBreak:
				// if the current row is empty, then we're ok to interpolate nil and just add an empty Text object
				rows = append(rows, T(curRow...))
				curRow = nil
				curRowLen = 0
			default:
				curRow = append(curRow, tType)
			}
		case []rune:
			for len(tType) > 0 {
				if curRowLen+len(tType) <= width || width == 0 {
					curRow = append(curRow, string(tType))
					curRowLen += len(tType)
					tType = nil
				} else {
					curRow = append(curRow, string(tType[:width-curRowLen]))
					tType = tType[width-curRowLen:]
					rows = append(rows, T(curRow...))
					curRow = nil
					curRowLen = 0
				}
			}
		}
	}

	if len(curRow) > 0 {
		rows = append(rows, T(curRow...))
	}

	return rows
}

func (t *Text) Render(options ...RenderOptionFunc) []string {
	opt := &RenderOption{}
	for _, ofunc := range options {
		ofunc(opt)
	}
	if opt.MinRows == 0 {
		opt.MinRows = 1
	}

	var rows []string
	var curRow strings.Builder
	curRowLen := 0

	// first apply any style defaults
	for _, su := range opt.DefaultStyles {
		_, _ = curRow.WriteString(su.Ansi)
	}

	for _, token := range t.text {
		switch tType := token.(type) {
		case Style:
			switch tType.StyleType {
			case StyleTypeLineBreak:
				// this if block isn't executed when width is zero, which means no padding happens, this is correct
				if curRowLen < opt.Width {
					curRow.WriteString(strings.Repeat(" ", opt.Width-curRowLen))
				}
				rows = append(rows, curRow.String())
				curRow.Reset()
				curRowLen = 0
			default:
				curRow.WriteString(tType.Ansi)
				if tType.StyleType == StyleTypeReset {
					// apply the style defaults again
					for _, su := range opt.DefaultStyles {
						_, _ = curRow.WriteString(su.Ansi)
					}
				}
			}
		case []rune:
			for len(tType) > 0 {
				if curRowLen+len(tType) <= opt.Width || opt.Width == 0 {
					curRow.WriteString(string(tType))
					curRowLen += len(tType)
					tType = nil
				} else {
					curRow.WriteString(string(tType[:opt.Width-curRowLen]))
					tType = tType[opt.Width-curRowLen:]
					rows = append(rows, curRow.String())
					curRow.Reset()
					curRowLen = 0
				}
			}
		}
	}

	if curRowLen > 0 && curRowLen < opt.Width {
		curRow.WriteString(strings.Repeat(" ", opt.Width-curRowLen))
	}
	if curRow.Len() > 0 {
		rows = append(rows, curRow.String())
		curRow.Reset()
	}

	// if there are empty rows as far as the minimum height is concerned, this will fill them
	for i := len(rows); i < opt.MinRows; i++ {
		if opt.Width > 0 {
			rows = append(rows, strings.Repeat(" ", opt.Width))
		} else {
			rows = append(rows, "")
		}
	}

	return rows
}

// StripAnsi will remove any ANSI sequences from the provided text
func StripAnsi(text string) string {
	return ansiCodeRemover.ReplaceAllString(text, "")
}

func build256ColorPalette() []Color {
	palette := make([]Color, 256)

	palette[0] = FromRgb(0, 0, 0)
	palette[1] = FromRgb(128, 0, 0)
	palette[2] = FromRgb(0, 128, 0)
	palette[3] = FromRgb(128, 128, 0)
	palette[4] = FromRgb(0, 0, 128)
	palette[5] = FromRgb(128, 0, 128)
	palette[6] = FromRgb(0, 128, 128)
	palette[7] = FromRgb(192, 192, 192)
	palette[8] = FromRgb(128, 128, 128)
	palette[9] = FromRgb(255, 0, 0)
	palette[10] = FromRgb(0, 255, 0)
	palette[11] = FromRgb(255, 255, 0)
	palette[12] = FromRgb(0, 0, 255)
	palette[13] = FromRgb(255, 0, 255)
	palette[14] = FromRgb(0, 255, 255)
	palette[15] = FromRgb(255, 255, 255)

	// color section
	intensity := [6]uint32{0, 95, 135, 175, 215, 255}
	for r := range 6 {
		for g := range 6 {
			for b := range 6 {
				index := 16 + (36*r + 6*g + b)
				palette[index] = FromRgb(intensity[r], intensity[g], intensity[b])
			}
		}
	}

	// grayscale section
	for i := range 24 {
		color := uint32(10*i + 8)
		palette[232+i] = FromRgb(color, color, color)
	}

	return palette
}

// Returns a color from the standard palette
func GetPaletteColor(color uint8) Color {
	return color256Palette[color]
}
