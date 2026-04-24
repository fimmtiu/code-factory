package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/gitutil"
	"github.com/fimmtiu/code-factory/internal/models"
)

func openUITestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir, dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	d.SetGitClient(&gitutil.FakeGitClient{})
	t.Cleanup(func() { d.Close() })
	return d
}

// runMsgChain synchronously executes a tea.Cmd, draining any batched
// sub-commands until nothing remains. Returns every non-nil message produced.
func runMsgChain(cmd tea.Cmd) []tea.Msg {
	var out []tea.Msg
	if cmd == nil {
		return out
	}
	stack := []tea.Cmd{cmd}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if c == nil {
			continue
		}
		msg := c()
		switch m := msg.(type) {
		case tea.BatchMsg:
			for _, sub := range m {
				stack = append(stack, sub)
			}
		default:
			if msg != nil {
				out = append(out, msg)
			}
		}
	}
	return out
}

func TestAddTicketDialog_NoDepsCreatesImplementTicket(t *testing.T) {
	d := openUITestDB(t)
	if err := d.CreateProject("proj", "P", nil, ""); err != nil {
		t.Fatal(err)
	}
	parent := &models.WorkUnit{Identifier: "proj", IsProject: true}

	dlg := NewAddTicketDialog(d, parent, nil, 100)
	dlg.slug = []rune("ticket-one")
	dlg.desc.insertRunes([]rune("Do the thing"))

	_, cmd := dlg.submit()
	msgs := runMsgChain(cmd)

	var created *ticketCreatedMsg
	for _, m := range msgs {
		if tc, ok := m.(ticketCreatedMsg); ok {
			created = &tc
		}
	}
	if created == nil {
		t.Fatal("no ticketCreatedMsg emitted")
	}
	if created.errMsg != "" {
		t.Fatalf("unexpected error: %s", created.errMsg)
	}
	if created.identifier != "proj/ticket-one" {
		t.Fatalf("identifier = %q, want proj/ticket-one", created.identifier)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	var got *models.WorkUnit
	for _, u := range units {
		if u.Identifier == "proj/ticket-one" {
			got = u
		}
	}
	if got == nil {
		t.Fatal("created ticket not found in DB")
	}
	if got.Phase != models.PhaseImplement {
		t.Errorf("phase = %s, want implement", got.Phase)
	}
	if got.Status != models.StatusIdle {
		t.Errorf("status = %s, want idle", got.Status)
	}
}

func TestAddTicketDialog_WithDepsCreatesBlockedTicket(t *testing.T) {
	d := openUITestDB(t)
	if err := d.CreateProject("proj", "P", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/first", "First", nil, ""); err != nil {
		t.Fatal(err)
	}
	parent := &models.WorkUnit{Identifier: "proj", IsProject: true}
	units := []*models.WorkUnit{{Identifier: "proj/first"}}

	dlg := NewAddTicketDialog(d, parent, units, 100)
	dlg.slug = []rune("second")
	dlg.desc.insertRunes([]rune("Depends on first"))
	dlg.deps.AddPicked("proj/first")

	_, cmd := dlg.submit()
	runMsgChain(cmd)

	got := findCreatedUnit(t, d, "proj/second")
	if got.Phase != models.PhaseBlocked {
		t.Errorf("phase = %s, want blocked", got.Phase)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0] != "proj/first" {
		t.Errorf("dependencies = %v, want [proj/first]", got.Dependencies)
	}
}

func TestAddTicketDialog_EmptySlugShowsError(t *testing.T) {
	d := openUITestDB(t)
	parent := &models.WorkUnit{Identifier: "proj", IsProject: true}
	dlg := NewAddTicketDialog(d, parent, nil, 100)
	dlg.desc.insertRunes([]rune("desc"))

	_, cmd := dlg.submit()
	if cmd != nil {
		t.Error("submit() returned a command despite empty slug")
	}
	if !strings.Contains(dlg.errMsg, "Name") {
		t.Errorf("errMsg = %q, want mention of Name", dlg.errMsg)
	}
}

func findCreatedUnit(t *testing.T, d *db.DB, identifier string) *models.WorkUnit {
	t.Helper()
	units, err := d.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range units {
		if u.Identifier == identifier {
			return u
		}
	}
	t.Fatalf("unit %q not found", identifier)
	return nil
}
