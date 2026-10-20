package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/starframe-dev/cue-tty/pkg/cli"
)

func runDaemon() error {
	daemon := cli.NewDaemon()
	errCh := make(chan error, 1)
	go func() { errCh <- daemon.Run() }()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-errCh:
		return err
	case <-signals:
		deadline := time.Now().Add(2 * time.Second)
		for !cli.IsDaemonRunning() {
			if time.Now().After(deadline) {
				return fmt.Errorf("daemon did not become ready before shutdown")
			}
			select {
			case err := <-errCh:
				return err
			case <-time.After(10 * time.Millisecond):
			}
		}
		if err := daemon.Stop(); err != nil {
			return err
		}
		return <-errCh
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	return runWithSender(args, cli.SendCommand)
}

func runWithSender(args []string, send func(string, string, []string) (string, error)) error {
	session, command, err := parseCLIArgs(args)
	if err != nil {
		return err
	}
	if len(command) == 0 {
		return cli.Run(session)
	}

	if command[0] == "daemon" {
		if len(command) > 1 && command[1] == "stop" {
			output, err := send(session, "stop", nil)
			if err == nil {
				fmt.Println(output)
			}
			return err
		}
		return runDaemon()
	}

	output, err := send(session, command[0], command[1:])
	if err != nil {
		return err
	}
	if output != "" {
		fmt.Println(output)
	}
	return nil
}

func parseCLIArgs(args []string) (string, []string, error) {
	session := "default"
	for i := 0; i < len(args); {
		arg := args[i]
		switch {
		case arg == "--":
			return session, append([]string(nil), args[i+1:]...), nil
		case arg == "-s" || arg == "--session":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s requires a session name", arg)
			}
			session = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "-s="):
			session = arg[len("-s="):]
			i++
		case strings.HasPrefix(arg, "--session="):
			session = arg[len("--session="):]
			i++
		default:
			return session, append([]string(nil), args[i:]...), nil
		}
	}
	return session, nil, nil
}
