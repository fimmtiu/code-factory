// Package daemon implements the tickets background daemon. It opens a Unix
// socket, accepts one command per connection, routes commands through a
// single serialised queue, and returns responses to each caller.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fimmtiu/tickets/internal/gitutil"
	"github.com/fimmtiu/tickets/internal/protocol"
)

// QueueItem is a unit of work pushed onto the daemon's command queue by a
// connection handler. The handler blocks on Response until the worker sends
// a reply.
type QueueItem struct {
	Cmd      protocol.Command
	Response chan protocol.Response
}

// Daemon listens on a Unix socket, accepts one command per connection, and
// serialises all commands through a buffered queue for processing by a single
// worker goroutine.
type Daemon struct {
	socketPath string
	ticketsDir string
	state      *State
	gitClient  gitutil.GitClient
	listener   net.Listener
	queue      chan *QueueItem
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewDaemon creates a Daemon that will listen on socketPath and manage state
// stored in ticketsDir. Call Start to begin accepting connections.
func NewDaemon(socketPath, ticketsDir string) *Daemon {
	ctx, cancel := context.WithCancel(context.Background())
	return &Daemon{
		socketPath: socketPath,
		ticketsDir: ticketsDir,
		state:      NewState(ticketsDir),
		gitClient:  gitutil.NewRealGitClient(),
		queue:      make(chan *QueueItem, 64),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// State returns the daemon's in-memory state.
func (d *Daemon) State() *State {
	return d.state
}

// TicketsDir returns the path to the .tickets directory.
func (d *Daemon) TicketsDir() string {
	return d.ticketsDir
}

// RepoRoot returns the root of the git repository (the parent of TicketsDir).
func (d *Daemon) RepoRoot() string {
	return filepath.Dir(d.ticketsDir)
}

// GitClient returns the git client used by this daemon.
func (d *Daemon) GitClient() gitutil.GitClient {
	return d.gitClient
}

// SetGitClient replaces the git client. Intended for testing.
func (d *Daemon) SetGitClient(gc gitutil.GitClient) {
	d.gitClient = gc
}

// Context returns the daemon's context. It is cancelled when Stop is called.
func (d *Daemon) Context() context.Context {
	return d.ctx
}

// Queue returns the command queue. Callers (typically a Worker) consume
// *QueueItem values from this channel.
func (d *Daemon) Queue() chan *QueueItem {
	return d.queue
}

// Start begins listening on the Unix socket. If a socket file already exists:
//   - If a daemon is already responding to pings, Start returns an error.
//   - If the socket is stale (no response), it is removed before creating a
//     new listener.
func (d *Daemon) Start() error {
	if _, err := os.Stat(d.socketPath); err == nil {
		// Socket file exists — check whether it belongs to a live daemon.
		alive, err := IsRunning(d.socketPath)
		if err != nil {
			return fmt.Errorf("checking existing socket: %w", err)
		}
		if alive {
			return errors.New("daemon already running")
		}
		// Stale socket — clean it up.
		if err := os.Remove(d.socketPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing stale socket: %w", err)
		}
	}

	// Load state from disk before accepting any connections.
	if err := d.state.Load(); err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	ln, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", d.socketPath, err)
	}
	d.listener = ln

	d.wg.Add(1)
	go d.acceptLoop()

	d.watchSignals()

	return nil
}

// watchSignals registers for SIGINT and SIGHUP. When either signal is received
// the daemon is stopped gracefully. The goroutine also exits if the daemon
// context is cancelled first (e.g. Stop was called directly).
func (d *Daemon) watchSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		select {
		case <-sigCh:
			d.Stop()
		case <-d.ctx.Done():
			signal.Stop(sigCh)
		}
	}()
}

// Wait blocks until the daemon's context is done (i.e. Stop has been called)
// and all internal goroutines have exited. It is safe to call from main after
// Start returns.
func (d *Daemon) Wait() {
	<-d.ctx.Done()
	d.wg.Wait()
}

// Stop shuts down the daemon: cancels the context, closes the listener so
// acceptLoop exits, removes the socket file, and waits for all goroutines.
func (d *Daemon) Stop() {
	d.cancel()
	if d.listener != nil {
		d.listener.Close()
	}
	d.wg.Wait()
	if err := os.Remove(d.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}
	close(d.queue)
}

// acceptLoop repeatedly accepts connections until the listener is closed.
func (d *Daemon) acceptLoop() {
	defer d.wg.Done()
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			// Any error on Accept after context cancellation means we're done.
			select {
			case <-d.ctx.Done():
				return
			default:
			}
			// Non-context error (e.g. "use of closed network connection") → stop.
			return
		}
		d.wg.Add(1)
		go d.handleConnection(conn)
	}
}

// handleConnection reads one command from conn, enqueues it, waits for the
// worker's response, writes the response, and closes the connection.
//
// A 200 ms read deadline is applied to ensure the goroutine unblocks
// promptly when the daemon shuts down.
func (d *Daemon) handleConnection(conn net.Conn) {
	defer d.wg.Done()
	defer conn.Close()

	// Apply a short polling read deadline so we can respect context cancellation
	// even while blocked waiting for the client to send a command.
	const pollInterval = 200 * time.Millisecond
	for {
		if err := conn.SetReadDeadline(time.Now().Add(pollInterval)); err != nil {
			return
		}
		cmd, err := protocol.ReadCommand(conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-d.ctx.Done():
					return
				default:
					continue
				}
			}
			// Client closed the connection or sent malformed data.
			return
		}

		// Got a valid command. Clear the deadline for the rest of this exchange.
		if err := conn.SetDeadline(time.Time{}); err != nil {
			return
		}

		respCh := make(chan protocol.Response, 1)
		item := &QueueItem{
			Cmd:      cmd,
			Response: respCh,
		}

		select {
		case d.queue <- item:
		case <-d.ctx.Done():
			return
		}

		select {
		case resp := <-respCh:
			if err := protocol.WriteResponse(conn, resp); err != nil {
				return
			}
		case <-d.ctx.Done():
			return
		}
		return
	}
}

// IsRunning attempts to connect to the socket at socketPath and send a ping
// command. It returns true if a live daemon responds with success, false
// otherwise. A non-nil error is only returned for unexpected I/O failures;
// a missing or stale socket simply returns (false, nil).
func IsRunning(socketPath string) (bool, error) {
	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		// Connection refused or no such file → not running.
		return false, nil
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return false, nil
	}

	if err := protocol.WriteCommand(conn, protocol.Command{Name: "ping"}); err != nil {
		return false, nil
	}

	resp, err := protocol.ReadResponse(conn)
	if err != nil {
		return false, nil
	}

	return resp.Success, nil
}
