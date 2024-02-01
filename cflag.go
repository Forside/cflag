package cflag

import (
	"bytes"
	"fmt"
	flag "github.com/spf13/pflag"
	"os"
	"slices"
	"strings"
)

type Command struct {
	name        string
	usage       string
	description string
	flags       *flag.FlagSet
	commands    []*Command
	active      bool
}

var command Command

// AddCommand adds command to the global register.
// When a command with the same name already exists,
// the operation is cancelled and false is returned.
// Reports whether the command is added successfully.
func (c *Command) AddCommand(command *Command) bool {
	if command == nil || len(command.name) == 0 {
		return false
	}

	// Check if a command with the same name is already defined.
	if slices.ContainsFunc(c.commands, func(cmd *Command) bool {
		return cmd.name == command.name
	}) {
		return false
	}

	c.commands = append(c.commands, command)
	return true
}

// Cmd creates and adds a new command to the global register.
// When the command is added successfully, the Command value is returned.
// Else nil is returned.
func (c *Command) Cmd(name string, usage string, flags *flag.FlagSet) *Command {
	cmd := NewCommand(name, usage, flags)
	if c.AddCommand(cmd) {
		return cmd
	} else {
		return nil
	}
}

func (c *Command) SetDescription(description string) {
	c.description = description
}

func (c *Command) IsActive() bool {
	return c.active
}

func (c *Command) FindActive(name string) *Command {
	if len(name) == 0 {
		return c
	}

	// Find active command with matching name.
	if iCmd := slices.IndexFunc(c.commands, func(cmd *Command) bool {
		return cmd.active && cmd.name == name
	}); iCmd >= 0 {
		return c.commands[iCmd]
	}

	return nil
}

func (c *Command) CommandUsages() string {
	buf := new(bytes.Buffer)

	// Add command usage.
	if len(c.usage) > 0 {
		_, _ = fmt.Fprintln(buf, c.usage)
	}

	// Add command description.
	if len(c.description) > 0 {
		_, _ = fmt.Fprintln(buf, c.description)
	}

	// Add sub-command usages.
	if len(c.commands) > 0 {
		_, _ = fmt.Fprintln(buf, "Commands:")

		// Find maximum name length to calculate gap width.
		maxNameLen := 0
		for _, cmd := range c.commands {
			nameLen := len(cmd.name)
			if nameLen > maxNameLen {
				maxNameLen = nameLen
			}
		}

		// Create line containing command name and usage.
		for _, cmd := range c.commands {
			nameLen := len(cmd.name)
			gapLen := maxNameLen - nameLen + 3
			gap := strings.Repeat(" ", gapLen)
			_, _ = fmt.Fprintln(buf, "  "+cmd.name+gap+cmd.usage)
		}
	}

	// Add flag usages.
	if c.flags.HasAvailableFlags() {
		_, _ = fmt.Fprintln(buf, "Flags:")
		_, _ = fmt.Fprint(buf, c.flags.FlagUsages())
	}

	//return usages
	return buf.String()
}

func (c *Command) Parse(arguments []string) {
	if len(arguments) == 0 {
		return
	}

	var argsBeforeCmd []string
	var argsAfterCmd []string
	var cmd *Command

	// Search matching sub-command in arguments.
	if len(c.commands) > 0 {
		for iArg, arg := range arguments {
			if iCmd := slices.IndexFunc(c.commands, func(cmd *Command) bool {
				return cmd.name == arg
			}); iCmd >= 0 {
				// Cache arguments before and after command.
				cmd = c.commands[iCmd]
				argsBeforeCmd = arguments[:iArg]
				argsAfterCmd = arguments[iArg+1:]
				break
			}
		}
	}

	// Use all arguments when no sub-command is found.
	if cmd == nil {
		argsBeforeCmd = arguments
	}

	// Parse arguments for this command.
	if c.flags != nil && len(argsBeforeCmd) > 0 {
		// Add help option when none is set.
		paramHelp := new(bool)
		*paramHelp = false
		if _, err := c.flags.GetBool("help"); err != nil {
			paramHelp = c.flags.BoolP("help", "h", false, "Display help.")
		}

		// Parse command.
		_ = c.flags.Parse(argsBeforeCmd)

		// Print help and exit when help option is set.
		if *paramHelp {
			print(c.CommandUsages())
			os.Exit(0)
		}
	}

	// Parse sub-command.
	if cmd != nil {
		cmd.active = true
		cmd.Parse(argsAfterCmd)
	}
}

func NewCommand(name string, usage string, flags *flag.FlagSet) *Command {
	return &Command{
		name:     name,
		usage:    usage,
		flags:    flags,
		commands: nil,
		active:   false,
	}
}

func AddCommand(command *Command) bool {
	return command.AddCommand(command)
}

func Cmd(name string, usage string, flags *flag.FlagSet) *Command {
	return command.Cmd(name, usage, flags)
}

func SetDescription(description string) {
	command.SetDescription(description)
}

func IsActive() bool {
	return command.IsActive()
}

func FindActive(name string) *Command {
	return command.FindActive(name)
}

func CommandUsages() string {
	return command.CommandUsages()
}

func Parse(arguments []string, flags *flag.FlagSet) {
	command.flags = flags
	command.active = true
	command.Parse(arguments[1:])
}

func Reset() {
	command = Command{
		name:     "",
		usage:    "",
		flags:    nil,
		commands: nil,
		active:   false,
	}
}
