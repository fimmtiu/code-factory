package theme

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
)

// currentTheme holds the active theme, set via Init and read via Current.
// Using atomic.Pointer protects against data races if Init is ever called
// while rendering is active (e.g. for runtime theme switching).
// It defaults to Tan so the application always has a usable theme even if
// Init has not yet been called.
var currentTheme atomic.Pointer[Theme]

func init() { currentTheme.Store(Tan()) }

// Current returns the active theme.
func Current() *Theme {
	return currentTheme.Load()
}

// SetCurrent replaces the active theme. This is intended for tests that
// need to swap in a custom theme; production code should use Init.
func SetCurrent(th *Theme) {
	currentTheme.Store(th)
}

// themes maps valid theme names to their constructor functions.
var themes = map[string]func() *Theme{
	"tan":   Tan,
	"dark":  Dark,
	"light": Light,
}

// Init validates name, constructs the corresponding theme, and stores it
// atomically. An empty name is treated as "tan" (the default) so that a
// partial settings.json with `"terminal_theme": ""` does not break startup.
// Returns an error for unrecognised names.
func Init(name string) error {
	if name == "" {
		name = "tan"
	}
	ctor, ok := themes[name]
	if !ok {
		names := make([]string, 0, len(themes))
		for k := range themes {
			names = append(names, k)
		}
		sort.Strings(names)
		return fmt.Errorf("unknown terminal theme %q; valid themes are: %s", name, strings.Join(names, ", "))
	}
	currentTheme.Store(ctor())
	return nil
}
