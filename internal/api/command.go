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

func (c Command) Validate(user map[string]struct{}, exe map[string]struct{}, args ...func(string) error) error {
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
			fmt.Errorf("error for argument '%s' at %d: %w", options[i], i, err)
		}
	}

	return nil
}

func Any() map[string]struct{} {
	return map[string]struct{}{}
}

func Contains(arg ...string) map[string]struct{} {
	args := make(map[string]struct{})
	for _, a := range arg {
		args[a] = struct{}{}
	}
	return args
}

func NotEmpty(s string) error {
	if s == "" {
		return fmt.Errorf("cannot be empty")
	}
	return nil
}

func OneOf(v *string, args ...string) func(string) error {
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
		v = &s
		return nil
	}
}

func Int(d *int) func(s string) error {
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
		d = &i
		return nil
	}
}
