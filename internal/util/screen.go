package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
	"golang.org/x/sys/unix"
)

// Terminal sequences written when a full-screen program hands the screen back.
// The alt-screen exit and the mode resets are idempotent, so writing them
// after bubbletea has already cleaned up costs nothing and repairs the cases
// where it hasn't.
const (
	cursorPositionRequest = "\x1b[6n"
	exitAltScreenSeq      = "\x1b[?1049l"
	showCursorSeq         = "\x1b[?25h"
	mouseTrackingOffSeq   = "\x1b[?1002l\x1b[?1003l\x1b[?1006l"
	bracketedPasteOffSeq  = "\x1b[?2004l"
	resetScrollRegionSeq  = "\x1b[r"
	resetAttributesSeq    = "\x1b[0m"
)

// cprTimeoutTenths bounds the wait for a cursor position report, in tenths of
// a second: the unit of the VTIME termios field.
const cprTimeoutTenths = 2

// CursorPosition is a 1-based terminal cursor position. The zero value means
// the position is unknown, either because stdio is not a terminal or because
// the terminal did not answer the position request.
type CursorPosition struct {
	Row, Col int
}

// SaveCursorPosition asks the terminal where the cursor is. Call it before a
// full-screen program takes over the screen, and pass the result to
// RestoreTerminal afterwards.
//
// Leaving the alternate screen is supposed to restore the cursor by itself,
// but terminals drop the position they saved when the window is resized while
// the TUI holds the screen. The shell prompt then lands at the top of the
// screen, on top of the output that was there before code-factory started.
func SaveCursorPosition() CursorPosition {
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		return CursorPosition{}
	}

	fd := int(os.Stdin.Fd())
	previous, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return CursorPosition{}
	}

	// Read the reply without echoing it, and without a blocking read that a
	// terminal which ignores the request would never release: VMIN 0 with
	// VTIME set makes the read return empty when the timeout expires.
	polling := *previous
	polling.Lflag &^= unix.ICANON | unix.ECHO
	polling.Cc[unix.VMIN] = 0
	polling.Cc[unix.VTIME] = cprTimeoutTenths
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &polling); err != nil {
		return CursorPosition{}
	}
	defer func() { _ = unix.IoctlSetTermios(fd, unix.TIOCSETA, previous) }()

	if _, err := os.Stdout.WriteString(cursorPositionRequest); err != nil {
		return CursorPosition{}
	}
	return parseCursorPositionReport(readCursorPositionReport(fd))
}

// RestoreTerminal gives the screen back to the shell: out of the alternate
// screen, with the input modes the TUI switched on switched off again, and the
// cursor visible at pos so that the prompt appears below the output that was
// on the screen before the TUI started instead of over it. A zero pos leaves
// the cursor wherever the alt-screen exit put it.
func RestoreTerminal(pos CursorPosition) {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return
	}

	var seq strings.Builder
	seq.WriteString(mouseTrackingOffSeq)
	seq.WriteString(bracketedPasteOffSeq)
	seq.WriteString(exitAltScreenSeq)
	// Both of these apply to the screen we have just switched back to, so
	// they have to follow the alt-screen exit.
	seq.WriteString(resetScrollRegionSeq)
	seq.WriteString(resetAttributesSeq)
	seq.WriteString(showCursorSeq)
	if row := clampRowToScreen(pos.Row); row > 0 {
		fmt.Fprintf(&seq, "\x1b[%d;1H", row)
	}

	_, _ = os.Stdout.WriteString(seq.String())
}

// readCursorPositionReport reads the terminal's answer to a position request.
// It gives up on the first empty read, which is how the VTIME timeout above
// reports that nothing is coming.
func readCursorPositionReport(fd int) string {
	var report strings.Builder
	buf := make([]byte, 32)
	for report.Len() < 64 {
		n, err := unix.Read(fd, buf)
		if err != nil || n <= 0 {
			break
		}
		report.Write(buf[:n])
		if strings.ContainsRune(report.String(), 'R') {
			break
		}
	}
	return report.String()
}

// parseCursorPositionReport extracts the position from a report of the form
// "\x1b[<row>;<col>R", returning the zero value if report holds anything else.
func parseCursorPositionReport(report string) CursorPosition {
	start := strings.LastIndex(report, "\x1b[")
	end := strings.LastIndex(report, "R")
	if start < 0 || end < start {
		return CursorPosition{}
	}

	row, col, found := strings.Cut(report[start+2:end], ";")
	if !found {
		return CursorPosition{}
	}
	rowNum, rowErr := strconv.Atoi(row)
	colNum, colErr := strconv.Atoi(col)
	if rowErr != nil || colErr != nil || rowNum < 1 || colNum < 1 {
		return CursorPosition{}
	}
	return CursorPosition{Row: rowNum, Col: colNum}
}

// clampRowToScreen keeps row inside the terminal's current height, which may
// have shrunk while the TUI held the screen. It returns 0 when the row is
// unknown or the height cannot be read.
func clampRowToScreen(row int) int {
	if row < 1 {
		return 0
	}
	size, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || size.Row == 0 {
		return row
	}
	if row > int(size.Row) {
		return int(size.Row)
	}
	return row
}
