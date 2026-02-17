# Prompt

[![Go Reference](https://pkg.go.dev/badge/github.com/matthewmueller/prompt.svg)](https://pkg.go.dev/github.com/matthewmueller/prompt)

Minimal prompting library for Go.

## Features

- Functional options API
- Supports inputs, passwords and confirmations
- Supports validations, defaults and optionals
- Supports context canceling

## Install

```sh
go get github.com/matthewmueller/prompt
```

## Examples

```go
ctx := context.Background()

// Ask for some input
name, err := prompt.Ask(ctx, "What is your name?")

// Optional inputs
age, err := prompt.Ask(ctx, "What is your age?", prompt.WithOptional(true))

// Default values
age, err = prompt.Ask(ctx, "What is your age?", prompt.WithDefault("21"))

// Validations
func validPass(input string) error {
  if len(input) < 8 {
    return errors.New("password is too short")
  }
}

// Passwords
pass, err := prompt.Password(ctx, "What is your password?", prompt.WithCheck(validPass))

// Confirmations
shouldCreate, err := prompt.Confirm(ctx, "Create new user? (yes/no)")

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
age, err := prompt.Ask(ctx, "What is your age?", prompt.WithDefault("21"), prompt.WithCheck(validAge))

// Custom IO (optional)
age, err = prompt.Ask(ctx, "What is your age?",
  prompt.WithReader(reader),
  prompt.WithWriter(writer),
)
```

## Development

First, clone the repo:

```sh
git clone https://github.com/matthewmueller/prompt
cd prompt
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
