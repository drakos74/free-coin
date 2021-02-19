package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand_Validate(t *testing.T) {

	type test struct {
		cmd           Command
		userValidator map[string]struct{}
		execValidator map[string]struct{}
		options       []Validator
		values        []interface{}
		err           bool
	}

	var option_1 string
	var option_2 string
	var option_int int

	tests := map[string]test{
		"no-user-any": {
			cmd:           Command{},
			userValidator: AnyUser(),
		},
		"no-user-err": {
			cmd:           Command{},
			userValidator: Contains("test"),
			err:           true,
		},
		"wrong-user": {
			cmd: Command{
				User: "test-user",
			},
			userValidator: Contains("test"),
			err:           true,
		},
		"correct-user": {
			cmd: Command{
				User: "test-user",
			},
			userValidator: Contains("test-user"),
		},
		"correct-exec": {
			cmd: Command{
				User:    "test-user",
				Content: "command option-1 option-2",
			},
			userValidator: Contains("test-user"),
			execValidator: Contains("command"),
		},
		"no-exec": {
			cmd: Command{
				User:    "test-user",
				Content: "no-command option-1 option-2",
			},
			userValidator: Contains("test-user"),
			execValidator: Contains("command"),
			err:           true,
		},
		"options-any-nil": {
			cmd: Command{
				User:    "test-user",
				Content: "no-command option-1 option-2",
			},
			userValidator: Contains("test-user"),
			execValidator: Contains("command"),
			options:       []Validator{OneOf(nil, "option-1"), OneOf(nil, "option-2")},
			err:           true,
		},
		"options-any-value": {
			cmd: Command{
				User:    "test-user",
				Content: "command option-1 option-2 30",
			},
			userValidator: Contains("test-user"),
			execValidator: Contains("command"),
			values:        []interface{}{"option-1", "option-2", 30},
			options:       []Validator{OneOf(&option_1, "option-1"), OneOf(&option_2, "option-2"), Int(&option_int)},
		},
		"options-any-value-format": {
			cmd: Command{
				User:    "test-user",
				Content: "command option-1    option-2 30",
			},
			userValidator: Contains("test-user"),
			execValidator: Contains("command"),
			options:       []Validator{OneOf(&option_1, "option-1"), OneOf(&option_2, "option-2"), Int(&option_int)},
			err:           true,
		},
		"options-arg-value-format": {
			cmd: Command{
				User:    "test-user",
				Content: "command option-2 option-2 30",
			},
			userValidator: Contains("test-user"),
			execValidator: Contains("command"),
			options:       []Validator{OneOf(&option_1, "option-1"), OneOf(&option_2, "option-2"), Int(&option_int)},
			err:           true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := tt.cmd.Validate(tt.userValidator, tt.execValidator, tt.options...)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if len(tt.values) > 0 {
					assert.Equal(t, tt.values[0], option_1)
					assert.Equal(t, tt.values[1], option_2)
					assert.Equal(t, tt.values[2], option_int)
				}
			}
		})
	}

}

func TestOneOf(t *testing.T) {

	type test struct {
		values []string
		arg    string
		err    bool
		value  string
	}

	tests := map[string]test{
		"no-one-of": {
			values: []string{"test-1", "test-2", "test-3"},
			arg:    "test-4",
			err:    true,
		},
		"is-one-of": {
			values: []string{"test-1", "test-2", "test-3", "test-4"},
			arg:    "test-4",
			value:  "test-4",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			var v string
			err := OneOf(&v, tt.values...)(tt.arg)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.value, v)
			}

		})
	}

}

func TestInt(t *testing.T) {

	type test struct {
		arg   string
		err   bool
		value int
	}

	tests := map[string]test{
		"no-int-string": {
			arg: "test-4",
			err: true,
		},
		"no-int": {
			arg: "4asd",
			err: true,
		},
		"no-int-decimal-.": {
			arg: "4.54",
			err: true,
		},
		"no-int-decimal-,": {
			arg: "4,54",
			err: true,
		},
		"is-int": {
			arg:   "4",
			value: 4,
		},
		"is-int-neg": {
			arg:   "-4",
			value: -4,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			var v int
			err := Int(&v)(tt.arg)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.value, v)
			}

		})
	}

}
