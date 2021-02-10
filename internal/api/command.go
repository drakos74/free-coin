package api

import (
	"fmt"
	"strconv"
	"strings"
)

// Command is the definitions of metadata for a command.
type Command struct {
	ID      int
	User    string
	Content string
}

// Validator is a validation function that checks the string for the given type.
type Validator func(string) error

// Validate validates the command with the given arguments.
func (c Command) Validate(user map[string]struct{}, exe map[string]struct{}, args ...Validator) error {
	if _, ok := user[c.User]; !ok && len(user) > 0 {
		return fmt.Errorf("command cannot be executed: %s", c.User)
	}
	// TODO : find a better way to parse the arguments i.e. with regex
	cmd := strings.Split(c.Content, " ")
	if len(cmd) == 0 {
		return fmt.Errorf("cannot parse empty command: %s", c.Content)
	}
	exec := cmd[0]
	if _, ok := exe[exec]; !ok && len(exe) > 0 {
		return fmt.Errorf("unknown command: %s", exec)
	}

	options := cmd[1:]

	for i, arg := range args {
		err := arg(options[i])
		if err != nil {
			return fmt.Errorf("error for argument '%s' at %d: %w", options[i], i, err)
		}
	}
	return nil
}

// Any is a predefined validator for any value.
func Any() map[string]struct{} {
	return map[string]struct{}{}
}

// Contains is a predefined validator for the argument being one of the given values.
func Contains(arg ...string) map[string]struct{} {
	args := make(map[string]struct{})
	for _, a := range arg {
		args[a] = struct{}{}
	}
	return args
}

// NotEmpty is a predefined Validator that checks if the argument is empty.
func NotEmpty(s string) error {
	if s == "" {
		return fmt.Errorf("cannot be empty")
	}
	return nil
}

// OneOf is a predefined Validator checking that the value is on of the provided arguments.
// it passes the reference to the value to the given interface argument.
func OneOf(v *string, args ...string) Validator {
	return func(s string) error {
		var isOneOf bool
		for _, arg := range args {
			if arg == s {
				isOneOf = true
			}
		}
		if !isOneOf {
			return fmt.Errorf("must be one of %v", args)
		}
		*v = s
		return nil
	}
}

// Int is a predefined Validator checking that the argument is an int.
// it passes the reference to the value to the given interface argument.
func Int(d *int) Validator {
	return func(s string) error {
		if s == "" {
			i := 0
			d = &i
			return nil
		}
		number, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		i := int(number)
		*d = i
		return nil
	}
}
