package prompt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"
)

// ErrRequired is returned when a required input is empty
var ErrRequired = fmt.Errorf("prompt: input is required")

// ErrInterrupted is returned when a terminal prompt is interrupted (Ctrl+C).
var ErrInterrupted = fmt.Errorf("prompt: interrupted")

type fd interface {
	Fd() uintptr
}

func getFd(r io.Reader) int {
	if f, ok := r.(fd); ok {
		return int(f.Fd())
	}
	return -1
}

// Option configures an input question.
type Option func(*prompt)

type fn func(string) error

// WithDefault sets a default value for a question.
func WithDefault(defaultTo string) Option {
	return func(q *prompt) {
		q.defaultTo = defaultTo
	}
}

// WithOptional marks a question as optional.
func WithOptional(optional bool) Option {
	return func(q *prompt) {
		q.optional = optional
	}
}

// WithCheck appends checks for a question.
func WithCheck(checks ...fn) Option {
	return func(q *prompt) {
		q.checks = append(q.checks, checks...)
	}
}

// WithWriter overrides the writer for a single question.
func WithWriter(w io.Writer) Option {
	return func(q *prompt) {
		if w == nil {
			return
		}
		q.writer = w
	}
}

// WithReader overrides the reader for a single question.
func WithReader(r io.Reader) Option {
	if r == nil {
		return func(*prompt) {}
	}
	fd := getFd(r)
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return func(q *prompt) {
		q.reader = br
		q.fd = fd
	}
}

// Ask asks a question and returns the input.
func Ask(ctx context.Context, prompt string, options ...Option) (string, error) {
	q := newPrompt(options...)
	return q.Ask(ctx, prompt)
}

// Password asks for a password and returns the input.
func Password(ctx context.Context, prompt string, options ...Option) (string, error) {
	q := newPrompt(options...)
	return q.Password(ctx, prompt)
}

// Confirm asks for a confirmation and returns the input.
func Confirm(ctx context.Context, prompt string, options ...Option) (bool, error) {
	q := newPrompt(options...)
	return q.Confirm(ctx, prompt)
}

// prompt is a single prompt invocation.
type prompt struct {
	writer    io.Writer
	reader    *bufio.Reader
	fd        int
	checks    []fn
	defaultTo string
	optional  bool
}

func newPrompt(options ...Option) *prompt {
	q := &prompt{
		writer: os.Stdout,
		reader: bufio.NewReader(os.Stdin),
		fd:     getFd(os.Stdin),
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		option(q)
	}
	return q
}

func (q *prompt) isTerminal() bool {
	return q.fd > -1 && term.IsTerminal(q.fd)
}

func (q *prompt) scanLine(inputCh chan<- string, errorCh chan<- error) {
	// Read the input.
	input, err := q.reader.ReadString('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) {
			errorCh <- err
			return
		}
		// If we're at the end of the input, and there is a default, use it,
		// otherwise return a required error.
		if q.defaultTo != "" {
			inputCh <- q.defaultTo
			return
		} else if !q.optional {
			errorCh <- ErrRequired
			return
		}
	}

	// Trim the input.
	input = strings.TrimRight(input, "\r\n")
	inputCh <- input
}

func (q *prompt) readTerminalLine(inputOffset int) (string, error) {
	state, err := term.MakeRaw(q.fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(q.fd, state)

	line := []rune{}
	cursor := 0

	for {
		b, err := q.reader.ReadByte()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return "", err
			}
			return q.eofValue(string(line))
		}

		oldCursor := cursor
		oldLen := len(line)
		switch b {
		case '\r', '\n':
			fmt.Fprint(q.writer, "\r\n")
			return string(line), nil
		case 0x03: // Ctrl+C
			return "", handleInterrupt(q.writer)
		case 0x01: // Ctrl+A
			cursor = 0
		case 0x02: // Ctrl+B
			if cursor > 0 {
				cursor--
			}
		case 0x05: // Ctrl+E
			cursor = len(line)
		case 0x06: // Ctrl+F
			if cursor < len(line) {
				cursor++
			}
		case 0x0b: // Ctrl+K
			line = line[:cursor]
		case 0x15: // Ctrl+U
			line, cursor = backwardKillLine(line, cursor)
		case 0x17: // Ctrl+W
			line, cursor = backwardKillWord(line, cursor)
		case 0x04: // Ctrl+D
			if len(line) == 0 {
				return q.eofValue("")
			}
			if cursor < len(line) {
				line = append(line[:cursor], line[cursor+1:]...)
			}
		case 0x08, 0x7f: // Backspace
			if cursor > 0 {
				line = append(line[:cursor-1], line[cursor:]...)
				cursor--
			}
		case 0x1b: // Escape sequence
			seq, err := readEscapeSequence(q.reader)
			if err != nil {
				return "", err
			}
			line, cursor = applyEscapeSequence(seq, line, cursor)
		default:
			if err := q.reader.UnreadByte(); err != nil {
				return "", err
			}
			r, _, err := q.reader.ReadRune()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return q.eofValue(string(line))
				}
				return "", err
			}
			if unicode.IsControl(r) {
				continue
			}
			line = append(line[:cursor], append([]rune{r}, line[cursor:]...)...)
			cursor++
		}

		redrawTerminalLine(q.writer, line, oldLen, oldCursor, cursor, inputOffset, getTerminalWidth(q.fd))
	}
}

func handleInterrupt(w io.Writer) error {
	fmt.Fprint(w, "^C\r\n")
	return ErrInterrupted
}

func (q *prompt) eofValue(input string) (string, error) {
	if input != "" {
		return input, nil
	}
	if q.defaultTo != "" {
		return q.defaultTo, nil
	}
	if !q.optional {
		return "", ErrRequired
	}
	return "", nil
}

func getTerminalWidth(fd int) int {
	if fd < 0 {
		return 0
	}
	width, _, err := term.GetSize(fd)
	if err != nil {
		return 0
	}
	return width
}

func redrawTerminalLine(w io.Writer, line []rune, oldLen, oldCursor, cursor, inputOffset, terminalWidth int) {
	if terminalWidth <= 0 {
		redrawTerminalLineLegacy(w, line, oldCursor, cursor)
		return
	}
	inputCol := inputOffset % terminalWidth
	moveVisualCursor(w, inputCol, terminalWidth, oldCursor, 0)
	fmt.Fprint(w, string(line))
	printedLen := len(line)
	if oldLen > len(line) {
		fmt.Fprint(w, strings.Repeat(" ", oldLen-len(line)))
		printedLen = oldLen
	}
	moveRenderedCursorToLogical(w, inputCol, terminalWidth, printedLen, cursor)
}

func redrawTerminalLineLegacy(w io.Writer, line []rune, oldCursor, cursor int) {
	if oldCursor > 0 {
		fmt.Fprintf(w, "\x1b[%dD", oldCursor)
	}
	fmt.Fprint(w, string(line))
	fmt.Fprint(w, "\x1b[K")
	if back := len(line) - cursor; back > 0 {
		fmt.Fprintf(w, "\x1b[%dD", back)
	}
}

func moveVisualCursor(w io.Writer, inputCol, width, fromIndex, toIndex int) {
	if fromIndex == toIndex {
		return
	}
	fromRow, _ := visualPosition(inputCol, fromIndex, width)
	toRow, toCol := visualPosition(inputCol, toIndex, width)
	moveCursor(w, fromRow, toRow, toCol)
}

func moveRenderedCursorToLogical(w io.Writer, inputCol, width, renderedIndex, logicalIndex int) {
	fromRow, _ := renderedPosition(inputCol, renderedIndex, width)
	toRow, toCol := visualPosition(inputCol, logicalIndex, width)
	moveCursor(w, fromRow, toRow, toCol)
}

func moveCursor(w io.Writer, fromRow, toRow, toCol int) {
	switch {
	case fromRow > toRow:
		fmt.Fprintf(w, "\x1b[%dA", fromRow-toRow)
	case fromRow < toRow:
		fmt.Fprintf(w, "\x1b[%dB", toRow-fromRow)
	}
	fmt.Fprint(w, "\r")
	if toCol > 0 {
		fmt.Fprintf(w, "\x1b[%dC", toCol)
	}
}

func visualPosition(inputCol, index, width int) (int, int) {
	absolute := inputCol + index
	return absolute / width, absolute % width
}

func renderedPosition(inputCol, index, width int) (int, int) {
	absolute := inputCol + index
	row := absolute / width
	col := absolute % width
	if index > 0 && col == 0 {
		row--
		col = width - 1
	}
	return row, col
}

func readEscapeSequence(r *bufio.Reader) (string, error) {
	seq := make([]byte, 0, 16)
	for i := 0; i < 16; i++ {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return string(seq), nil
			}
			return "", err
		}
		seq = append(seq, b)
		if isEscapeSequenceTerminator(b) {
			break
		}
	}
	return string(seq), nil
}

func isEscapeSequenceTerminator(b byte) bool {
	if b == '~' || b == 0x7f || unicode.IsControl(rune(b)) {
		return true
	}
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func applyEscapeSequence(seq string, line []rune, cursor int) ([]rune, int) {
	switch seq {
	case "[D", "OD":
		if cursor > 0 {
			cursor--
		}
	case "[C", "OC":
		if cursor < len(line) {
			cursor++
		}
	case "[H", "[1~", "[7~", "OH":
		cursor = 0
	case "[F", "[4~", "[8~", "OF":
		cursor = len(line)
	case "[3~":
		if cursor < len(line) {
			line = append(line[:cursor], line[cursor+1:]...)
		}
	case "b", "B", "[1;5D", "[5D":
		cursor = moveCursorWordLeft(line, cursor)
	case "f", "F", "[1;5C", "[5C":
		cursor = moveCursorWordRight(line, cursor)
	case "\x7f", "\x08", "[3;3~", "[8;3u", "[127;3u":
		line, cursor = backwardKillWord(line, cursor)
	}
	return line, cursor
}

func moveCursorWordLeft(line []rune, cursor int) int {
	for cursor > 0 && unicode.IsSpace(line[cursor-1]) {
		cursor--
	}
	for cursor > 0 && !unicode.IsSpace(line[cursor-1]) {
		cursor--
	}
	return cursor
}

func moveCursorWordRight(line []rune, cursor int) int {
	for cursor < len(line) && unicode.IsSpace(line[cursor]) {
		cursor++
	}
	for cursor < len(line) && !unicode.IsSpace(line[cursor]) {
		cursor++
	}
	return cursor
}

func backwardKillWord(line []rune, cursor int) ([]rune, int) {
	start := cursor
	for start > 0 && unicode.IsSpace(line[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(line[start-1]) {
		start--
	}
	return append(line[:start], line[cursor:]...), start
}

func backwardKillLine(line []rune, cursor int) ([]rune, int) {
	return append(line[:0], line[cursor:]...), 0
}

// Read the password. If the file descriptor is available, use term.ReadPassword
// otherwise read the line from the scanner.
func (q *prompt) scanPassword(inputCh chan<- string, errorCh chan<- error) {
	if q.isTerminal() {
		pass, err := term.ReadPassword(q.fd)
		if err != nil {
			errorCh <- err
			return
		}
		inputCh <- string(pass)
		return
	}

	q.scanLine(inputCh, errorCh)
}

// Reads the input from the reader.
func (q *prompt) readInput(ctx context.Context, inputOffset int) (string, error) {
	// Check if the context has already been cancelled.
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Terminal input is handled synchronously to guarantee raw mode cleanup.
	if q.isTerminal() {
		return q.readTerminalLine(inputOffset)
	}

	inputCh := make(chan string)
	errorCh := make(chan error)

	// Scan for the input in a goroutine, so we can listen for cancellations.
	go q.scanLine(inputCh, errorCh)

	// Wait for input, an error or the context to be cancelled.
	select {
	case input := <-inputCh:
		close(inputCh)
		close(errorCh)
		return input, nil
	case err := <-errorCh:
		close(inputCh)
		close(errorCh)
		return "", err
	case <-ctx.Done():
		// In this case, we're leaking the goroutine that's reading the input.
		// This is because we can't really cancel reads without limitations.
		// This seems acceptable because typically when context is canceled, the
		// process will exit shortly.
		return "", ctx.Err()
	}
}

// Reads the password from the reader.
func (q *prompt) readPassword(ctx context.Context) (string, error) {
	// Check if the context has already been cancelled.
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	inputCh := make(chan string)
	errorCh := make(chan error)

	// Scan for the password in a goroutine, so we can listen for cancelations.
	go q.scanPassword(inputCh, errorCh)

	// Wait for input, an error or the context to be cancelled.
	select {
	case input := <-inputCh:
		close(inputCh)
		close(errorCh)
		return input, nil
	case err := <-errorCh:
		close(inputCh)
		close(errorCh)
		return "", err
	case <-ctx.Done():
		// In this case, we're leaking the goroutine that's reading the password.
		// This is because we can't really cancel reads without limitations.
		// This seems acceptable because typically when context is canceled, the
		// process will exit shortly.
		return "", ctx.Err()
	}
}

// Ask asks a question and returns the input.
func (q *prompt) Ask(ctx context.Context, prompt string) (string, error) {
	// Write out the formatted prompt.
retry:
	promptText := prompt + " "
	fmt.Fprint(q.writer, promptText)

	// Read the input.
	input, err := q.readInput(ctx, utf8.RuneCountInString(promptText))
	if err != nil {
		return "", err
	}

	// If the input is empty, and there is a default, use it otherwise ask again.
	if input == "" {
		if q.defaultTo != "" {
			return q.defaultTo, nil
		} else if !q.optional {
			goto retry
		}
	}

	// If any checks fail, print the error and ask again.
	for _, check := range q.checks {
		if err := check(input); err != nil {
			fmt.Fprintln(q.writer, err)
			goto retry
		}
	}

	return input, nil
}

// Password asks for a password and returns the input.
func (q *prompt) Password(ctx context.Context, prompt string) (string, error) {
	// Write out the formatted prompt.
retry:
	fmt.Fprint(q.writer, prompt, " ")

	// Read the input.
	pass, err := q.readPassword(ctx)
	if err != nil {
		return "", err
	}
	// Print a newline after the password.
	fmt.Fprintln(q.writer)

	if pass == "" {
		if q.defaultTo != "" {
			return q.defaultTo, nil
		} else if !q.optional {
			goto retry
		}
	}

	// If any checks fail, print the error and ask again.
	for _, check := range q.checks {
		if err := check(pass); err != nil {
			fmt.Fprintln(q.writer, err)
			goto retry
		}
	}

	return pass, nil
}

func isYes(s string) bool {
	switch strings.ToLower(s) {
	case "y", "yes", "true":
		return true
	}
	return false
}

// Confirm asks for a confirmation and returns the input.
func (q *prompt) Confirm(ctx context.Context, prompt string) (bool, error) {
	// Add a check to ensure the input is yes or no.
	q.checks = append(q.checks, func(s string) error {
		switch strings.ToLower(s) {
		case "y", "yes":
			return nil
		case "n", "no":
			return nil
		default:
			return fmt.Errorf("invalid value %q, must enter yes or no", s)
		}
	})

	input, err := q.Ask(ctx, prompt)
	if err != nil {
		return false, err
	}

	return isYes(input), nil
}
