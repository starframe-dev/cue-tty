package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Run starts the REPL with the given session name.
func Run(sessionName string) error {
	if !validSessionName(sessionName) {
		return fmt.Errorf("invalid session name %q", sessionName)
	}
	sm := NewSessionManager()
	session := sm.GetOrCreate(sessionName)
	commands := Commands(sm)

	// Check if there are inline commands (non-interactive mode)
	args := os.Args[1:]
	// Filter out the -s flag and its value
	filtered := make([]string, 0, len(args))
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "-s" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(arg, "-s=") {
			continue
		}
		filtered = append(filtered, arg)
	}

	// If there are positional args, run them as a command
	if len(filtered) > 0 {
		result, err := runCommand(session, commands, strings.Join(filtered, " "))
		if err != nil {
			return err
		}
		fmt.Println(result)
		return nil
	}

	// Interactive REPL mode
	fmt.Printf("cue-tty> session %q ready. Type 'help' for commands.\n", sessionName)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("cue-tty> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		result, err := runCommand(session, commands, line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		command := parseLine(line)
		if len(command) > 0 && (command[0] == "exit" || command[0] == "quit") {
			if result != "" {
				fmt.Println(result)
			}
			break
		}

		if result != "" {
			fmt.Println(result)
		}
	}

	// Cleanup
	sm.CloseAll()
	return scanner.Err()
}

func runCommand(session *Session, commands map[string]CommandHandler, line string) (string, error) {
	parts := parseLine(line)
	if len(parts) == 0 {
		return "", nil
	}

	cmdName := parts[0]
	args := parts[1:]

	return runCommandArgs(session, commands, cmdName, args)
}

func runCommandArgs(session *Session, commands map[string]CommandHandler, cmdName string, args []string) (string, error) {
	handler, ok := commands[cmdName]
	if !ok {
		return "", fmt.Errorf("unknown command: %q (type 'help' for available commands)", cmdName)
	}
	return handler(session, args)
}

// parseLine splits a line into command and arguments, respecting quoted strings.
func parseLine(line string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false

	for _, r := range line {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
