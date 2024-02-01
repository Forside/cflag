package cflag

import (
	"bytes"
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

func captureOutput(f func() error) (string, error) {
	// Create pipe to transfer output.
	outOrig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}

	// Redirect stdout and call function.
	os.Stdout = w
	err = f()
	os.Stdout = outOrig

	// Close pipe and read output.
	_ = w.Close()
	output, err := io.ReadAll(r)

	return string(output), err
}

func captureOutputC(f func() error) (string, error) {
	// Create pipe to transfer output.
	outOrig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}

	// Create channel to transfer pipe traffic.
	outC := make(chan string)
	// Create subroutine that listens on channel traffic.
	go func() {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			return
		}
		outC <- buf.String()
	}()

	// Redirect stdout and call function.
	os.Stdout = w
	err = f()
	os.Stdout = outOrig

	// Close pipe and read output from channel.
	_ = w.Close()
	output := <-outC

	return string(output), err
}

func buildTestContext() *testContext {
	ctx := new(testContext)
	ctx.arguments = slices.Clone(os.Args)

	// Define base flags.
	ctx.flags = flag.NewFlagSet("", flag.ExitOnError)
	ctx.flags.SortFlags = false
	ctx.flags.ParseErrorsWhitelist.UnknownFlags = true
	ctx.paramTest0 = ctx.flags.Int("test0", 0, "Test 0.")
	ctx.paramVersion = ctx.flags.BoolP("version", "v", false, "Display the application version.")

	// Define flags for command 'foo'.
	ctx.flagsFoo = flag.NewFlagSet("", flag.ExitOnError)
	ctx.flagsFoo.SortFlags = false
	ctx.flagsFoo.ParseErrorsWhitelist.UnknownFlags = true
	ctx.paramTest1 = ctx.flagsFoo.Int("test1", 1, "Test 1.")

	// Define flags for command 'foo/bar'
	ctx.flagsFooBar = flag.NewFlagSet("", flag.ExitOnError)
	ctx.flagsFooBar.SortFlags = false
	ctx.flagsFooBar.ParseErrorsWhitelist.UnknownFlags = true
	ctx.paramTest2 = ctx.flagsFooBar.Int("test2", 2, "Test 2.")

	// Define flags for command 'bar'.
	ctx.flagsWorld = flag.NewFlagSet("", flag.ExitOnError)
	ctx.flagsWorld.SortFlags = false
	ctx.flagsWorld.ParseErrorsWhitelist.UnknownFlags = true
	ctx.paramTest3 = ctx.flagsWorld.Int("test3", 3, "Test 3.")

	// Define flags for command 'types'.
	ctx.flagsTypes = flag.NewFlagSet("", flag.ExitOnError)
	ctx.flagsTypes.SortFlags = false
	ctx.flagsTypes.ParseErrorsWhitelist.UnknownFlags = true
	ctx.paramB = ctx.flagsTypes.BoolP("bool", "b", false, "Bool flag.")
	ctx.paramI = ctx.flagsTypes.IntP("int", "i", 0, "Int flag.")
	ctx.paramS = ctx.flagsTypes.StringP("str", "s", "", "String flag.")

	// Reset global cflag state and add commands.
	Reset()
	SetDescription("cflag test application.")
	ctx.cmdFoo = Cmd("foo", "Foo flags.", ctx.flagsFoo)
	ctx.cmdFooBar = ctx.cmdFoo.Cmd("bar", "Bar flags.", ctx.flagsFooBar)
	ctx.cmdWorld = Cmd("world", "World flags.", ctx.flagsWorld)
	ctx.cmdTypes = Cmd("types", "Types flags.", ctx.flagsTypes)

	return ctx
}

func TestParse(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Setup arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"--test0", "10", "foo", "--test1", "11", "bar", "--test2", "12", "--test3", "13"}...,
	)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)

	// Check version flag.
	if *ctx.paramVersion {
		fmt.Printf("%s\n", versionString)
		return
	}

	// Print flags.
	fmt.Printf("base: %t\n", IsActive())
	fmt.Printf("foo: %t\n", ctx.cmdFoo.IsActive())
	fmt.Printf("foo/bar: %t\n", ctx.cmdFooBar.IsActive())
	fmt.Printf("world: %t\n", ctx.cmdWorld.IsActive())

	fmt.Printf("Test 0: %t %d\n", ctx.flags.Changed("test0"), *ctx.paramTest0)
	fmt.Printf("Test 1: %t %d\n", ctx.flagsFoo.Changed("test1"), *ctx.paramTest1)
	fmt.Printf("Test 2: %t %d\n", ctx.flagsFooBar.Changed("test2"), *ctx.paramTest2)
	fmt.Printf("Test 3: %t %d\n", ctx.flagsWorld.Changed("test3"), *ctx.paramTest3)

	// Check flag values.
	a.Equal(true, IsActive())
	a.Equal(true, ctx.cmdFoo.IsActive())
	a.Equal(true, ctx.cmdFooBar.IsActive())
	a.Equal(false, ctx.cmdWorld.IsActive())
	a.Equal(10, *ctx.paramTest0)
	a.Equal(11, *ctx.paramTest1)
	a.Equal(12, *ctx.paramTest2)
	a.Equal(3, *ctx.paramTest3)
}

func TestTypes(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Setup arguments.
	ctx.arguments = append(ctx.arguments,
		[]string{"types", "-b", "-i", "1", "-s", "foobar"}...,
	)

	// Run cflag parser.
	Parse(ctx.arguments, ctx.flags)

	fmt.Printf("Bool flag: %t %t\n", ctx.flagsTypes.Changed("bool"), *ctx.paramB)
	fmt.Printf("Int flag: %t %d\n", ctx.flagsTypes.Changed("int"), *ctx.paramI)
	fmt.Printf("String flag: %t %s\n", ctx.flagsTypes.Changed("str"), *ctx.paramS)

	// Check parsed values.
	a.Equal(true, *ctx.paramB)
	a.Equal(1, *ctx.paramI)
	a.Equal("foobar", *ctx.paramS)
}

func TestHelp(t *testing.T) {
	a := assert.New(t)
	ctx := buildTestContext()

	// Setup arguments.
	ctx.arguments = slices.Insert(ctx.arguments, 1, "--help")

	// The test framework panics when os.Exit() is called.
	// Use recover to catch this after the help is printed.
	defer func() {
		if r := recover(); r != nil {
			a.Contains(r, "os.Exit(0)")
		}
	}()

	// Capture output from function.
	output, err := captureOutput(func() error {
		// Run cflag parser.
		Parse(ctx.arguments, ctx.flags)
		return nil
	})
	a.Nil(err)

	// Check output for help string.
	a.Contains(output, "cflag test application.")
}

func TestVersion(t *testing.T) {
	a := assert.New(t)

	// Setup arguments.
	argsOrig := slices.Clone(os.Args)
	os.Args = slices.Insert(os.Args, 1, "--version")

	// The test framework panics when os.Exit() is called.
	// Use recover to catch this after the version is printed.
	defer func() {
		if r := recover(); r != nil {
			a.Contains(r, "os.Exit(0)")
		}
	}()

	// Capture output from function.
	output, err := captureOutput(func() error {
		// Run TestParse which checks for --version flag.
		TestParse(t)
		return nil
	})
	a.Nil(err)
	os.Args = argsOrig

	// Check output for version string.
	println("Version:", output)
	a.EqualValues(versionString+"\n", output)
}

func TestStandalone(t *testing.T) {
	a := assert.New(t)

	// Setup arguments.
	args := slices.Clone(os.Args)
	args = append(args, "--test", "1")

	// Define flags.
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.SortFlags = false
	flags.ParseErrorsWhitelist.UnknownFlags = true
	paramTest := flags.Int("test", 0, "Test.")

	// Create command.
	cmd := NewCommand("test", "Test.", flags)

	// Run cflag parser.
	cmd.Parse(args[1:])

	fmt.Printf("Test: %t %d\n", flags.Changed("test"), *paramTest)

	// Check parsed value.
	a.Equal(1, *paramTest)
}
