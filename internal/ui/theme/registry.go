package theme

import "fmt"

// Current holds the active theme, set once at startup via Init.
var Current *Theme

// themes maps valid theme names to their constructor functions.
var themes = map[string]func() *Theme{
	"tan":   Tan,
	"dark":  Tan, // placeholder until US-002
	"light": Tan, // placeholder until US-003
}

// Init validates name, constructs the corresponding theme, and assigns it to
// Current. Returns an error for unrecognised names.
func Init(name string) error {
	ctor, ok := themes[name]
	if !ok {
		return fmt.Errorf("unknown terminal theme %q; valid themes are: tan, dark, light", name)
	}
	Current = ctor()
	return nil
}
