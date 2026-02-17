package prompter

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"
)

// ErrRequired is returned when a required input is empty
var ErrRequired = fmt.Errorf("prompter: input is required")

// ErrInterrupted is returned when a terminal prompt is interrupted (Ctrl+C).
var ErrInterrupted = fmt.Errorf("prompter: interrupted")

// Default creates a default prompt using stdin and stdout
func Default() *Prompt {
	return New(os.Stdout, os.Stdin)
}

// New prompt
func New(w io.Writer, r io.Reader) *Prompt {
	fd := getFd(r)
	return &Prompt{
		writer: w,
		reader: bufio.NewReader(r),
		fd:     fd,
	}
}

type fd interface {
	Fd() uintptr
}

func getFd(r io.Reader) int {
	if f, ok := r.(fd); ok {
		return int(f.Fd())
	}
	return -1
}

// Prompt can ask for inputs and validate them
type Prompt struct {
	writer io.Writer
	reader *bufio.Reader
	fd     int
}

func (p *Prompt) isTerminal() bool {
	return p.fd > -1 && term.IsTerminal(p.fd)
}

// Default sets the default value for the question
func (p *Prompt) Default(defaultTo string) *Question {
	q := newQuestion(p)
	q.defaultTo = defaultTo
	return q
}

// Optional sets the question as optional
func (p *Prompt) Optional(optional bool) *Question {
	q := newQuestion(p)
	q.optional = optional
	return q
}

// Is adds validators to the question
func (p *Prompt) Is(validators ...func(string) error) *Question {
	q := newQuestion(p)
	q.validators = append(q.validators, validators...)
	return q
}

// Ask asks a question and returns the input
func (p *Prompt) Ask(ctx context.Context, prompt string) (string, error) {
	q := newQuestion(p)
	return q.Ask(ctx, prompt)
}

// Password asks for a password and returns the input
func (p *Prompt) Password(ctx context.Context, prompt string) (string, error) {
	q := newQuestion(p)
	return q.Password(ctx, prompt)
}

// Confirm asks for a confirmation and returns the input
func (p *Prompt) Confirm(ctx context.Context, prompt string) (bool, error) {
	q := newQuestion(p)
	return q.Confirm(ctx, prompt)
}

func newQuestion(p *Prompt) *Question {
	return &Question{
		prompter: p,
	}
}

// Question that can be asked
type Question struct {
	prompter   *Prompt
	validators []func(string) error
	defaultTo  string
	optional   bool
}

func (q *Question) scanLine(inputCh chan<- string, errorCh chan<- error) {
	p := q.prompter

	// Read the input
	input, err := p.reader.ReadString('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) {
			errorCh <- err
			return
		}
		// If we're at the end of the input, and there is a default, use it,
		// otherwise return a required error
		if q.defaultTo != "" {
			inputCh <- q.defaultTo
			return
		} else if !q.optional {
			errorCh <- ErrRequired
			return
		}
	}

	// Trim the input
	input = strings.TrimRight(input, "\r\n")
	inputCh <- input
}

func (q *Question) readTerminalLine() (string, error) {
	p := q.prompter
	state, err := term.MakeRaw(p.fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(p.fd, state)

	line := []rune{}
	cursor := 0

	for {
		b, err := p.reader.ReadByte()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return "", err
			}
			return q.eofValue(string(line))
		}

		oldCursor := cursor
		switch b {
		case '\r', '\n':
			fmt.Fprint(p.writer, "\r\n")
			return string(line), nil
		case 0x03: // Ctrl+C
			return "", handleInterrupt(p.writer)
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
			seq, err := readEscapeSequence(p.reader)
			if err != nil {
				return "", err
			}
			line, cursor = applyEscapeSequence(seq, line, cursor)
		default:
			if err := p.reader.UnreadByte(); err != nil {
				return "", err
			}
			r, _, err := p.reader.ReadRune()
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

		redrawTerminalLine(p.writer, line, oldCursor, cursor)
	}
}

func handleInterrupt(w io.Writer) error {
	fmt.Fprint(w, "^C\r\n")
	return ErrInterrupted
}

func (q *Question) eofValue(input string) (string, error) {
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

func redrawTerminalLine(w io.Writer, line []rune, oldCursor, cursor int) {
	if oldCursor > 0 {
		fmt.Fprintf(w, "\x1b[%dD", oldCursor)
	}
	fmt.Fprint(w, string(line))
	fmt.Fprint(w, "\x1b[K")
	if back := len(line) - cursor; back > 0 {
		fmt.Fprintf(w, "\x1b[%dD", back)
	}
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
// otherwise read the line from the scanner
func (q *Question) scanPassword(inputCh chan<- string, errorCh chan<- error) {
	p := q.prompter

	if p.isTerminal() {
		pass, err := term.ReadPassword(p.fd)
		if err != nil {
			errorCh <- err
			return
		}
		inputCh <- string(pass)
		return
	}

	q.scanLine(inputCh, errorCh)
}

// Default sets the default value for the question
func (q *Question) Default(defaultTo string) *Question {
	q.defaultTo = defaultTo
	return q
}

// Optional sets the question as optional
func (q *Question) Optional(optional bool) *Question {
	q.optional = optional
	return q
}

// Is adds validators to the question
func (q *Question) Is(validators ...func(string) error) *Question {
	q.validators = append(q.validators, validators...)
	return q
}

// Reads the input from the reader
func (q *Question) readInput(ctx context.Context) (string, error) {
	// Check if the context has already been cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Terminal input is handled synchronously to guarantee raw mode cleanup.
	if q.prompter.isTerminal() {
		return q.readTerminalLine()
	}

	inputCh := make(chan string)
	errorCh := make(chan error)

	// Scan for the input in a goroutine, so we can listen for cancellations.
	go q.scanLine(inputCh, errorCh)

	// Wait for input, an error or the context to be cancelled
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

// Reads the password from the reader
func (q *Question) readPassword(ctx context.Context) (string, error) {
	// Check if the context has already been cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	inputCh := make(chan string)
	errorCh := make(chan error)

	// Scan for the password in a goroutine, so we can listen for cancelations.
	go q.scanPassword(inputCh, errorCh)

	// Wait for input, an error or the context to be cancelled
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

// Ask asks a question and returns the input
func (q *Question) Ask(ctx context.Context, prompt string) (string, error) {
	p := q.prompter

	// Write out the formatted prompt
retry:
	fmt.Fprint(p.writer, prompt, " ")

	// Read the input
	input, err := q.readInput(ctx)
	if err != nil {
		return "", err
	}

	// If the input is empty, and there is a default, use it otherwise ask again
	if input == "" {
		if q.defaultTo != "" {
			return q.defaultTo, nil
		} else if !q.optional {
			goto retry
		}
	}

	// If any validators fail, print the error and ask again
	for _, validate := range q.validators {
		if err := validate(input); err != nil {
			fmt.Fprintln(p.writer, err)
			goto retry
		}
	}

	return input, nil
}

// Password asks for a password and returns the input
func (q *Question) Password(ctx context.Context, prompt string) (string, error) {
	p := q.prompter

	// Write out the formatted prompt
retry:
	fmt.Fprint(p.writer, prompt, " ")

	// Read the input
	pass, err := q.readPassword(ctx)
	if err != nil {
		return "", err
	}
	// Print a newline after the password
	fmt.Fprintln(p.writer)

	if pass == "" {
		if q.defaultTo != "" {
			return q.defaultTo, nil
		} else if !q.optional {
			goto retry
		}
	}

	// If any validators fail, print the error and ask again
	for _, validate := range q.validators {
		if err := validate(pass); err != nil {
			fmt.Fprintln(p.writer, err)
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

// Confirm asks for a confirmation and returns the input
func (q *Question) Confirm(ctx context.Context, prompt string) (bool, error) {
	// Add a validator to ensure the input is yes or no
	q.validators = append(q.validators, func(s string) error {
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
