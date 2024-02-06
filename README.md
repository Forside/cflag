# cflag

cflag adds command functionality to [pflag](https://github.com/spf13/pflag) by making use of pflag's FlagSet and its parsing capabilities. Commands can have subcommands, enabling deep command structures with independent flags. The API is leaned on flag.

Unlike pflag, cflag is not a drop-in replacement for flag or pflag.

## Installation

cflag is available using the standard `go get` command.

Install by running:
```
go get github.com/forside/cflag
```

Run tests by running:
```
go test github.com/forside/cflag
```

## Usage

### Imports

```go
import (
    flag "github.com/spf13/pflag"
    "github.com/forside/cflag"
)
```

### Parsing base application flags

To parse application arguments without commands, define a FlagSet, register flags and parse the arguments using cflag.

```go
flags := flag.NewFlagSet("", flag.ExitOnError)
paramVersion = flags.BoolP("version", "v", false, "Display the application version.")

cflag.Parse(os.Args, flags)
fmt.Printf("version flag: %t\n", *paramVersion)
```

### Parsing a command with flags

To add a command with a new set of flags, define a FlagSet, register a command with the set and parse the arguments using cflag. When no flags are required for the application or a command, supply nil instead of a FlagSet.

```go
flagsFoo := flag.NewFlagSet("", flag.ExitOnError)
paramFooTest1 := flagsFoo.Int("test1", 1, "Test 1.")
cmdFoo := cflag.Cmd("foo", flagsFoo)

cflag.Parse(os.Args, nil)
fmt.Printf("foo command supplied: %t\n", cmdFoo.IsActive())
fmt.Printf("test1 flag: %d\n", *paramFooTest1)
```

### Parsing subcommands

To add a subcommand to another command, register it to the command instead of to cflag directly.

```go
cmdFoo := cflag.Cmd("foo", nil)
flagsFooBar := flag.NewFlagSet("", flag.ExitOnError)
paramFooBarTest2 := flagsFoo.Int("test2", 2, "Test 2.")
cmdFooBar := cmdFoo.Cmd("bar", flagsFooBar)

cflag.Parse(os.Args, nil)
fmt.Printf("foo command supplied: %t\n", cmdFoo.IsActive())
fmt.Printf("foo/bar command supplied: %t\n", cmdFooBar.IsActive())
fmt.Printf("test2 flag: %d\n", *paramFooBarTest2)
```

### Full example

```go
package main

import (
    "fmt"
    "github.com/forside/cflag"
    flag "github.com/spf13/pflag"
    "os"
)

func main() {
    // Define top-level flags.
    flags := cflag.NewFlagSet("", flag.ExitOnError)
    flags.SortFlags = false
    paramVersion := flags.BoolP("version", "v", false, "Display the application version.")

    // Define foo command.
    flagsFoo := cflag.NewFlagSet("", flag.ExitOnError)
    flagsFoo.SortFlags = false
    paramFooTest1 := flagsFoo.Int("test1", 1, "Test 1.")
    cmdFoo, _ := cflag.Cmd("foo", "Foo command.", flagsFoo)

    // Define foo/bar command.
    flagsFooBar := cflag.NewFlagSet("", flag.ExitOnError)
    paramFooBarTest2 := flagsFooBar.Int("test2", 2, "Test 2.")
    flagsFooBar.SortFlags = false
    cmdFooBar, _ := cmdFoo.Cmd("bar", "Bar command", flagsFooBar)

    // Parse arguments and print values.
    cflag.Parse(os.Args, flags)
    fmt.Printf("version flag: %t\n", *paramVersion)
    fmt.Printf("foo command supplied: %t\n", cmdFoo.IsActive())
    fmt.Printf("foo/bar command supplied: %t\n", cmdFooBar.IsActive())
    fmt.Printf("test1 flag: %d\n", *paramFooTest1)
    fmt.Printf("test2 flag: %d\n", *paramFooBarTest2)
}
```

```shellsession
$ go build
$ ./main -v foo --test1 11 bar --test2 12
version flag: true
foo command supplied: true
foo/bar command supplied: true
test1 flag: 11
test2 flag: 12
```

For more examples check [cflag_test.go](./cflag_test.go).

### Using callbacks

To automatically execute a function for an active command, callbacks can be defined. The last active command in a command chain and its FlagSet is passed to the callback. When a callback is defined for a parent command but not for its subcommands, the callback of the parent command is executed and the subcommand and its FlagSet is passed to it.

```go
cb := func(command *cflag.Command, flags *flag.FlagSet) {
    // Print flag.
    paramTest, _ := flags.GetInt("test")
    fmt.Printf("Test: %t %d\n", flags.Changed("test"), paramTest)
}

// Define flags.
flags := clag.NewFlagSet("", flag.ExitOnError)
flags.SortFlags = false
_ = flags.Int("test", 0, "Test.")

// Set global command callback.
cflag.SetCallback(cb)

// Run cflag parser.
cflag.Parse(args, flags)
```

See `TestCallback` in [cflag_test.go](./cflag_test.go).

### Using cflag without global values

cflag can be used standalone without using global values. While parsing the arguments, a command expects its name to be either empty or equal `args[0]`. This means the name of the top-level command must be either empty or `args[0]`. 

```go
// Define top-level flags.
flags := NewFlagSet("", flag.ExitOnError)
flags.SortFlags = false
paramTest := flags.Int("test", 0, "Test.")

// Create top-level command with empty name.
cmd := NewCommand("", "Test.", flags)

// Parse arguments and print values.
cmd.Parse(os.Args)
fmt.Printf("Test: %t %d\n", flags.Changed("test"), *paramTest)
```

See `TestStandalone` in [cflag_test.go](./cflag_test.go).

### Help page

cflag automatically generates help pages for all commands. It can be accessed by supplying `-h, --help` to a command. To add a description to your command, use `SetDescription()`.

```go
// Define top-level flags.
flags := cflag.NewFlagSet("", flag.ExitOnError)
flags.SortFlags = false
paramVersion := flags.BoolP("version", "v", false, "Display the application version.")

// Define foo command.
flagsFoo := cflag.NewFlagSet("", flag.ExitOnError)
flagsFoo.SortFlags = false
paramFooTest1 := flagsFoo.Int("test1", 1, "Test 1.")
cmdFoo, _ := cflag.Cmd("foo", "Foo command.", flagsFoo)

// Parse arguments.
cflag.SetDescription("cflag test application.")
cflag.Parse(os.Args, flags)
```

```shellsession
$ ./main -h
cflag test application.
Commands:
  foo   Foo command.
Flags:
  -v, --version   Display the application version.
  -h, --help      Display help.
```

```shellsession
$ ./main foo -h
Foo command.
Flags:
      --test1 int   Test 1. (default 1)
  -h, --help        Display help.
```

See `TestHelp`, `TestHidden` and `TestDeprecated` in [cflag_test.go](./cflag_test.go) for more options.

## Development

Clone the repository and run `go build` to build the module or `go test` to run the integrated tests.
