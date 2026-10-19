package main

import (
	"fmt"
	"os"
	"os/signal"
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
	session := "default"
	command := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-s" || arg == "--session":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a session name", arg)
			}
			i++
			session = args[i]
		case len(arg) > 3 && arg[:3] == "-s=":
			session = arg[3:]
		case len(arg) > 10 && arg[:10] == "--session=":
			session = arg[10:]
		default:
			command = append(command, arg)
		}
	}

	if len(command) == 0 {
		return cli.Run(session)
	}

	if command[0] == "daemon" {
		if len(command) > 1 && command[1] == "stop" {
			output, err := cli.SendCommand(session, "stop", nil)
			if err == nil {
				fmt.Println(output)
			}
			return err
		}
		return runDaemon()
	}

	output, err := cli.SendCommand(session, command[0], command[1:])
	if err != nil {
		return err
	}
	if output != "" {
		fmt.Println(output)
	}
	return nil
}
