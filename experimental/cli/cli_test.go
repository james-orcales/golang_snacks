package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/james-orcales/golang_snacks/cli"
	"github.com/james-orcales/golang_snacks/invariant"
	"github.com/james-orcales/golang_snacks/snap"
)

func TestMain(m *testing.M) {
	database.Grow(1024 * 256)
	cli.Stdout = &bytes.Buffer{}
	cli.Stderr = &bytes.Buffer{}
	defer cli.Stdout.(*bytes.Buffer).Reset()
	defer cli.Stderr.(*bytes.Buffer).Reset()

	invariant.RegisterPackagesForAnalysis()
	initGlobal()
	code := m.Run()
	invariant.AnalyzeAssertionFrequency()
	os.Exit(code)
}

// initializing program AFTER registering packages so that cli.New assertions are registered
func initGlobal() {
	program = cli.New(
		"todoctl",
		"is a todo list manager",
		cli.Command{Label: "help", Description: "print help message"},
		cli.Command{
			Label: "add",
			Arguments: []cli.Option{
				{Label: "task", Value: "", Description: "describes what you need to do"},
			},
			Flags: []cli.Option{
				{Label: "deadline", Value: "Jan 02 Mon", Description: "deadline date"},
				{Label: "priority", Value: "low", Description: "set priority"},
				{Label: "noop", Value: false, Description: "random flag that does nothing"},
			},
		},
		cli.Command{
			Label: "list",
			Flags: []cli.Option{
				{Label: "columns", Value: "all", Description: "comma-separated (all, deadline, priority, description)"},
				{Label: "count", Value: 0, Description: "how many tasks to display"},
			},
		},
		cli.Command{
			Label:       "delete",
			Description: "remove a task by ID",
			Arguments: []cli.Option{
				// Setting Value to 0 ensures the parser treats this as an int
				{Label: "id", Value: 0, Description: "the integer index of the task to remove"},
			},
		},
	)
}

var (
	database = &bytes.Buffer{}
	program  cli.Program
	// Usually, you'd inline this in func main(). Here, we extracted it out into separate
	// function since each test is a separate entrypoint.
	command_handler = func(command cli.Command) {
		switch command.Label {
		case "help":
			program.PrintHelp()
		case "add":
			database.WriteString(fmt.Sprintf(
				"%s | %s | %s\n",
				cli.GetOption(command.Flags, "deadline").Value,
				cli.GetOption(command.Flags, "priority").Value,
				cli.GetOption(command.Arguments, "task").Value,
			))
		case "delete":
			id := cli.GetOption(command.Arguments, "id").Value.(int)
			content := strings.TrimSpace(database.String())
			if content == "" {
				fmt.Fprintln(cli.Stdout, "List is already empty")
				return
			}

			lines := strings.Split(content, "\n")
			if id < 0 || id >= len(lines) {
				fmt.Fprintf(cli.Stdout, "Error: ID %d is out of range (0 to %d)\n", id, len(lines)-1)
				return
			}

			database.Reset()
			for i, line := range lines {
				if i == id {
					continue
				}
				database.WriteString(line + "\n")
			}

			fmt.Fprintf(cli.Stdout, "Deleted task %d\n", id)
		case "list":
			columns := cli.GetOption(command.Flags, "columns").Value.(string)
			count := cli.GetOption(command.Flags, "count").Value.(int)
			all, deadline, priority, description := false, false, false, false
			for column := range strings.SplitSeq(columns, ",") {
				if column == "all" {
					all = true
					break
				}
				if column == "deadline" {
					deadline = true
				}
				if column == "priority" {
					priority = true
				}
				if column == "description" {
					description = true
				}
			}
			if all {
				fmt.Fprintln(cli.Stdout, database.String())
			} else {
				iteration := 0
				for line := range strings.Lines(database.String()) {
					if iteration >= count {
						break
					}
					cols := strings.Split(line, " | ")
					out := []string{}
					if deadline {
						out = append(out, cols[0])
					}
					if priority {
						out = append(out, cols[1])
					}
					if description {
						out = append(out, string(cols[2][:len(cols[2])-1]))
					}

					fmt.Fprintln(cli.Stdout, strings.Join(out, "::"))
				}

			}
		}
	}
)

func check(t *testing.T, out, err *bytes.Buffer, snapshot snap.Snapshot) {
	t.Helper()
	defer out.Reset()
	defer err.Reset()
	defer database.Reset()
	expect := fmt.Sprintf("Stdout:\n%s\nStderr:\n%s\nDatabase:\n%s\n", out.String(), err.String(), database.String())
	if !snapshot.IsEqual(expect) {
		t.Fatal("Snapshot mismatch")
	}
}

func TestHelpMessage(t *testing.T) {
	program.PrintHelp()
	check(t, cli.Stdout.(*bytes.Buffer), cli.Stderr.(*bytes.Buffer), snap.Init(`Stdout:
todoctl is a todo list manager

Usage:
    todoctl <command> [arguments] [-flags[=value]]

Available Commands:
    help print help message

                      
    add <task:string> 

        -deadline=string  (default: Jan 02 Mon)  deadline date
        -priority=string  (default: low)         set priority
        -noop             (default: false)       random flag that does nothing
                        
    list                

        -columns=string  (default: all)  comma-separated (all, deadline, priority, description)
        -count=int       (default: 0)    how many tasks to display
                       
    delete <id:int>    remove a task by ID



Stderr:

Database:

`))
}

func TestDemo(t *testing.T) {
	commands := [][]string{
		{"todoctl", "add", "commit to github", "-deadline=Nov 21 Fri"},
		{"todoctl", "add", "something important", "-noop"},
		{"todoctl", "add", "foo bar baz"},
		{"todoctl", "list", "-count=2"},
		{"todoctl", "delete", "1"},
		{"todoctl", "list", "-columns=priority,description"},
	}
	for _, command := range commands {
		command, err := program.Parse(command)
		if err != nil {
			panic(err)
		}
		command_handler(command)
	}
	check(t, cli.Stdout.(*bytes.Buffer), cli.Stderr.(*bytes.Buffer), snap.Init(`Stdout:
Nov 21 Fri | low | commit to github
Nov 21 Fri | low | something important
Nov 21 Fri | low | foo bar baz

Deleted task 1
low::commit to github
low::foo bar baz

Stderr:

Database:
Nov 21 Fri | low | commit to github
Nov 21 Fri | low | foo bar baz

`))
}

func TestUserError(t *testing.T) {
	commands := [][]string{
		{"different_label"},
		{"todoctl", "arsotitnaroisen"},
		{"todoctl", "add"},
		{"todoctl", "add", "without flags"},
		{"todoctl", "add", "missing flag value", "-deadline="},
		{"todoctl", "add", "another missing flag value", "-deadline"},
		{"todoctl", "add", "unknown flag", "-unknown"},
		{"todoctl", "add", "-out_of_place", "commit to github"},
		{"todoctl", "list", "-columns=priority,description", "-count=-1"},
		{"todoctl", "list", "-count=0", "-count=-1", "-count=-2"},
	}
	for _, command := range commands {
		command, err := program.Parse(command)
		if err != nil {
			fmt.Fprintln(cli.Stderr, err.Error())
			continue
		}
		command_handler(command)
	}
	check(t, cli.Stdout.(*bytes.Buffer), cli.Stderr.(*bytes.Buffer), snap.Init(`Stdout:
different_label is a todo list manager

Usage:
    different_label <command> [arguments] [-flags[=value]]

Available Commands:
    help print help message

                      
    add <task:string> 

        -deadline=string  (default: Nov 21 Fri)  deadline date
        -priority=string  (default: low)         set priority
        -noop             (default: true)        random flag that does nothing
                        
    list                

        -columns=string  (default: priority,description)  comma-separated (all, deadline, priority, description)
        -count=int       (default: 2)                     how many tasks to display
                       
    delete <id:int>    remove a task by ID



Stderr:
"arsotitnaroisen" is an unknown command
"add" expects 1 arguments. Got 0
"deadline" expects a value. You must set flag values with this syntax: -foo_bar=baz.
"deadline" expects a value. You must set flag values with this syntax: -foo_bar=baz.
"unknown" is an unknown flag
Positional arguments cannot appear after flags. Got "commit to github"
"list" supports 2 flags at most. Got 3

Database:
Nov 21 Fri | low | without flags

`))
}
