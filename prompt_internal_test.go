package prompt

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestApplyEscapeSequenceHomeAndEnd(t *testing.T) {
	is := is.New(t)
	line := []rune("hello world")

	_, cursor := applyEscapeSequence("[H", line, 5)
	is.Equal(cursor, 0)

	_, cursor = applyEscapeSequence("OF", line, 2)
	is.Equal(cursor, len(line))
}

func TestApplyEscapeSequenceArrows(t *testing.T) {
	is := is.New(t)
	line := []rune("hello")

	_, cursor := applyEscapeSequence("[C", line, 1)
	is.Equal(cursor, 2)

	_, cursor = applyEscapeSequence("[D", line, cursor)
	is.Equal(cursor, 1)
}

func TestApplyEscapeSequenceDelete(t *testing.T) {
	is := is.New(t)
	line := []rune("abc")

	line, cursor := applyEscapeSequence("[3~", line, 1)
	is.Equal(string(line), "ac")
	is.Equal(cursor, 1)
}

func TestApplyEscapeSequenceWordMotion(t *testing.T) {
	is := is.New(t)
	line := []rune("hello brave world")

	_, cursor := applyEscapeSequence("[1;5D", line, len(line))
	is.Equal(cursor, 12)

	_, cursor = applyEscapeSequence("b", line, cursor)
	is.Equal(cursor, 6)

	_, cursor = applyEscapeSequence("f", line, cursor)
	is.Equal(cursor, 11)

	_, cursor = applyEscapeSequence("[1;5C", line, cursor)
	is.Equal(cursor, len(line))
}

func TestApplyEscapeSequenceBackwardKillWord(t *testing.T) {
	is := is.New(t)
	line := []rune("hello brave world")

	line, cursor := applyEscapeSequence("\x7f", line, len(line))
	is.Equal(string(line), "hello brave ")
	is.Equal(cursor, 12)

	line, cursor = applyEscapeSequence("\x08", line, len(line))
	is.Equal(string(line), "hello ")
	is.Equal(cursor, 6)

	line = []rune("hello brave world")
	line, cursor = applyEscapeSequence("[127;3u", line, len(line))
	is.Equal(string(line), "hello brave ")
	is.Equal(cursor, 12)

	line = []rune("hello brave world")
	line, cursor = applyEscapeSequence("[8;3u", line, len(line))
	is.Equal(string(line), "hello brave ")
	is.Equal(cursor, 12)

	line = []rune("hello brave world")
	line, cursor = applyEscapeSequence("[3;3~", line, len(line))
	is.Equal(string(line), "hello brave ")
	is.Equal(cursor, 12)
}

func TestBackwardKillLine(t *testing.T) {
	is := is.New(t)
	line := []rune("hello brave world")

	line, cursor := backwardKillLine(line, 6)
	is.Equal(string(line), "brave world")
	is.Equal(cursor, 0)
}

func TestReadEscapeSequenceAltBackspace(t *testing.T) {
	is := is.New(t)
	r := bufio.NewReader(strings.NewReader("\x7fX"))

	seq, err := readEscapeSequence(r)
	is.NoErr(err)
	is.Equal(seq, "\x7f")

	b, err := r.ReadByte()
	is.NoErr(err)
	is.Equal(string([]byte{b}), "X")
}

func TestReadEscapeSequenceCSIU(t *testing.T) {
	is := is.New(t)
	r := bufio.NewReader(strings.NewReader("[127;3uX"))

	seq, err := readEscapeSequence(r)
	is.NoErr(err)
	is.Equal(seq, "[127;3u")

	b, err := r.ReadByte()
	is.NoErr(err)
	is.Equal(string([]byte{b}), "X")
}

func TestBackwardKillWordCtrlW(t *testing.T) {
	is := is.New(t)
	line := []rune("hello brave world")

	line, cursor := backwardKillWord(line, len(line))
	is.Equal(string(line), "hello brave ")
	is.Equal(cursor, 12)
}

func TestHandleInterrupt(t *testing.T) {
	is := is.New(t)
	writer := new(bytes.Buffer)

	err := handleInterrupt(writer)
	is.True(errors.Is(err, ErrInterrupted))
	is.Equal(writer.String(), "^C\r\n")
}
