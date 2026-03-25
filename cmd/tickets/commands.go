package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fimmtiu/tickets/internal/client"
	"github.com/fimmtiu/tickets/internal/protocol"
	"github.com/fimmtiu/tickets/internal/storage"
)

// socketPathForRepo locates the daemon socket path and repo root from the
// current working directory.
func socketPathForRepo() (string, string, error) {
	repoRoot, err := storage.FindRepoRoot(".")
	if err != nil {
		return "", "", err
	}
	ticketsDir := storage.TicketsDirPath(repoRoot)
	sockPath := filepath.Join(ticketsDir, ".daemon.sock")
	return sockPath, repoRoot, nil
}

// ensureDaemon finds the daemon socket for the current repo and auto-starts
// the daemon if it is not already running. It returns the socket path on
// success.
func ensureDaemon() (string, error) {
	sockPath, repoRoot, err := socketPathForRepo()
	if err != nil {
		return "", err
	}
	if err := client.EnsureRunning(sockPath, repoRoot); err != nil {
		return "", err
	}
	return sockPath, nil
}

// runCommand dispatches to the appropriate subcommand handler.
func runCommand(subcommand string, args []string) error {
	switch subcommand {
	case "init":
		return runInit()
	case "running":
		sockPath, _, err := socketPathForRepo()
		if err != nil {
			// If we can't find the repo, there is definitely no daemon running.
			fmt.Println("No daemon running")
			return nil
		}
		return runRunning(sockPath)
	case "exit":
		sockPath, _, err := socketPathForRepo()
		if err != nil {
			fmt.Println("No daemon running")
			return nil
		}
		return runExit(sockPath)
	case "status":
		sockPath, err := ensureDaemon()
		if err != nil {
			return err
		}
		return runStatus(sockPath)
	case "create-project":
		sockPath, err := ensureDaemon()
		if err != nil {
			return err
		}
		return runCreateProject(sockPath, args, os.Stdin)
	case "create-ticket":
		sockPath, err := ensureDaemon()
		if err != nil {
			return err
		}
		return runCreateTicket(sockPath, args, os.Stdin)
	case "set-status":
		sockPath, err := ensureDaemon()
		if err != nil {
			return err
		}
		return runSetStatus(sockPath, args)
	case "claim":
		sockPath, err := ensureDaemon()
		if err != nil {
			return err
		}
		return runClaim(sockPath, args)
	case "release":
		sockPath, err := ensureDaemon()
		if err != nil {
			return err
		}
		return runRelease(sockPath, args)
	case "add-comment":
		sockPath, err := ensureDaemon()
		if err != nil {
			return err
		}
		return runAddComment(sockPath, args, os.Stdin)
	case "close-thread":
		sockPath, err := ensureDaemon()
		if err != nil {
			return err
		}
		return runCloseThread(sockPath, args)
	default:
		return fmt.Errorf("unknown subcommand %q; run 'tickets' for usage", subcommand)
	}
}

// runInit finds the repo root, initialises .tickets/, and prints a message.
func runInit() error {
	repoRoot, err := storage.FindRepoRoot(".")
	if err != nil {
		return err
	}
	if err := storage.InitTicketsDir(repoRoot); err != nil {
		return err
	}
	fmt.Printf("Initialized .tickets/ in %s\n", repoRoot)
	return nil
}

// runRunning checks whether the daemon is reachable and prints its PID or a
// "No daemon running" message.
func runRunning(socketPath string) error {
	c := client.NewClient(socketPath)
	pid, err := c.Ping()
	if err != nil {
		fmt.Println("No daemon running")
		return nil
	}
	fmt.Printf("Daemon is running (pid %d)\n", pid)
	return nil
}

// runExit sends an "exit" command to the daemon if it is running, otherwise
// prints "No daemon running".
func runExit(socketPath string) error {
	if !client.IsRunning(socketPath) {
		fmt.Println("No daemon running")
		return nil
	}
	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{Name: "exit"})
	if err != nil {
		return fmt.Errorf("exit: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("exit: daemon returned error: %s", resp.Error)
	}
	fmt.Println("Daemon exiting")
	return nil
}

// printResponseData handles a daemon response: on failure it prints the
// response as JSON to stdout and returns nil; on success it pretty-prints the
// Data payload (if any) and returns nil. Client-side errors (connection
// failures, bad arguments) are returned as errors and never reach this function.
func printResponseData(resp protocol.Response) error {
	if !resp.Success {
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
		return nil
	}
	if resp.Data == nil {
		return nil
	}
	var pretty interface{}
	if err := json.Unmarshal(resp.Data, &pretty); err != nil {
		// Not valid JSON — just print raw
		fmt.Println(string(resp.Data))
		return nil
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		fmt.Println(string(resp.Data))
		return nil
	}
	fmt.Println(string(out))
	return nil
}

// runStatus sends "status" and pretty-prints the response.
func runStatus(socketPath string) error {
	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{Name: "status"})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// stdinInput holds the parsed fields from the stdin JSON for create-project/create-ticket.
type stdinInput struct {
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies"`
}

// parseStdinInput reads and parses a stdinInput from r, returning an error if
// the JSON is malformed or the description field is absent.
func parseStdinInput(cmdName string, r io.Reader) (stdinInput, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return stdinInput{}, fmt.Errorf("%s: read stdin: %w", cmdName, err)
	}
	var input stdinInput
	if err := json.Unmarshal(data, &input); err != nil {
		return stdinInput{}, fmt.Errorf("%s: parse stdin JSON: %w", cmdName, err)
	}
	if input.Description == "" {
		return stdinInput{}, fmt.Errorf("%s: stdin JSON must contain a 'description' field", cmdName)
	}
	return input, nil
}

// runCreateProject sends "create-project" with identifier and description from stdin.
func runCreateProject(socketPath string, args []string, stdin io.Reader) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets create-project <identifier>")
	}
	identifier := args[0]

	input, err := parseStdinInput("create-project", stdin)
	if err != nil {
		return err
	}

	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name: "create-project",
		Params: map[string]string{
			"identifier":  identifier,
			"description": input.Description,
		},
	})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// runCreateTicket sends "create-ticket" with identifier, description, and
// optional dependencies from stdin.
func runCreateTicket(socketPath string, args []string, stdin io.Reader) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets create-ticket <identifier>")
	}
	identifier := args[0]

	input, err := parseStdinInput("create-ticket", stdin)
	if err != nil {
		return err
	}

	params := map[string]string{
		"identifier":  identifier,
		"description": input.Description,
	}
	if len(input.Dependencies) > 0 {
		params["dependencies"] = strings.Join(input.Dependencies, ",")
	}

	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name:   "create-ticket",
		Params: params,
	})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// runSetStatus sends "set-status" with the given identifier, phase, and
// optional status (defaults to "idle" when omitted).
func runSetStatus(socketPath string, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tickets set-status <identifier> <phase> [<status>]")
	}
	params := map[string]string{
		"identifier": args[0],
		"phase":      args[1],
	}
	if len(args) >= 3 {
		params["status"] = args[2]
	}
	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name:   "set-status",
		Params: params,
	})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// runClaim sends "claim" with the given PID and pretty-prints the response.
func runClaim(socketPath string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets claim <pid>")
	}
	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name:   "claim",
		Params: map[string]string{"pid": args[0]},
	})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// runRelease sends "release" with the given identifier.
func runRelease(socketPath string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets release <identifier>")
	}
	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name:   "release",
		Params: map[string]string{"identifier": args[0]},
	})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// runAddComment sends "add-comment" with identifier, code location, author,
// and text read from stdin.
func runAddComment(socketPath string, args []string, stdin io.Reader) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: tickets add-comment <identifier> <code-location> <author>")
	}
	text, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("add-comment: read stdin: %w", err)
	}
	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name: "add-comment",
		Params: map[string]string{
			"identifier":    args[0],
			"code_location": args[1],
			"author":        args[2],
			"text":          string(text),
		},
	})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// runCloseThread sends "close-thread" with the given thread ID.
func runCloseThread(socketPath string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets close-thread <thread-id>")
	}
	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name:   "close-thread",
		Params: map[string]string{"thread_id": args[0]},
	})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}
