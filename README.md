# cflag

cflag adds command functionality to [pflag](https://github.com/spf13/pflag) by making use of pflag's FlagSet and its parsing capabilities. Commands can have subcommands, enabling deep command structures with independent flags.

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

### Defining a command

## Development

Clone the repository and run `go build` to build the module or `go test` to run the integrated tests.
