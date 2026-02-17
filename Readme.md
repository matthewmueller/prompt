# Prompter

**Deprecated:** use https://github.com/Bowery/prompt instead.

---

[![Go Reference](https://pkg.go.dev/badge/github.com/matthewmueller/prompter.svg)](https://pkg.go.dev/github.com/matthewmueller/prompter)

Minimal prompting library for Go.

## Features

- Functional options API
- Supports inputs, passwords and confirmations
- Supports validations, defaults and optionals
- Supports context canceling

## Install

```sh
go get github.com/matthewmueller/prompter
```

## Examples

```go
ctx := context.Background()

// Ask for some input
name, err := prompter.Ask(ctx, "What is your name?")

// Optional inputs
age, err := prompter.Ask(ctx, "What is your age?", prompter.WithOptional(true))

// Default values
age, err = prompter.Ask(ctx, "What is your age?", prompter.WithDefault("21"))

// Validations
func validPass(input string) error {
  if len(input) < 8 {
    return errors.New("password is too short")
  }
}

// Passwords
pass, err := prompter.Password(ctx, "What is your password?", prompter.WithCheck(validPass))

// Confirmations
shouldCreate, err := prompter.Confirm(ctx, "Create new user? (yes/no)")

// Multiple options
func validAge(input string) error {
  n, err := strconv.Atoi(input)
  if err != nil {
    return fmt.Errorf("%q must be a number")
  } else if n < 0 {
    return fmt.Errorf("%q must be greater than 0")
  }
  return nil
}
age, err := prompter.Ask(ctx, "What is your age?", prompter.WithDefault("21"), prompter.WithCheck(validAge))

// Custom IO (optional)
age, err = prompter.Ask(ctx, "What is your age?",
  prompter.WithReader(reader),
  prompter.WithWriter(writer),
)
```

## Development

First, clone the repo:

```sh
git clone https://github.com/matthewmueller/prompter
cd prompter
```

Next, install dependencies:

```sh
go mod tidy
```

Finally, try running the tests:

```sh
go test ./...
```

## License

MIT
