package prompter_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/matryer/is"
	"github.com/matthewmueller/diff"
	"github.com/matthewmueller/prompter"
)

func TestAsk(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("Mark\n27\n")
	withReader := prompter.WithReader(reader)
	withWriter := prompter.WithWriter(io.Discard)

	name, err := prompter.Ask(ctx, "What is your name?",
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(name, "Mark")

	age, err := prompter.Ask(ctx, "What is your age?",
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(age, "27")
}

func TestAskWithoutPromptInitialization(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	writer := new(bytes.Buffer)
	reader := bytes.NewBufferString("Mark\n")

	name, err := prompter.Ask(ctx, "What is your name?",
		prompter.WithWriter(writer),
		prompter.WithReader(reader),
	)
	is.NoErr(err)
	is.Equal(name, "Mark")
	is.Equal(writer.String(), "What is your name? ")
}

func TestAskWithReaderOptionIsPerCall(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	overridden, err := prompter.Ask(ctx, "Override name?",
		prompter.WithReader(bytes.NewBufferString("Amy\n")),
		prompter.WithWriter(io.Discard),
	)
	is.NoErr(err)
	is.Equal(overridden, "Amy")

	name, err := prompter.Ask(ctx, "What is your name?",
		prompter.WithReader(bytes.NewBufferString("Mark\n")),
		prompter.WithWriter(io.Discard),
	)
	is.NoErr(err)
	is.Equal(name, "Mark")
}

func TestAskErrRequired(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("Mark\n27\n")
	withReader := prompter.WithReader(reader)
	withWriter := prompter.WithWriter(io.Discard)

	name, err := prompter.Ask(ctx, "What is your name?",
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(name, "Mark")

	age, err := prompter.Ask(ctx, "What is your age?",
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(age, "27")

	height, err := prompter.Ask(ctx, "What is your height?",
		withReader,
		withWriter,
	)
	is.True(errors.Is(err, prompter.ErrRequired))
	is.Equal(height, "")
}

func TestAskOptional(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("Mark\n")
	withReader := prompter.WithReader(reader)
	withWriter := prompter.WithWriter(io.Discard)

	name, err := prompter.Ask(ctx, "What is your name?",
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(name, "Mark")

	age, err := prompter.Ask(ctx, "What is your age?",
		prompter.WithOptional(true),
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(age, "")
}

func TestAskDefault(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("Mark\n")
	withReader := prompter.WithReader(reader)
	withWriter := prompter.WithWriter(io.Discard)

	name, err := prompter.Ask(ctx, "What is your name?",
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(name, "Mark")

	age, err := prompter.Ask(ctx, "What is your age?",
		prompter.WithDefault("21"),
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(age, "21")
}

func TestAskValidate(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	writer := new(bytes.Buffer)
	reader := bytes.NewBufferString("Am\nAmy\n")
	validName := func(s string) error {
		if len(s) < 3 {
			return fmt.Errorf("'%s' is too short", s)
		}
		return nil
	}

	name, err := prompter.Ask(ctx, "What is your name?",
		prompter.WithCheck(validName),
		prompter.WithWriter(writer),
		prompter.WithReader(reader),
	)
	is.NoErr(err)
	is.Equal(name, "Amy")
	diff.TestString(t, writer.String(), "What is your name? 'Am' is too short\nWhat is your name? ")
}

func TestAskDefaultGiven(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("Mark\n27\n")
	withReader := prompter.WithReader(reader)
	withWriter := prompter.WithWriter(io.Discard)

	name, err := prompter.Ask(ctx, "What is your name?",
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(name, "Mark")

	age, err := prompter.Ask(ctx, "What is your age?",
		prompter.WithDefault("21"),
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(age, "27")
}

func TestAskDefaultOptional(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("Mark\n")
	withReader := prompter.WithReader(reader)
	withWriter := prompter.WithWriter(io.Discard)

	name, err := prompter.Ask(ctx, "What is your name?",
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(name, "Mark")

	age, err := prompter.Ask(ctx, "What is your age?",
		prompter.WithOptional(true),
		prompter.WithDefault("21"),
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(age, "21")
}

func TestAskDefaultOptionalGiven(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("Mark\n27\n")
	withReader := prompter.WithReader(reader)
	withWriter := prompter.WithWriter(io.Discard)

	name, err := prompter.Ask(ctx, "What is your name?",
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(name, "Mark")

	age, err := prompter.Ask(ctx, "What is your age?",
		prompter.WithOptional(true),
		prompter.WithDefault("21"),
		withReader,
		withWriter,
	)
	is.NoErr(err)
	is.Equal(age, "27")
}

func TestPassword(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("some password\n")

	pass, err := prompter.Password(ctx, "What is your password?",
		prompter.WithReader(reader),
		prompter.WithWriter(io.Discard),
	)
	is.NoErr(err)
	is.Equal(pass, "some password")
}

func TestPasswordDefault(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("")

	pass, err := prompter.Password(ctx, "What is your password?",
		prompter.WithDefault("idk"),
		prompter.WithReader(reader),
		prompter.WithWriter(io.Discard),
	)
	is.NoErr(err)
	is.Equal(pass, "idk")
}

func TestPasswordOptional(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("")

	pass, err := prompter.Password(ctx, "What is your password?",
		prompter.WithOptional(true),
		prompter.WithReader(reader),
		prompter.WithWriter(io.Discard),
	)
	is.NoErr(err)
	is.Equal(pass, "")
}

func TestPasswordValidate(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("mypassword\nsome password\n")
	validate := func(s string) error {
		if s != "some password" {
			return errors.New("invalid password")
		}
		return nil
	}

	pass, err := prompter.Password(ctx, "What is your password?",
		prompter.WithCheck(validate),
		prompter.WithReader(reader),
		prompter.WithWriter(io.Discard),
	)
	is.NoErr(err)
	is.Equal(pass, "some password")
}

func TestConfirmTrue(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("hello\nyes\n")

	create, err := prompter.Confirm(ctx, "Create new user? (yes/no)",
		prompter.WithReader(reader),
		prompter.WithWriter(io.Discard),
	)
	is.NoErr(err)
	is.Equal(create, true)
}

func TestConfirmFalse(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	reader := bytes.NewBufferString("hello\nno\n")

	create, err := prompter.Confirm(ctx, "Create new user? (yes/no)",
		prompter.WithReader(reader),
		prompter.WithWriter(io.Discard),
	)
	is.NoErr(err)
	is.Equal(create, false)
}

func TestAskCancel(t *testing.T) {
	is := is.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := bytes.NewBufferString("Mark\n")
	cancel() // Cancel the context before asking

	name, err := prompter.Ask(ctx, "What is your name?",
		prompter.WithReader(reader),
		prompter.WithWriter(io.Discard),
	)
	is.True(errors.Is(err, context.Canceled))
	is.Equal(name, "")
}

func TestPasswordCancel(t *testing.T) {
	is := is.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := bytes.NewBufferString("some password\n")
	cancel() // Cancel the context before asking

	_, err := prompter.Password(ctx, "What is your password?",
		prompter.WithReader(reader),
		prompter.WithWriter(io.Discard),
	)
	is.True(errors.Is(err, context.Canceled))
}

func TestConfirmCancel(t *testing.T) {
	is := is.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := bytes.NewBufferString("yes\n")
	cancel() // Cancel the context before asking

	_, err := prompter.Confirm(ctx, "Create new user? (yes/no)",
		prompter.WithReader(reader),
		prompter.WithWriter(io.Discard),
	)
	is.True(errors.Is(err, context.Canceled))
}
