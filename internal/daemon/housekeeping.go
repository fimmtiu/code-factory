package daemon

import (
	"time"

	"github.com/fimmtiu/tickets/internal/config"
	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/protocol"
)

func init() {
	// "housekeeping" must not update the last-non-housekeeping-command timestamp.
	housekeepingCommands["housekeeping"] = true
}

// RegisterHousekeeping registers the housekeeping command handler on the
// given worker.
func RegisterHousekeeping(w *Worker, d *Daemon) {
	w.RegisterHandler("housekeeping", makeHousekeepingHandler(w, d))
}

// StartHousekeepingTimer starts a ticker that fires every interval and pushes
// a housekeeping command onto the daemon's queue. The caller is responsible
// for stopping the ticker when the daemon shuts down.
func StartHousekeepingTimer(d *Daemon, interval time.Duration) *time.Ticker {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			item := &QueueItem{
				Cmd:      protocol.Command{Name: "housekeeping"},
				Response: make(chan protocol.Response, 1),
			}
			select {
			case d.queue <- item:
			case <-d.ctx.Done():
				return
			}
		}
	}()
	return ticker
}

// makeHousekeepingHandler returns a HandlerFunc that:
//   - Resets the status of any stale "in-progress" ticket back to "idle".
//   - Releases the claim on any stale claimed ticket.
//   - Calls stopFn if the daemon has been idle for longer than ExitAfterMinutes.
func makeHousekeepingHandler(w *Worker, d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		cfg, err := config.Load(d.ticketsDir)
		if err != nil {
			return protocol.Response{Success: false, Error: "housekeeping: load config: " + err.Error()}
		}

		staleThreshold := time.Duration(cfg.StaleThresholdMinutes) * time.Minute
		now := time.Now().UTC()

		for _, wu := range d.state.All() {
			if wu.IsProject {
				continue
			}
			if now.Sub(wu.LastUpdated) <= staleThreshold {
				continue
			}

			changed := false
			if wu.Status == models.StatusInProgress {
				wu.Status = models.StatusIdle
				changed = true
			}
			if wu.ClaimedBy != "" {
				wu.ClaimedBy = ""
				changed = true
			}
			if changed {
				if err := d.state.Update(wu); err != nil {
					// Best-effort; continue processing other units.
					continue
				}
			}
		}

		// Check whether the daemon has been idle too long.
		exitAfter := time.Duration(cfg.ExitAfterMinutes) * time.Minute
		lastCmd := w.LastNonHousekeepingCmd()
		idle := lastCmd.IsZero() || now.Sub(lastCmd) > exitAfter
		if idle {
			go w.stopFn()
		}

		return protocol.Response{Success: true}
	}
}
