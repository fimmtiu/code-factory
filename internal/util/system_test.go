package util

import (
	"strings"
	"testing"

	"github.com/fimmtiu/code-factory/internal/config"
)

// setConfigTerminal points config.Current at settings using the named
// terminal, returning a function that restores the previous value.
func setConfigTerminal(t *testing.T, name string) func() {
	t.Helper()
	prev := config.Current
	s := config.Default()
	s.Terminal = name
	config.Current = s
	return func() { config.Current = prev }
}

func TestSupportedTerminals_IsSortedAndComplete(t *testing.T) {
	got := SupportedTerminals()

	want := []string{"cmux", "iterm2", "orca", "terminal"}
	if len(got) != len(want) {
		t.Fatalf("SupportedTerminals() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SupportedTerminals()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestValidateTerminal_AcceptsEverySupportedName(t *testing.T) {
	for _, name := range SupportedTerminals() {
		if err := ValidateTerminal(name); err != nil {
			t.Errorf("ValidateTerminal(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateTerminal_RejectsUnknownAndListsOrca(t *testing.T) {
	err := ValidateTerminal("hyper")
	if err == nil {
		t.Fatal("ValidateTerminal(\"hyper\") = nil, want an error")
	}
	if !strings.Contains(err.Error(), "orca") {
		t.Errorf("error should list orca as a supported value, got: %v", err)
	}
}

func TestTerminalProfiles_EveryProfileHasBundleIDAndArgs(t *testing.T) {
	for name, p := range TerminalProfiles {
		if p.BundleID == "" {
			t.Errorf("profile %q has no BundleID", name)
		}
		if len(p.OpenArgs) == 0 {
			t.Errorf("profile %q has no OpenArgs", name)
		}
	}
}

func TestOrcaProfile_UsesCLIAndBundleID(t *testing.T) {
	p, ok := TerminalProfiles["orca"]
	if !ok {
		t.Fatal("no \"orca\" terminal profile")
	}
	if p.BundleID != "com.stablyai.orca" {
		t.Errorf("orca BundleID = %q, want %q", p.BundleID, "com.stablyai.orca")
	}
	if p.OpenArgs[0] != "orca" {
		t.Errorf("orca OpenArgs should invoke the orca CLI, got %v", p.OpenArgs)
	}
	// "open -a Orca ." would only focus the app: Orca registers no handler
	// for folders, so the directory has to reach it as a CLI argument.
	if !strings.Contains(strings.Join(p.OpenArgs, " "), dirPlaceholder) {
		t.Errorf("orca OpenArgs must interpolate the directory, got %v", p.OpenArgs)
	}
}

func TestResolveOpenArgs_ExpandsPlaceholder(t *testing.T) {
	p := TerminalProfiles["orca"]

	args := p.resolveOpenArgs("/tmp/wt")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, dirPlaceholder) {
		t.Errorf("placeholder should be expanded, got %v", args)
	}
	if !strings.Contains(joined, "path:/tmp/wt") {
		t.Errorf("expected the worktree selector to carry the path, got %v", args)
	}
}

func TestResolveOpenArgs_KeepsPathWithSpacesInOneArgument(t *testing.T) {
	p := TerminalProfiles["orca"]

	args := p.resolveOpenArgs("/tmp/my worktree")
	found := false
	for _, a := range args {
		if a == "path:/tmp/my worktree" {
			found = true
		}
	}
	if !found {
		t.Errorf("a path containing a space must stay one argument, got %v", args)
	}
}

func TestResolveOpenArgs_LeavesPlaceholderlessProfilesAlone(t *testing.T) {
	p := TerminalProfiles["iterm2"]

	args := p.resolveOpenArgs("/tmp/wt")
	want := []string{"open", "-a", "iTerm", "."}
	if len(args) != len(want) {
		t.Fatalf("resolveOpenArgs() = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestResolveOpenArgs_DoesNotMutateProfile(t *testing.T) {
	p := TerminalProfiles["orca"]
	before := strings.Join(p.OpenArgs, " ")

	p.resolveOpenArgs("/tmp/wt")

	if after := strings.Join(TerminalProfiles["orca"].OpenArgs, " "); after != before {
		t.Errorf("resolveOpenArgs mutated the profile: %q became %q", before, after)
	}
}

func TestLastNonEmptyLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "\n  \n\t\n", ""},
		{"single line", "selector_not_found", "selector_not_found"},
		{"trailing newline", "selector_not_found\n", "selector_not_found"},
		{
			// The orca CLI prints Electron startup chatter before the real
			// complaint, so the last line is the useful one.
			"noise before the error",
			"[0909/122011.308540:ERROR:codesign_util.cc:149] SecCodeCheckValidity\nselector_not_found\n",
			"selector_not_found",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lastNonEmptyLine(c.in); got != c.want {
				t.Errorf("lastNonEmptyLine(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestOpenTerminal_UnknownTerminalReturnsError(t *testing.T) {
	restore := setConfigTerminal(t, "definitely-not-a-terminal")
	defer restore()

	if err := OpenTerminal(t.TempDir()); err == nil {
		t.Error("OpenTerminal with an unknown terminal = nil, want an error")
	}
}

func TestTerminalBundleID(t *testing.T) {
	restore := setConfigTerminal(t, "orca")
	defer restore()

	if got := TerminalBundleID(); got != "com.stablyai.orca" {
		t.Errorf("TerminalBundleID() = %q, want %q", got, "com.stablyai.orca")
	}

	restoreUnknown := setConfigTerminal(t, "nope")
	defer restoreUnknown()

	if got := TerminalBundleID(); got != "" {
		t.Errorf("TerminalBundleID() for an unknown terminal = %q, want empty", got)
	}
}
