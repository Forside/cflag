package cflag

import (
	"fmt"
	"io"
	"os"
	"slices"
	"testing"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

const VERSION_MAJOR = 0
const VERSION_MINOR = 1
const VERSION_PATCH = 0

var versionString = fmt.Sprintf("%d.%d.%d", VERSION_MAJOR, VERSION_MINOR, VERSION_PATCH)

type testContext struct {
	arguments []string

	flags, flagsFoo, flagsFooBar, flagsWorld       *flag.FlagSet
	cmdFoo, cmdFooBar, cmdWorld                    *Command
	paramTest0, paramTest1, paramTest2, paramTest3 *int
	paramVersion                                   *bool

	flagsTypes *flag.FlagSet
	cmdTypes   *Command
	paramB     *bool
	paramI     *int
	paramS     *string
}

type outputCaptureContext struct {
	outOrig, errOrig *os.File
	r, w             *os.File
}

// startCaptureOutput redirects stdout and stderr to capture any output written afterwards.
// Returns a capture context or nil and an error.
func startCaptureOutput(includeStdout, includeStderr bool) (*outputCaptureContext, error) {
	ctx := new(outputCaptureContext)
	var err error

	// Create pipe to transfer output.
	ctx.outOrig = os.Stdout
	ctx.errOrig = os.Stderr
	ctx.r, ctx.w, err = os.Pipe()
	if err != nil {
		return nil, err
	}

	// Redirect stdout and call function.
	if includeStdout {
		os.Stdout = ctx.w
	}
	if includeStderr {
		os.Stderr = ctx.w
	}

	return ctx, nil
}

// stopCaptureOutput captures the output written to stdout and stderr since
// startCaptureOutput was called and routes stdout and stderr back to their original destination.
// Returns the captured output or an empty string and an error.
func (ctx *outputCaptureContext) stopCaptureOutput() (string, error) {
	if ctx.outOrig == nil || ctx.errOrig == nil || ctx.r == nil || ctx.w == nil {
		return "", fmt.Errorf("uninitialised capture context")
	}

	os.Stdout = ctx.outOrig
	os.Stderr = ctx.errOrig

	// Close pipe and read output.
	_ = ctx.w.Close()
	output, err := io.ReadAll(ctx.r)

	return string(output), err
}

// captureOutput runs f and captures any output to stdout and stderr.
// Returns the output and the error returned by f.
func captureOutput(includeStdout, includeStderr bool, f func() error) (string, error) {
	// Create pipe to transfer output.
	outOrig := os.Stdout
	errOrig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}

	// Redirect stdout and call function.
	if includeStdout {
		os.Stdout = w
	}
	if includeStderr {
		os.Stderr = w
	}
	err = f()
	os.Stdout = outOrig
	os.Stderr = errOrig

	// Close pipe and read output.
	_ = w.Close()
	output, _ := io.ReadAll(r)

	return string(output), err
}

// buildTestContext defines some flags and commands used by test functions.
func buildTestContext() *testContext {
	ctx := new(testContext)
	ctx.arguments = slices.Clone(os.Args)

	// Define base flags.
	ctx.flags = NewFlagSet("", flag.ExitOnError)
	ctx.flags.SortFlags = false
	ctx.paramTest0 = ctx.flags.Int("test0", 0, "Test 0.")
	ctx.paramVersion = ctx.flags.BoolP("version", "v", false, "Display the application version.")

	// Define flags for command 'foo'.
	ctx.flagsFoo = NewFlagSet("", flag.ExitOnError)
	ctx.flagsFoo.SortFlags = false
	ctx.paramTest1 = ctx.flagsFoo.Int("test1", 1, "Test 1.")

	// Define flags for command 'foo/bar'
	ctx.flagsFooBar = NewFlagSet("", flag.ExitOnError)
	ctx.flagsFooBar.SortFlags = false
	ctx.paramTest2 = ctx.flagsFooBar.Int("test2", 2, "Test 2.")

	// Define flags for command 'bar'.
	ctx.flagsWorld = NewFlagSet("", flag.ExitOnError)
	ctx.flagsWorld.SortFlags = false
	ctx.paramTest3 = ctx.flagsWorld.Int("test3", 3, "Test 3.")

	// Define flags for command 'types'.
	ctx.flagsTypes = NewFlagSet("", flag.ExitOnError)
	ctx.flagsTypes.SortFlags = false
	ctx.paramB = ctx.flagsTypes.BoolP("bool", "b", false, "Bool flag.")
	ctx.paramI = ctx.flagsTypes.IntP("int", "i", 0, "Int flag.")
	ctx.paramS = ctx.flagsTypes.StringP("str", "s", "", "String flag.")

	// Reset global cflag state and add commands.
	Reset()
	SetDescription("cflag test application.")
	ctx.cmdFoo, _ = Cmd("foo", "Foo command.", ctx.flagsFoo)
	ctx.cmdFoo.SetDescription("Foo command description.")
	ctx.cmdFooBar, _ = ctx.cmdFoo.Cmd("bar", "Bar command.", ctx.flagsFooBar)
	ctx.cmdWorld, _ = Cmd("world", "World command.", ctx.flagsWorld)
	ctx.cmdTypes, _ = Cmd("types", "Types command.", ctx.flagsTypes)

	return ctx
}

func TestParse(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Setup test arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"--test0", "10", "foo", "--test1", "11", "bar", "--test2", "12", "--test3", "13"}...,
	)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)

	// Check version flag.
	if *ctx.paramVersion {
		t.Logf("%s\n", versionString)
		// Print version so TestVersion can catch it.
		fmt.Printf("%s\n", versionString)
		return
	}

	// Print flags.
	t.Logf("base: %t\n", IsActive())
	t.Logf("foo: %t\n", ctx.cmdFoo.IsActive())
	t.Logf("foo/bar: %t\n", ctx.cmdFooBar.IsActive())
	t.Logf("world: %t\n", ctx.cmdWorld.IsActive())

	t.Logf("Test 0: %t %d\n", ctx.flags.Changed("test0"), *ctx.paramTest0)
	t.Logf("Test 1: %t %d\n", ctx.flagsFoo.Changed("test1"), *ctx.paramTest1)
	t.Logf("Test 2: %t %d\n", ctx.flagsFooBar.Changed("test2"), *ctx.paramTest2)
	t.Logf("Test 3: %t %d\n", ctx.flagsWorld.Changed("test3"), *ctx.paramTest3)

	// Check flag values.
	a.True(IsActive())
	a.True(ctx.cmdFoo.IsActive())
	a.True(ctx.cmdFooBar.IsActive())
	a.False(ctx.cmdWorld.IsActive())
	a.Equal(10, *ctx.paramTest0)
	a.Equal(11, *ctx.paramTest1)
	a.Equal(12, *ctx.paramTest2)
	a.Equal(3, *ctx.paramTest3)
}

func TestTypes(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Setup test arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"types", "-b", "-i", "1", "-s", "foobar"}...,
	)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)

	// Print flags.
	t.Logf("Bool flag: %t %t\n", ctx.flagsTypes.Changed("bool"), *ctx.paramB)
	t.Logf("Int flag: %t %d\n", ctx.flagsTypes.Changed("int"), *ctx.paramI)
	t.Logf("String flag: %t %s\n", ctx.flagsTypes.Changed("str"), *ctx.paramS)

	// Check parsed values.
	a.True(*ctx.paramB)
	a.Equal(1, *ctx.paramI)
	a.Equal("foobar", *ctx.paramS)
}

func TestVersion(t *testing.T) {
	a := assert.New(t)

	// Setup test arguments.
	argsOrig := slices.Clone(os.Args)
	os.Args = slices.Insert(os.Args, 1, "--version")

	// Capture output from function.
	output, err := captureOutput(true, true, func() error {
		// Run TestParse which checks for --version flag.
		TestParse(t)
		return nil
	})
	a.Nil(err)
	os.Args = argsOrig

	// Check output for version string.
	t.Logf("Version: %s\n", output)
	a.Equal(versionString+"\n", output)
}

func TestMisc(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Setup test arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"--test0", "10", "foo", "--test1", "11", "bar", "--test2", "12", "--test3", "13"}...,
	)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)

	// Find commands.
	cmdFoo := Lookup("foo")
	cmdWorld := Lookup("world")
	cmdOther := Lookup("other")

	// Print command states.
	t.Logf("Foo defined:%t active:%t\n", cmdFoo != nil, cmdFoo != nil && cmdFoo.IsActive())
	t.Logf("World defined:%t active:%t\n", cmdWorld != nil, cmdWorld != nil && cmdWorld.IsActive())
	t.Logf("Other defined:%t active:%t\n", cmdOther != nil, cmdOther != nil && cmdOther.IsActive())

	// Check commands.
	a.NotNil(cmdFoo)
	a.NotNil(cmdWorld)
	a.Nil(cmdOther)

	// Find active commands.
	fooActive := Active("foo")
	worldActive := Active("world")
	otherActive := Active("other")

	// Print command activation.
	t.Logf("Foo active: %t\n", fooActive)
	t.Logf("World active: %t\n", worldActive)
	t.Logf("Other active: %t\n", otherActive)

	// Check command activation.
	a.True(fooActive)
	a.False(worldActive)
	a.False(otherActive)
}

func TestHelp(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Setup test arguments.
	ctx.arguments = slices.Insert(ctx.arguments, 1, "--help")

	var capCtx *outputCaptureContext

	// The test framework panics when os.Exit() is called.
	// Use recover to catch this after the help is printed.
	defer func() {
		if r := recover(); r != nil {
			a.Contains(r, "os.Exit(0)")

			// Receive captured output.
			a.NotNil(capCtx)
			output, err := capCtx.stopCaptureOutput()
			a.NoError(err)
			t.Log(output)

			// Check output for help string.
			a.Contains(output, "cflag test application.")
		}
	}()

	// Capture output to stdout and stderr.
	capCtx, err := startCaptureOutput(true, true)
	a.NoError(err)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)
}

func TestCommandHelp(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Setup test arguments.
	ctx.arguments = slices.Insert(ctx.arguments, 1, "foo", "--help")

	var capCtx *outputCaptureContext

	// Define optional custom Usage function which is called automatically
	// during parsing when -h, --help is found.
	// The logic here roughly equals the default help message.
	SetUsageFunc(func(c *Command) {
		fmt.Printf("%s\n%s\nCommands:\n%sFlags:\n%s", c.GetUsage(), c.GetDescription(), c.CommandUsages(), c.FlagUsages())
	})

	// The test framework panics when os.Exit() is called.
	// Use recover to catch this after the help is printed.
	defer func() {
		if r := recover(); r != nil {
			a.Contains(r, "os.Exit(0)")

			// Receive captured output.
			a.NotNil(capCtx)
			output, err := capCtx.stopCaptureOutput()
			a.NoError(err)
			t.Log(output)

			// Check output for help string.
			a.Contains(output, "Foo command.")
		}
	}()

	// Capture output to stdout and stderr.
	capCtx, err := startCaptureOutput(true, true)
	a.NoError(err)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)
}

func TestHidden(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Mark the 'world' command as hidden.
	ctx.cmdWorld.MarkHidden()

	// Setup test arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"-h"}...,
	)

	var capCtx *outputCaptureContext

	// The test framework panics when os.Exit() is called.
	// Use recover to catch this after the help is printed.
	defer func() {
		if r := recover(); r != nil {
			a.Contains(r, "os.Exit(0)")

			// Receive captured output.
			a.NotNil(capCtx)
			output, err := capCtx.stopCaptureOutput()
			a.NoError(err)
			t.Log(output)

			// Check output for help string.
			a.NotContains(output, "world")
		}
	}()

	// Capture output to stdout and stderr.
	capCtx, err := startCaptureOutput(true, true)
	a.NoError(err)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)
}

func TestDeprecated(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Mark the command as deprecated.
	ctx.cmdWorld.MarkDeprecated()

	// Setup test arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"world", "-h"}...,
	)

	var capCtx *outputCaptureContext

	// The test framework panics when os.Exit() is called.
	// Use recover to catch this after the help is printed.
	defer func() {
		if r := recover(); r != nil {
			a.Contains(r, "os.Exit(0)")

			// Receive captured output.
			a.NotNil(capCtx)
			output, err := capCtx.stopCaptureOutput()
			a.NoError(err)
			t.Log(output)

			// Check output for help string.
			a.Contains(output, "DEPRECATED")
		}
	}()

	// Capture output to stdout and stderr.
	capCtx, err := startCaptureOutput(true, true)
	a.NoError(err)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)
}

func TestDeprecatedUse(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Mark the world command as deprecated.
	ctx.cmdWorld.MarkDeprecated()

	// Setup test arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"world", "--test3", "3"}...,
	)

	// Capture output from function.
	output, err := captureOutput(true, true, func() error {
		// Run cflag parser.
		Parse(ctx.arguments, ctx.flags)
		return nil
	})
	a.Nil(err)
	a.NotEmpty(output)

	// Print flags.
	t.Logf("world: %t\n", ctx.cmdWorld.IsActive())
	t.Logf("Test 3: %t %d\n", ctx.flagsWorld.Changed("test3"), *ctx.paramTest3)

	// Check flag values.
	a.True(ctx.cmdWorld.IsActive())
	a.Equal(3, *ctx.paramTest3)
	a.Contains(output, "deprecated")

	t.Log(output)
}

func TestParentArgs(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Parse arguments passed to foo/bar recursively using its parent commands (base and foo).
	ctx.cmdFooBar.SetRecurseArguments()

	// Setup test arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"foo", "bar", "--test0", "10", "--test1", "11", "--test2", "12"}...,
	)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)

	// Print flags.
	t.Logf("base: %t\n", IsActive())
	t.Logf("foo: %t\n", ctx.cmdFoo.IsActive())
	t.Logf("foo/bar: %t\n", ctx.cmdFooBar.IsActive())

	t.Logf("Test 0: %t %d\n", ctx.flags.Changed("test0"), *ctx.paramTest0)
	t.Logf("Test 1: %t %d\n", ctx.flagsFoo.Changed("test1"), *ctx.paramTest1)
	t.Logf("Test 2: %t %d\n", ctx.flagsFooBar.Changed("test2"), *ctx.paramTest2)

	// Check flag values.
	a.True(IsActive())
	a.True(ctx.cmdFoo.IsActive())
	a.True(ctx.cmdFooBar.IsActive())
	a.Equal(10, *ctx.paramTest0)
	a.Equal(11, *ctx.paramTest1)
	a.Equal(12, *ctx.paramTest2)
}

func TestRedirectOutput(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Setup test arguments.
	ctx.arguments = slices.Insert(ctx.arguments, 1, "--help")

	var capCtx *outputCaptureContext

	// The test framework panics when os.Exit() is called.
	// Use recover to catch this after the help is printed.
	defer func() {
		if r := recover(); r != nil {
			a.Contains(r, "os.Exit(0)")

			// Receive captured output.
			a.NotNil(capCtx)
			output, err := capCtx.stopCaptureOutput()
			a.NoError(err)
			t.Log(output)

			// Check output for help string.
			a.Contains(output, "cflag test application.")
		}
	}()

	// Capture output to stdout only.
	capCtx, err := startCaptureOutput(true, false)
	a.NoError(err)

	// cflag by default prints to stderr.
	// Set stdout as the output to test redirection.
	SetOutput(os.Stdout)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)
}

func TestCallback(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	cb := func(command *Command, flags *flag.FlagSet) {
		// Print flag.
		paramTest, _ := flags.GetInt("test0")
		t.Logf("Test 0: %t %d\n", flags.Changed("test0"), paramTest)

		// Check parsed value.
		a.Equal(10, paramTest)
	}

	// Setup test arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"--test0", "10"}...,
	)

	// Set global command callback.
	SetCallback(cb)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)
}

func TestCallback2(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	cbFoo := func(command *Command, flags *flag.FlagSet) {
		// Print flags.
		t.Logf("Test 0: %t %d\n", ctx.flags.Changed("test0"), *ctx.paramTest0)
		t.Logf("Test 1: %t %d\n", ctx.flagsFoo.Changed("test1"), *ctx.paramTest1)

		// Check parsed values.
		a.Equal(10, *ctx.paramTest0)
		a.Equal(11, *ctx.paramTest1)
	}

	// Setup test arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"foo", "--test0", "10", "--test1", "11"}...,
	)

	// Set global command callback.
	ctx.cmdFoo.SetCallback(cbFoo).SetRecurseArguments()

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)
}

func TestStandalone(t *testing.T) {
	a := assert.New(t)

	// Setup test arguments.
	args := slices.Clone(os.Args)
	args = append(args, "--test", "1")

	// Define flags.
	flags := NewFlagSet("", flag.ExitOnError)
	flags.SortFlags = false
	paramTest := flags.Int("test", 0, "Test.")

	// Create top-level command with empty name.
	cmd := NewCommand("", "Test.", flags)

	// Run cflag parser.
	cmd.Parse(args)

	// Print flag.
	t.Logf("Test: %t %d\n", flags.Changed("test"), *paramTest)

	// Check parsed value.
	a.Equal(1, *paramTest)
}

func TestExample(t *testing.T) {
	a := assert.New(t)

	// Reset global cflag register.
	Reset()

	// Setup test arguments.
	args := slices.Clone(os.Args)
	args = append(args, "-v", "foo", "--test1", "11", "bar", "--test2", "12")

	// Define top-level flags.
	flags := NewFlagSet("", flag.ExitOnError)
	flags.SortFlags = false
	paramVersion := flags.BoolP("version", "v", false, "Display the application version.")

	// Define foo command.
	flagsFoo := NewFlagSet("", flag.ExitOnError)
	flagsFoo.SortFlags = false
	paramFooTest1 := flagsFoo.Int("test1", 1, "Test 1.")
	cmdFoo, _ := Cmd("foo", "Foo command.", flagsFoo)

	// Define foo/bar command.
	flagsFooBar := NewFlagSet("", flag.ExitOnError)
	paramFooBarTest2 := flagsFooBar.Int("test2", 2, "Test 2.")
	flagsFooBar.SortFlags = false
	cmdFooBar, _ := cmdFoo.Cmd("bar", "Bar command", flagsFooBar)

	// Parse arguments and print values.
	Parse(args, flags)
	t.Logf("version flag: %t\n", *paramVersion)
	t.Logf("foo command supplied: %t\n", cmdFoo.IsActive())
	t.Logf("foo/bar command supplied: %t\n", cmdFooBar.IsActive())
	t.Logf("test1 flag: %d\n", *paramFooTest1)
	t.Logf("test2 flag: %d\n", *paramFooBarTest2)

	// Check values.
	a.True(cmdFoo.IsActive())
	a.True(cmdFooBar.IsActive())
	a.True(*paramVersion)
	a.Equal(11, *paramFooTest1)
	a.Equal(12, *paramFooBarTest2)
}
