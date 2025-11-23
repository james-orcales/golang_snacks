package cli

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"unsafe"

	"golang_snacks/invariant"
)

var (
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
	Stdin  io.Writer = os.Stdin
)

type Program struct {
	Label       string
	Description string
	Commands    []Command
}

type Command struct {
	Label       string
	Description string
	// Arguments are required and ordered. They must ALL appear before flags.
	Arguments []Option
	// Flags are optional and unordered.
	Flags []Option
}

type Option struct {
	Label       string
	Description string
	Value       any
	IsFlag      bool
}

func New(label, description string, commands ...Command) Program {
	panic_when(len(commands) == 0, "Program has zero commands specified")

	program := Program{
		Label:       label,
		Description: description,
		Commands:    commands,
	}

	for _, command := range program.Commands {
		panic_when(command.Label == "", "Program.Commands[%d].Label is unset", i)
		for i, arg := range command.Arguments {
			panic_when(arg.Label == "", "Argument #%d for command %q has no label", i, command.Label)
			switch arg.Value.(type) {
			default:
				panic_when(true, "Argument %q has unsupported type: %T", arg.Label, arg.Value)
			case string, int:
			}
		}
		for i, flag := range command.Flags {
			panic_when(flag.Label == "", "Flag #%d for command %q has no label", i, command.Label)
			panic_when(
				strings.Contains(flag.Label, "-"),
				"Flags cannot contain dashes. Instead of %q, use %q",
				flag.Label,
				strings.ReplaceAll(flag.Label, "-", "_"),
			)
			panic_when(
				strings.Contains(flag.Label, " "),
				"Flags cannot contain spaces: %q",
				flag.Label,
			)
			switch flag.Value.(type) {
			default:
				panic_when(true, "Flag %q has unsupported type: %T", flag.Label, flag.Value)
			case string, bool, int:
			}
		}
	}
	return program
}

func (program *Program) Parse(os_args []string) (active_command Command, err error) {
	panic_when(len(os_args) == 0, "program.Parse needs at least one os_arg")

	invariant.Sometimes(os_args[0] != program.Label, "Executable name at runtime is different from default program label")
	program.Label = os_args[0]

	defer func() {
		invariant.Sometimes(len(active_command.Arguments) == 0, "Command does not take arguments")
		invariant.Sometimes(len(active_command.Arguments) > 0, "Command takes arguments")
		invariant.Sometimes(len(active_command.Flags) == 0, "Command does not support flags")
		invariant.Sometimes(len(active_command.Flags) > 0, "Command supports flags")
	}()

	// === Finding active_command command ===
	active_command = program.Commands[0]
	if len(os_args) == 1 {
		invariant.Sometimes(true, "Defaulted to first declared command")
		return active_command, err
	}
	for i, command := range program.Commands {
		if command.Label == os_args[1] {
			active_command = command
			break
		} else if i == len(program.Commands)-1 {
			invariant.Sometimes(true, "User specified an unknown command")
			return active_command, fmt.Errorf("%q is an unknown command", os_args[1])
		}
	}

	// === Collecting ===
	positional_arguments := make([]string, 0, len(active_command.Arguments))
	flags := make([]string, 0, len(active_command.Flags))
	if len(os_args) > 2 {
		start_of_flags := -1
		for i, argument := range os_args[2:] {
			if strings.HasPrefix(argument, "-") && argument != "-" {
				start_of_flags = i + 2
				break
			}
			positional_arguments = append(positional_arguments, argument)
		}
		if start_of_flags >= 0 && start_of_flags < len(os_args) {
			for _, flag := range os_args[start_of_flags:] {
				if strings.HasPrefix(flag, "-") {
					flags = append(flags, flag)
				} else {
					invariant.Sometimes(true, "User provides flags before positional arguments")
					return active_command, fmt.Errorf("Positional arguments cannot appear after flags. Got %q", flag)
				}
			}
		}
	}
	invariant.Sometimes(len(flags) == 0, "No flags were set")
	invariant.Sometimes(len(flags) < len(active_command.Flags), "Some flags were set")
	invariant.Sometimes(len(flags) == len(active_command.Flags), "All flags were set")

	if len(positional_arguments) != len(active_command.Arguments) {
		invariant.Sometimes(true, "User provided inexact number of arguments")
		return active_command, fmt.Errorf(
			"%q expects %d arguments. Got %d",
			active_command.Label,
			len(active_command.Arguments),
			len(positional_arguments),
		)
	}
	if len(flags) > len(active_command.Flags) {
		invariant.Sometimes(true, "User provided too many flags")
		return active_command, fmt.Errorf(
			"%q supports %d flags at most. Got %d",
			active_command.Label,
			len(active_command.Flags),
			len(flags),
		)
	}

	// === Parsing ===
	for i, positional_argument := range positional_arguments {
		switch active_command.Arguments[i].Value.(type) {
		default:
			panic_when(true, "unreachable")
		case string:
			invariant.Sometimes(true, "User provided a string positional argument")
			active_command.Arguments[i].Value = positional_argument
		case int:
			invariant.Sometimes(true, "User provided an int positional argument")
			num, err := strconv.Atoi(positional_argument)
			if err != nil {
				return active_command, fmt.Errorf("%s is an invalid number", positional_argument)
			}
			active_command.Arguments[i].Value = num
		}
	}
	for _, flag := range flags {
		flag, value, value_was_set := strings.Cut(flag, "=")
		invariant.Always(flag != "-", "Lone dashes are treated as postional arguments")
		flag = flag[1:]
		i := slices.IndexFunc(active_command.Flags, func(option Option) bool {
			return option.Label == flag
		})
		if i < 0 {
			invariant.Sometimes(true, "User provided unknown flag")
			return active_command, fmt.Errorf("%q is an unknown flag", flag)
		} else if _, is_bool := active_command.Flags[i].Value.(bool); !is_bool && (!value_was_set || value == "") {
			invariant.Sometimes(true, "User did not set a value to a non-bool flag")
			return active_command, fmt.Errorf("%q expects a value. You must set flag values with this syntax: -foo_bar=baz.", flag)
		}
		switch active_command.Flags[i].Value.(type) {
		case bool:
			invariant.Sometimes(true, "User set a boolean flag")
			active_command.Flags[i].Value = true
		case string:
			invariant.Sometimes(true, "User set a string flag")
			active_command.Flags[i].Value = value
		case int:
			invariant.Sometimes(true, "User set an int flag")
			num, err := strconv.Atoi(value)
			if err != nil {
				return active_command, fmt.Errorf("%s is an invalid number", value)
			}
			active_command.Flags[i].Value = num
		}
	}

	return active_command, nil
}

func (program Program) PrintHelp() {
	// Use tabwriter to align columns (minwidth, tabwidth, padding, padchar, flags)
	w := tabwriter.NewWriter(Stdout, 0, 8, 0, ' ', 0)

	// Header
	fmt.Fprintf(w, "%s %s\n\n", program.Label, program.Description)
	fmt.Fprintf(w, "Usage:\n    %s <command> [arguments] [-flags[=value]]\n\n", program.Label)
	fmt.Fprintln(w, "Available Commands:")

	for _, cmd := range program.Commands {
		signature := cmd.Label + " "
		for _, arg := range cmd.Arguments {
			signature += fmt.Sprintf("<%s:%T> ", arg.Label, arg.Value)
		}
		fmt.Fprintf(w, "    %s\t%s\n\n", signature, cmd.Description)

		for _, flag := range cmd.Flags {
			valType := ""
			if _, isBool := flag.Value.(bool); !isBool {
				valType = fmt.Sprintf("=%T", flag.Value)
			}
			fmt.Fprintf(w, "        -%s%s\t  (default: %v)\t  %s\n", flag.Label, valType, flag.Value, flag.Description)
		}
		// Add a blank line between commands for readability
		fmt.Fprintln(w, "\t")
	}

	// Flush the tabwriter to ensure all output is written to Stdout
	w.Flush()
}

func GetOption(flags []Option, label string) Option {
	for _, flag := range flags {
		if flag.Label == label {
			return flag
		}
	}
	panic_when(true, "%q is an unknown option", label)
	return Option{}
}

func panic_when(cond bool, message string, data ...any) {
	if cond {
		panic(fmt.Sprintf(message, data...))
	}
}

func to_bytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
