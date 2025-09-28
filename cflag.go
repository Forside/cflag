package cflag

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	flag "github.com/spf13/pflag"
	"golang.org/x/term"
)

type UsageFunc func(command *Command)
type CommandCallback func(command *Command, flags *flag.FlagSet) error

// A Command represents a (sub)command with a set of defined flags.
type Command struct {
	name        string
	usage       string
	description string
	active      bool
	hidden      bool
	deprecated  bool
	recurseArgs bool
	flags       *flag.FlagSet
	parent      *Command
	commands    []*Command
	output      io.Writer
	usageFunc   UsageFunc
	callback    CommandCallback
}

const (
	// The gap between the start of the line and the command name on the help page.
	commandGapLen = 2
	// The minimum gap between the command name and the command usage on the help page.
	commandUsageGapLen = 3
)

// Holds the global command register,
// i.e. top-level flags and commands defined for the application.
var command Command

// AddCommand adds command as a subcommand.
// When a command with the same name already exists,
// the operation is cancelled and an error is returned.
func (c *Command) AddCommand(command *Command) error {
	if command == nil || len(command.name) == 0 {
		return fmt.Errorf("invalid parameters")
	}

	// Check if a command with the same name is already defined.
	if slices.ContainsFunc(c.commands, func(cmd *Command) bool {
		return cmd.name == command.name
	}) {
		return fmt.Errorf("command with name '%s' already exists", command.name)
	}

	command.parent = c
	c.commands = append(c.commands, command)
	return nil
}

// Cmd creates a new command and adds it as a subcommand.
// When the command is added successfully, the Command value is returned.
// Else nil and an error is returned.
func (c *Command) Cmd(name string, usage string, flags *flag.FlagSet) (*Command, error) {
	cmd := NewCommand(name, usage, flags)
	if err := c.AddCommand(cmd); err != nil {
		return nil, err
	}
	return cmd, nil
}

// SetDescription defines a long description that is
// displayed on the generated help page. See CommandUsages.
func (c *Command) SetDescription(description string) *Command {
	c.description = description
	return c
}

// SetUsageFunc sets the function which prints the command usage when the
// --help flag is recognized during parsing.
// By default, it prints the output of CommandUsage which is roughly equivalent to
// fmt.Printf("%s\n%s\nCommands:\n%sFlags:\n%s", c.GetUsage(), c.GetDescription(), c.CommandUsages(), c.FlagUsages())
func (c *Command) SetUsageFunc(usageFunc UsageFunc) *Command {
	c.usageFunc = usageFunc
	return c
}

// SetCallback sets the function which is executed when the command
// is the last active command with a callback defined at the end of the parsing process.
// The last active command (with or without a callback defined)
// is passed to the callback.
func (c *Command) SetCallback(callback CommandCallback) *Command {
	c.callback = callback
	return c
}

// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.
func (c *Command) SetOutput(output io.Writer) *Command {
	c.output = output
	return c
}

// MarkHidden sets the command to 'hidden'. It will continue to
// function but will not show up in help or usage messages.
func (c *Command) MarkHidden() *Command {
	c.hidden = true
	return c
}

// MarkDeprecated indicates that the command is deprecated. It will
// continue to function but will not show up in help or usage messages.
func (c *Command) MarkDeprecated() *Command {
	c.hidden = true
	c.deprecated = true
	return c
}

// SetRecurseArguments enables recursive parsing of arguments
// using the parent commands of this command.
func (c *Command) SetRecurseArguments() *Command {
	c.recurseArgs = true
	return c
}

// IsActive reports whether the command is active,
// i.e. it was supplied to the command line when calling Parse.
func (c *Command) IsActive() bool {
	return c.active
}

// IsHidden reports whether the command is marked as hidden,
// i.e. it is not listed in help and usage messages.
func (c *Command) IsHidden() bool {
	return c.hidden
}

// IsDeprecated reports whether the command is marked as deprecated,
// i.e. it is not listed in help and usage messages and a warning is
// displayed on its help message.
func (c *Command) IsDeprecated() bool {
	return c.deprecated
}

// GetName returns the command name.
func (c *Command) GetName() string {
	return c.name
}

// GetUsage returns the command usage.
func (c *Command) GetUsage() string {
	return c.usage
}

// GetDescription returns the command description if set.
// See Command.SetDescription.
func (c *Command) GetDescription() string {
	return c.description
}

// GetFlags returns the registered FlagSet for the command.
func (c *Command) GetFlags() *flag.FlagSet {
	return c.flags
}

// Lookup searches for a registered subcommand by its name.
// If no matching command is found, nil is returned.
// Subsequent subcommands can be found by supplying multiple names.
// When no or an empty name is supplied, the command itself is returned.
func (c *Command) Lookup(names ...string) *Command {
	// Return the command itself when no name is supplied.
	if len(names) == 0 {
		return c
	}

	cmd := c
	for _, name := range names {
		// Keep current command on empty name.
		if name == "" {
			continue
		}

		// Find command with matching name.
		if iCmd := slices.IndexFunc(cmd.commands, func(cmd *Command) bool {
			return cmd.name == name
		}); iCmd >= 0 {
			cmd = cmd.commands[iCmd]
		} else {
			return nil
		}
	}

	return cmd
}

// Active searches for a registered subcommand by its name
// and reports its activation state. See IsActive.
func (c *Command) Active(name string) bool {
	// Lookup command with matching name.
	if cmd := c.Lookup(name); cmd != nil {
		return cmd.IsActive()
	}
	return false
}

// GetActiveSubcommand returns the active subcommand of the command
// or nil when no subcommand is active.
func (c *Command) GetActiveSubcommand() *Command {
	for _, cmd := range c.commands {
		if cmd.active {
			return cmd
		}
	}
	return nil
}

// GetFinalActiveCommand returns the final active command in the command chain.
// When no subcommand is active, the command itself is returned.
func (c *Command) GetFinalActiveCommand() *Command {
	cmd := c

	for {
		if subCmd := cmd.GetActiveSubcommand(); subCmd != nil {
			cmd = subCmd
		} else {
			break
		}
	}

	return cmd
}

// GetActiveCommandChain returns a slice containing all active commands,
// where the first element is the command itself.
func (c *Command) GetActiveCommandChain() []*Command {
	var cmdChain []*Command
	cmd := c

	for cmd != nil {
		cmdChain = append(cmdChain, cmd)
		cmd = cmd.GetActiveSubcommand()
	}

	return cmdChain
}

// CommandUsagesWrapped returns a string containing the usage information
// for all subcommands defined for this command.
// Wrapped to cols columns (0 for no wrapping).
func (c *Command) CommandUsagesWrapped(cols int) string {
	if len(c.commands) == 0 {
		return ""
	}

	buf := new(bytes.Buffer)

	// Filter visible commands.
	visibleCommands := filterSlice(c.commands, func(c *Command) bool {
		return !c.hidden
	})

	// Find maximum name length to calculate gap width.
	maxNameLen := 0
	for _, cmd := range visibleCommands {
		nameLen := len(cmd.name)
		if nameLen > maxNameLen {
			maxNameLen = nameLen
		}
	}

	// Get the full gap until usages are printed for wrapping.
	fullUsageGapLen := commandGapLen + maxNameLen + commandUsageGapLen

	// Create line containing command name and usage.
	for _, cmd := range visibleCommands {
		nameLen := len(cmd.name)
		gap := strings.Repeat(" ", commandGapLen)
		usageGapLen := maxNameLen - nameLen + commandUsageGapLen
		usageGap := strings.Repeat(" ", usageGapLen)
		cmdUsage := wrap(fullUsageGapLen, cols, cmd.usage)
		_, _ = fmt.Fprintf(buf, "%s%s%s%s\n", gap, cmd.name, usageGap, cmdUsage)
	}

	// Return usages string.
	return buf.String()
}

// CommandUsages returns a string containing the usage information
// for all subcommands defined for this command.
func (c *Command) CommandUsages() string {
	return c.CommandUsagesWrapped(0)
}

// FlagUsagesWrapped returns a string containing the usage information
// for all flags defined for this command.
// Wrapped to cols columns (0 for no wrapping).
func (c *Command) FlagUsagesWrapped(cols int) string {
	if c.flags == nil {
		return ""
	}
	return c.flags.FlagUsagesWrapped(cols)
}

// FlagUsages returns a string containing the usage information for all flags
// defined for this command.
func (c *Command) FlagUsages() string {
	return c.FlagUsagesWrapped(0)
}

// CommandUsage returns a string containing the usage information
// for this command and all subcommands, including the
// description for this command if defined.
func (c *Command) CommandUsage() string {
	buf := new(bytes.Buffer)
	needsSeparatorLine := false

	// Add deprecated warning.
	if c.deprecated {
		_, _ = fmt.Fprintln(buf, "! DEPRECATED !")
	}

	// Add command usage.
	if len(c.usage) > 0 {
		_, _ = fmt.Fprintln(buf, c.usage)
		needsSeparatorLine = true
	}

	// Add command description.
	if len(c.description) > 0 {
		_, _ = fmt.Fprintln(buf, c.description)
		needsSeparatorLine = true
	}

	// Get terminal width to wrap subcommand and flag usages.
	termWidth, _, _ := getTermSize()

	// Add subcommands.
	if len(c.commands) > 0 {
		if needsSeparatorLine {
			_, _ = fmt.Fprintln(buf)
		}
		_, _ = fmt.Fprintln(buf, "Commands:")
		_, _ = fmt.Fprint(buf, c.CommandUsagesWrapped(termWidth))
		needsSeparatorLine = true
	}

	// Add flag usages.
	if c.flags.HasAvailableFlags() {
		if needsSeparatorLine {
			_, _ = fmt.Fprintln(buf)
		}
		_, _ = fmt.Fprintln(buf, "Flags:")
		_, _ = fmt.Fprint(buf, c.FlagUsagesWrapped(termWidth))
	}

	return buf.String()
}

// Parse parses the command line arguments respecting the defined
// command structure. Arguments for each command are parsed using pflag.
func (c *Command) Parse(arguments []string) error {
	return c.parse(arguments, true)
}

func parseCmd(cmd *Command, arguments []string, executeCallback bool) (subCmd *Command, argsBeforeSubCmd, argsAfterSubCmd []string, err error) {
	// Mark command as active.
	cmd.active = true

	// Search for matching subcommand in arguments.
	if len(cmd.commands) > 0 {
		for iArg, arg := range arguments {
			// Find argument in command names.
			if subCmdTmp := cmd.Lookup(arg); subCmdTmp != nil {
				// Check whether the valid command name is a parameter value.
				argIsValue := false
				if iArg > 0 && cmd.flags != nil {
					prevArg := arguments[iArg-1]
					if f, r := findLastFlagFromArgC(cmd, prevArg); f != nil {
						argIsValue = f.Value.Type() != "bool" && f.Value.Type() != "boolSlice" && r == ""
					}
				}

				// Remember subcommand for next loop
				// and cache arguments before and after command name.
				if !argIsValue {
					subCmd = subCmdTmp
					argsBeforeSubCmd = arguments[:iArg]
					argsAfterSubCmd = arguments[iArg+1:]
					break
				}
			}
		}
	}

	// Use all arguments when no subcommand is found.
	if subCmd == nil {
		argsBeforeSubCmd = arguments
	}

	// Create flag set if unset.
	if cmd.flags == nil {
		cmd.flags = NewFlagSet("", flag.ExitOnError)
	}

	// Add help flag if unset.
	if cmd.flags.Lookup("help") == nil {
		if cmd.flags.ShorthandLookup("h") == nil {
			cmd.flags.BoolP("help", "h", false, "Display help.")
		} else if cmd.flags.ShorthandLookup("?") == nil {
			cmd.flags.BoolP("help", "?", false, "Display help.")
		} else {
			cmd.flags.Bool("help", false, "Display help.")
		}
	}

	// Parse command arguments.
	_ = cmd.flags.Parse(argsBeforeSubCmd)

	// Print help and exit when help flag is set.
	if paramHelp, err := cmd.flags.GetBool("help"); err == nil && paramHelp {
		cmd.printUsage()
		os.Exit(0)
	}

	// When recurseArgs is on, parse the arguments for the current command
	// using all parent commands.
	if cmd.recurseArgs && cmd.parent != nil && len(argsBeforeSubCmd) > 0 {
		_, _, _, _ = parseCmd(cmd.parent, argsBeforeSubCmd, false)
	}

	// Print deprecated warning.
	if cmd.deprecated {
		_, _ = fmt.Fprintf(cmd.out(), "Command %q is deprecated!\n", cmd.name)
	}

	return
}

// parse parses the command line arguments respecting the defined
// command structure. Arguments for each command are parsed using pflag.
// If executeCallback is true, the callback defined for the last active command
// will be executed (or the global callback if defined).
func (c *Command) parse(arguments []string, executeCallback bool) error {
	if len(arguments) == 0 {
		return os.ErrInvalid
	}

	cmd := c
	var subCmd *Command
	var err error

	// Check if the command name is empty (top-level command)
	// or matches the first argument (subcommand).
	if cmd.name != "" && cmd.name != arguments[0] {
		return fmt.Errorf("command %q does not match arguments", cmd.name)
	}

	// Remove command name from arguments.
	arguments = arguments[1:]

	// Slice to keep track of the chain of active commands.
	var cmdChain []*Command

	// Parse arguments and handle all commands and flags.
	for {
		if subCmd, _, arguments, err = parseCmd(cmd, arguments, executeCallback); err != nil {
			return err
		}

		// Add command to chain.
		cmdChain = append(cmdChain, cmd)

		if subCmd == nil {
			// No subcommand found. Exit loop.
			break
		}

		// Continue parsing the subcommand.
		cmd = subCmd
	}

	// Execute the callback function of the last active command which has a callback defined,
	// or the global callback function (if defined).
	if executeCallback {
		for i := range cmdChain {
			callbackCmd := cmdChain[len(cmdChain)-1-i]
			if callbackCmd.callback != nil || i == len(cmdChain)-1 {
				return callbackCmd.execCallback(cmd)
			}
		}
	}

	return nil
}

// printUsage calls the function defined via Command.SetUsageFunc
// or SetUsageFunc, or defaultUsage when both are undefined.
func (c *Command) printUsage() {
	if c.usageFunc != nil {
		c.usageFunc(c)
	} else if command.usageFunc != nil {
		command.usageFunc(c)
	} else {
		defaultUsage(c)
	}
}

// execCallback runs the callback defined via Command.SetCallback or SetCallback.
// When a target is supplied, it is passed to the callback instead of the command itself.
func (c *Command) execCallback(target *Command) error {
	var cb CommandCallback
	var cmd *Command

	// Use either the callback defined for this command or the
	// global command callback. Exit if no callback is defined.
	if c.callback != nil {
		cb = c.callback
	} else if command.callback != nil {
		cb = command.callback
	} else {
		return nil
	}

	// Pass either the supplied target or this command to the callback.
	if target != nil {
		cmd = target
	} else {
		cmd = c
	}

	// Execute the callback.
	return cb(cmd, cmd.flags)
}

// out returns the output stream defined for c or the global command,
// or os.Stderr if both are undefined.
func (c *Command) out() io.Writer {
	if c.output != nil {
		return c.output
	} else if command.output != nil {
		return command.output
	} else {
		return os.Stderr
	}
}

// NewFlagSet creates a flag.FlagSet with ParseErrorsWhitelist.UnknownFlags enabled,
// which is required to process subcommands.
func NewFlagSet(name string, errorHandling flag.ErrorHandling) *flag.FlagSet {
	flagSet := flag.NewFlagSet(name, errorHandling)
	flagSet.ParseErrorsWhitelist.UnknownFlags = true
	return flagSet
}

// NewCommand creates a new Command object for use with AddCommand.
// A top-level command must have an empty name.
// Use cflag.NewFlagSet to create the flag.FlagSet.
func NewCommand(name string, usage string, flags *flag.FlagSet) *Command {
	return &Command{
		name:  name,
		usage: usage,
		flags: flags,
	}
}

// AddCommand adds command to the global register.
// When a command with the same name already exists,
// the operation is cancelled and an error is returned.
func AddCommand(command *Command) error {
	return command.AddCommand(command)
}

// Cmd creates and adds a new command to the global register.
// When the command is added successfully, the Command value is returned.
// Else nil and an error is returned.
func Cmd(name string, usage string, flags *flag.FlagSet) (*Command, error) {
	return command.Cmd(name, usage, flags)
}

// SetDescription defines a long description that is
// displayed on the generated help page. See CommandUsages.
func SetDescription(description string) *Command {
	command.SetDescription(description)
	return &command
}

// SetUsageFunc sets the function which prints the command usage when the
// --help flag is recognized during parsing.
// By default, it prints the output of CommandUsage which is roughly equivalent to
// fmt.Printf("%s\n%s\nCommands:\n%sFlags:\n%s", c.GetUsage(), c.GetDescription(), c.CommandUsages(), c.FlagUsages())
func SetUsageFunc(usageFunc UsageFunc) *Command {
	command.SetUsageFunc(usageFunc)
	return &command
}

// SetCallback sets the global function which is executed at the end
// of the parsing process, when no callback is defined for the active command.
// The last active command (with or without a callback defined)
// is passed to the callback.
func SetCallback(callback CommandCallback) *Command {
	command.SetCallback(callback)
	return &command
}

// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.
func SetOutput(output io.Writer) *Command {
	command.output = output
	return &command
}

// IsActive reports whether the global command is active, i.e. Parse has been called.
func IsActive() bool {
	return command.IsActive()
}

// GetDescription returns the application description if set.
// See SetDescription.
func GetDescription() string {
	return command.GetDescription()
}

// GetFlags returns the registered FlagSet for the application.
func GetFlags() *flag.FlagSet {
	return command.flags
}

// Lookup searches for a registered subcommand by its name.
// If no matching command is found, nil is returned.
// Subsequent subcommands can be found by supplying multiple names.
// When no or an empty name is supplied, the global command itself is returned.
func Lookup(name ...string) *Command {
	return command.Lookup(name...)
}

// Active searches for a registered command by its name
// and reports its activation state. See IsActive.
func Active(name string) bool {
	return command.Active(name)
}

// GetActiveSubcommand returns the active subcommand of the global command
// or nil when no subcommand is active.
func GetActiveSubcommand() *Command {
	return command.GetActiveSubcommand()
}

// GetFinalActiveCommand returns the final active command in the global command chain.
// When no subcommand is active, the global command itself is returned.
func GetFinalActiveCommand() *Command {
	return command.GetFinalActiveCommand()
}

// GetActiveCommandChain returns a slice containing all active commands,
// where the first element is the global command itself.
func GetActiveCommandChain() []*Command {
	return command.GetActiveCommandChain()
}

// CommandUsagesWrapped returns a string containing the usage information
// for all subcommands defined for this command.
// Wrapped to cols columns (0 for no wrapping).
func CommandUsagesWrapped(cols int) string {
	return command.CommandUsagesWrapped(cols)
}

// CommandUsages returns a string containing the usage information
// for all commands defined for the application.
func CommandUsages() string {
	return command.CommandUsages()
}

// FlagUsages returns a string containing the usage information
// for all flags defined for the application.
func FlagUsages() string {
	return command.FlagUsages()
}

// CommandUsage returns a string containing the usage information
// for the application and all commands, including the
// application description if defined.
func CommandUsage() string {
	return command.CommandUsage()
}

// Parse parses the application command line arguments respecting the
// defined global command structure. Arguments for each command are parsed
// using pflag. The first argument is expected to be the application path.
// Use cflag.NewFlagSet to create the flag.FlagSet for parsing top-level application flags.
func Parse(arguments []string, flags *flag.FlagSet) error {
	command.flags = flags
	return command.Parse(arguments)
}

// Reset resets the global command register.
func Reset() {
	command = Command{}
}

// defaultUsage prints, to standard error unless configured
// otherwise, the default values of all defined flags in the set.
// This is the default function to print a usage message.
func defaultUsage(command *Command) {
	_, _ = fmt.Fprint(command.out(), command.CommandUsage())
}

func findLastFlagFromArgCC(cmdChain []*Command, arg string) (f *flag.Flag, remainder string) {
	if cmdChain == nil || arg == "" {
		return
	}

	if len(arg) >= 2 && arg[0] == '-' && arg[1] != '-' {
		// The argument is a shorthand flag.
		// Loop through all shorthands and find the last one that matches a registered flag.
		// When a non-bool flag is found, the remaining characters are returned as the value.
		// Otherwise, they are returned as the remainder.
	argLoop:
		//for iChar, argChar := range arg[1:] {
		for iChar := 1; iChar < len(arg); iChar++ {
			argChar := arg[iChar]
			cs := string(argChar)
			for iCmd := len(cmdChain) - 1; iCmd >= 0; iCmd-- {
				cmd := cmdChain[iCmd]
				if f2 := cmd.flags.ShorthandLookup(cs); f2 != nil {
					f = f2
					remainder = arg[iChar+1:]
					if f.Value.Type() == "bool" || f.Value.Type() == "boolSlice" {
						break
					} else {
						break argLoop
					}
				}
			}
		}
	} else if len(arg) >= 3 && arg[0] == '-' && arg[1] == '-' && arg[2] != '-' {
		// The argument is a longhand flag.
		arg = arg[2:]
		for iCmd := len(cmdChain) - 1; iCmd >= 0; iCmd-- {
			cmd := cmdChain[iCmd]
			f = cmd.flags.Lookup(arg)
		}
	}

	return
}

func findLastFlagFromArgC(command *Command, arg string) (f *flag.Flag, remainder string) {
	if command == nil || arg == "" {
		return
	}

	cmdChain := []*Command{command}
	f, remainder = findLastFlagFromArgCC(cmdChain, arg)
	return
}

// filterSlice filters out all elements where test returns false.
func filterSlice[T any](slice []T, test func(T) bool) []T {
	var res []T
	for _, s := range slice {
		if test(s) {
			res = append(res, s)
		}
	}
	return res
}

// getTermSize determines the dimensions of the active terminal.
func getTermSize() (width, height int, err error) {
	fd := int(os.Stdout.Fd())
	width, height, err = term.GetSize(fd)
	if err != nil {
		return 0, 0, err
	}
	return
}

/**
Wrap functions are copied from github.com/spf13/pflag.

Copyright (c) 2012 Alex Ogier. All rights reserved.
Copyright (c) 2012 The Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

// Splits the string `s` on whitespace into an initial substring up to
// `i` runes in length and the remainder. Will go `slop` over `i` if
// that encompasses the entire string (which allows the caller to
// avoid short orphan words on the final line).
func wrapN(i, slop int, s string) (string, string) {
	if i+slop > len(s) {
		return s, ""
	}

	w := strings.LastIndexAny(s[:i], " \t\n")
	if w <= 0 {
		return s, ""
	}
	nlPos := strings.LastIndex(s[:i], "\n")
	if nlPos > 0 && nlPos < w {
		return s[:nlPos], s[nlPos+1:]
	}
	return s[:w], s[w+1:]
}

// Wraps the string `s` to a maximum width `w` with leading indent
// `i`. The first line is not indented (this is assumed to be done by
// caller). Pass `w` == 0 to do no wrapping
func wrap(i, w int, s string) string {
	if w == 0 {
		return strings.Replace(s, "\n", "\n"+strings.Repeat(" ", i), -1)
	}

	// space between indent i and end of line width w into which
	// we should wrap the text.
	wrap := w - i

	var r, l string

	// Not enough space for sensible wrapping. Wrap as a block on
	// the next line instead.
	if wrap < 24 {
		i = 16
		wrap = w - i
		r += "\n" + strings.Repeat(" ", i)
	}
	// If still not enough space then don't even try to wrap.
	if wrap < 24 {
		return strings.Replace(s, "\n", r, -1)
	}

	// Try to avoid short orphan words on the final line, by
	// allowing wrapN to go a bit over if that would fit in the
	// remainder of the line.
	slop := 5
	wrap = wrap - slop

	// Handle first line, which is indented by the caller (or the
	// special case above)
	l, s = wrapN(wrap, slop, s)
	r = r + strings.Replace(l, "\n", "\n"+strings.Repeat(" ", i), -1)

	// Now wrap the rest
	for s != "" {
		var t string

		t, s = wrapN(wrap, slop, s)
		r = r + "\n" + strings.Repeat(" ", i) + strings.Replace(t, "\n", "\n"+strings.Repeat(" ", i), -1)
	}

	return r
}
