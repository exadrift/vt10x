package vt10x

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/alecthomas/assert"
)

func extractStr(term Terminal, x0, x1, row int) string {
	var s []rune
	for i := x0; i <= x1; i++ {
		attr := term.Cell(i, row)
		s = append(s, attr.Char)
	}
	return string(s)
}

func TestPlainChars(t *testing.T) {
	term := New()
	expected := "Hello world!"
	_, err := term.Write([]byte(expected))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	actual := extractStr(term, 0, len(expected)-1, 0)
	if expected != actual {
		t.Fatal(actual)
	}
}

func TestNewline(t *testing.T) {
	term := New()
	expected := "Hello world!\n...and more."
	_, err := term.Write([]byte("\033[20h")) // set CRLF mode
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	_, err = term.Write([]byte(expected))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}

	split := strings.Split(expected, "\n")
	actual := extractStr(term, 0, len(split[0])-1, 0)
	actual += "\n"
	actual += extractStr(term, 0, len(split[1])-1, 1)
	if expected != actual {
		t.Fatal(actual)
	}

	// A newline with a color set should not make the next line that color,
	// which used to happen if it caused a scroll event.
	st := (term.(*terminal))
	st.moveTo(0, st.rows-1)
	_, err = term.Write([]byte("\033[1;37m\n$ \033[m"))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	cur := term.Cursor()
	attr := term.Cell(cur.X, cur.Y)
	if attr.FG != DefaultFG {
		t.Fatal(st.cur.X, st.cur.Y, attr.FG, attr.BG)
	}
}

func TestHistoryBuffer(t *testing.T) {
	term := New(WithSize(10, 5))
	_, err := term.Write([]byte("\033[20h")) // set CRLF mode
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	for i := -15; i < 0; i++ {
		line := fmt.Sprintf("line %d\n", i)
		_, err := term.Write([]byte(line))
		assert.NoError(t, err)
	}
	_, err = term.Write([]byte("line 0"))
	assert.NoError(t, err)

	hbLen := term.HistoryBufferLength()
	//the blank line that gets created at the end when the scroll happens accounts for the extra line
	assert.Equal(t, 16, hbLen)

	rows := term.AnsiRows()
	assert.Len(t, rows, 5)

	historyRows := term.History(0)
	assert.Len(t, historyRows, 5)

	// The on screen rows should always be the same, no matter which call is made
	for i := range rows {
		assert.Equal(t, rows[i], historyRows[i])
	}

	offset := -3
	historyRows = term.History(offset)
	curRow := offset - (len(historyRows) - 1)
	for i, row := range historyRows {
		line := fmt.Sprintf("line %d", curRow+i)
		assert.True(t, strings.HasPrefix(row, line))
	}
}
