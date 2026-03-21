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

// socketPath returns the path to the daemon socket for the current repository.
// It finds the repo root from the current directory.
func socketPathForRepo() (string, string, error) {
	repoRoot, err := storage.FindRepoRoot(".")
	if err != nil {
		return "", "", err
	}
	ticketsDir := storage.TicketsDirPath(repoRoot)
	sockPath := filepath.Join(ticketsDir, ".daemon.sock")
	return sockPath, repoRoot, nil
}

// runCommand dispatches to the appropriate subcommand handler.
func runCommand(subcommand string, args []string) error {
	switch subcommand {
	case "init":
		return runInit(args)
	case "running":
		sockPath, _, err := socketPathForRepo()
		if err != nil {
			// If we can't find the repo, there is definitely no daemon running
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
		sockPath, repoRoot, err := socketPathForRepo()
		if err != nil {
			return err
		}
		if err := client.EnsureRunning(sockPath, repoRoot); err != nil {
			return err
		}
		return runStatus(sockPath)
	case "create-project":
		sockPath, repoRoot, err := socketPathForRepo()
		if err != nil {
			return err
		}
		if err := client.EnsureRunning(sockPath, repoRoot); err != nil {
			return err
		}
		return runCreateProject(sockPath, args, os.Stdin)
	case "create-ticket":
		sockPath, repoRoot, err := socketPathForRepo()
		if err != nil {
			return err
		}
		if err := client.EnsureRunning(sockPath, repoRoot); err != nil {
			return err
		}
		return runCreateTicket(sockPath, args, os.Stdin)
	case "get-work":
		sockPath, repoRoot, err := socketPathForRepo()
		if err != nil {
			return err
		}
		if err := client.EnsureRunning(sockPath, repoRoot); err != nil {
			return err
		}
		return runGetWork(sockPath)
	case "review-ready":
		sockPath, repoRoot, err := socketPathForRepo()
		if err != nil {
			return err
		}
		if err := client.EnsureRunning(sockPath, repoRoot); err != nil {
			return err
		}
		return runReviewReady(sockPath, args)
	case "get-review":
		sockPath, repoRoot, err := socketPathForRepo()
		if err != nil {
			return err
		}
		if err := client.EnsureRunning(sockPath, repoRoot); err != nil {
			return err
		}
		return runGetReview(sockPath)
	case "done":
		sockPath, repoRoot, err := socketPathForRepo()
		if err != nil {
			return err
		}
		if err := client.EnsureRunning(sockPath, repoRoot); err != nil {
			return err
		}
		return runDone(sockPath, args)
	default:
		return fmt.Errorf("unknown subcommand %q; run 'tickets' for usage", subcommand)
	}
}

// runInit finds the repo root, initialises .tickets/, and prints a message.
func runInit(args []string) error {
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

// printResponseData pretty-prints the Data field of a response, or prints an
// error and returns it if the response indicates failure.
func printResponseData(resp protocol.Response) error {
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
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

// runCreateProject sends "create-project" with identifier and description from stdin.
func runCreateProject(socketPath string, args []string, stdin io.Reader) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets create-project <identifier>")
	}
	identifier := args[0]

	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("create-project: read stdin: %w", err)
	}
	var input stdinInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("create-project: parse stdin JSON: %w", err)
	}
	if input.Description == "" {
		return fmt.Errorf("create-project: stdin JSON must contain a 'description' field")
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

	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("create-ticket: read stdin: %w", err)
	}
	var input stdinInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("create-ticket: parse stdin JSON: %w", err)
	}
	if input.Description == "" {
		return fmt.Errorf("create-ticket: stdin JSON must contain a 'description' field")
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

// runGetWork sends "get-work" and pretty-prints the response.
func runGetWork(socketPath string) error {
	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{Name: "get-work"})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// runReviewReady sends "review-ready" with the given identifier.
func runReviewReady(socketPath string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets review-ready <identifier>")
	}
	identifier := args[0]

	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name:   "review-ready",
		Params: map[string]string{"identifier": identifier},
	})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// runGetReview sends "get-review" and pretty-prints the response.
func runGetReview(socketPath string) error {
	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{Name: "get-review"})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}

// runDone sends "done" with the given identifier and prints the response.
func runDone(socketPath string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets done <identifier>")
	}
	identifier := args[0]

	c := client.NewClient(socketPath)
	resp, err := c.SendCommand(protocol.Command{
		Name:   "done",
		Params: map[string]string{"identifier": identifier},
	})
	if err != nil {
		return err
	}
	return printResponseData(resp)
}
