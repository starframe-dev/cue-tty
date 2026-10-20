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

	// Interactive REPL mode. Global flags and inline commands are handled by
	// the CLI entry point; do not reinterpret os.Args here.
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
