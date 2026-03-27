package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fimmtiu/tickets/internal/db"
)

// noopGitClient implements gitutil.GitClient without invoking real git.
type noopGitClient struct{}

func (noopGitClient) CreateWorktree(_, _, _ string) error    { return nil }
func (noopGitClient) MergeBranch(_, _, _ string) error       { return nil }
func (noopGitClient) RemoveWorktree(_, _, _ string) error    { return nil }
func (noopGitClient) GetHeadCommit(_ string) (string, error) { return "", nil }

// openTestDB creates a temporary directory and opens a fresh DB in it,
// with a no-op git client so tests don't require a real git repository.
func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir, dir)
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	d.SetGitClient(noopGitClient{})
	t.Cleanup(func() { d.Close() })
	return d
}

// captureOutput captures os.Stdout during fn() and returns what was printed.
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// ===== runInit =====

func TestRunInit_CreatesTicketsDir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	out := captureOutput(func() {
		if err := runInit(); err != nil {
			t.Fatalf("runInit returned error: %v", err)
		}
	})

	ticketsDir := filepath.Join(tmp, ".tickets")
	if _, err := os.Stat(ticketsDir); err != nil {
		t.Errorf("expected .tickets/ to exist: %v", err)
	}
	if !strings.Contains(out, "Initialized .tickets/") {
		t.Errorf("expected output to contain 'Initialized .tickets/', got: %q", out)
	}
}

func TestRunInit_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	if err := runInit(); err != nil {
		t.Fatalf("first runInit error: %v", err)
	}
	if err := runInit(); err != nil {
		t.Fatalf("second runInit error: %v", err)
	}
}

func TestRunInit_NoGitRepo(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	if err := runInit(); err == nil {
		t.Error("expected error when no .git directory, got nil")
	}
}

// ===== runStatus =====

func TestRunStatus_Empty(t *testing.T) {
	d := openTestDB(t)
	out := captureOutput(func() {
		if err := runStatus(d); err != nil {
			t.Fatalf("runStatus returned error: %v", err)
		}
	})
	if !strings.Contains(out, "[]") && !strings.Contains(out, "null") {
		// Accept either "[]" or "null" for an empty result set
		if out == "" {
			t.Error("expected non-empty output from runStatus")
		}
	}
}

func TestRunStatus_WithData(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("my-proj", "A test project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("my-proj/my-ticket", "A test ticket", nil); err != nil {
		t.Fatal(err)
	}

	out := captureOutput(func() {
		if err := runStatus(d); err != nil {
			t.Fatalf("runStatus returned error: %v", err)
		}
	})
	if !strings.Contains(out, "my-proj") {
		t.Errorf("expected output to contain 'my-proj', got: %q", out)
	}
	if !strings.Contains(out, "my-ticket") {
		t.Errorf("expected output to contain 'my-ticket', got: %q", out)
	}
}

// ===== runCreateProject =====

func TestRunCreateProject(t *testing.T) {
	d := openTestDB(t)
	stdin := strings.NewReader(`{"description":"A test project"}`)
	if err := runCreateProject(d, []string{"my-proj"}, stdin); err != nil {
		t.Fatalf("runCreateProject returned error: %v", err)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, u := range units {
		if u.Identifier == "my-proj" && u.IsProject {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected project 'my-proj' in DB after create-project")
	}
}

func TestRunCreateProject_MissingIdentifier(t *testing.T) {
	d := openTestDB(t)
	err := runCreateProject(d, []string{}, strings.NewReader(`{"description":"test"}`))
	if err == nil {
		t.Error("expected error when identifier is missing, got nil")
	}
}

func TestRunCreateProject_MissingDescription(t *testing.T) {
	d := openTestDB(t)
	err := runCreateProject(d, []string{"my-proj"}, strings.NewReader(`{}`))
	if err == nil {
		t.Error("expected error when description is missing, got nil")
	}
}

// ===== runCreateTicket =====

func TestRunCreateTicket(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	stdin := strings.NewReader(`{"description":"A test ticket"}`)
	if err := runCreateTicket(d, []string{"my-proj/my-ticket"}, stdin); err != nil {
		t.Fatalf("runCreateTicket returned error: %v", err)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, u := range units {
		if u.Identifier == "my-proj/my-ticket" && !u.IsProject {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ticket 'my-proj/my-ticket' in DB after create-ticket")
	}
}

func TestRunCreateTicket_MissingIdentifier(t *testing.T) {
	d := openTestDB(t)
	err := runCreateTicket(d, []string{}, strings.NewReader(`{"description":"test"}`))
	if err == nil {
		t.Error("expected error when identifier is missing, got nil")
	}
}

// ===== runSetStatus =====

func TestRunSetStatus(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil); err != nil {
		t.Fatal(err)
	}

	if err := runSetStatus(d, []string{"proj/ticket", "review"}); err != nil {
		t.Fatalf("runSetStatus returned error: %v", err)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range units {
		if u.Identifier == "proj/ticket" {
			if string(u.Phase) != "review" {
				t.Errorf("expected phase 'review', got %q", u.Phase)
			}
			return
		}
	}
	t.Error("ticket not found after set-status")
}

func TestRunSetStatus_MissingArgs(t *testing.T) {
	d := openTestDB(t)
	err := runSetStatus(d, []string{"only-one"})
	if err == nil {
		t.Error("expected error when phase is missing, got nil")
	}
}

// ===== runClaim =====

func TestRunClaim(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil); err != nil {
		t.Fatal(err)
	}

	out := captureOutput(func() {
		if err := runClaim(d, []string{"1234"}); err != nil {
			t.Fatalf("runClaim returned error: %v", err)
		}
	})
	if !strings.Contains(out, "claimed_by") {
		t.Errorf("expected output to contain 'claimed_by', got: %q", out)
	}
	if !strings.Contains(out, "1234") {
		t.Errorf("expected output to contain pid '1234', got: %q", out)
	}
}

func TestRunClaim_MissingPID(t *testing.T) {
	d := openTestDB(t)
	err := runClaim(d, []string{})
	if err == nil {
		t.Error("expected error when pid is missing, got nil")
	}
}

func TestRunClaim_NoneAvailable(t *testing.T) {
	d := openTestDB(t)
	err := runClaim(d, []string{"42"})
	if err == nil {
		t.Error("expected error when no claimable ticket available, got nil")
	}
}

// ===== runRelease =====

func TestRunRelease(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Claim(1234); err != nil {
		t.Fatal(err)
	}

	if err := runRelease(d, []string{"proj/ticket"}); err != nil {
		t.Fatalf("runRelease returned error: %v", err)
	}
}

func TestRunRelease_MissingIdentifier(t *testing.T) {
	d := openTestDB(t)
	err := runRelease(d, []string{})
	if err == nil {
		t.Error("expected error when identifier is missing, got nil")
	}
}

// ===== runAddChangeRequest =====

func TestRunAddChangeRequest(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil); err != nil {
		t.Fatal(err)
	}

	stdin := strings.NewReader("this method ignores context cancellation")
	if err := runAddChangeRequest(d, []string{"proj/ticket", "main.go:42", "alice"}, stdin); err != nil {
		t.Fatalf("runAddChangeRequest returned error: %v", err)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range units {
		if u.Identifier == "proj/ticket" {
			if len(u.ChangeRequests) == 0 {
				t.Error("expected change request on ticket after add-change-request")
			}
			return
		}
	}
	t.Error("ticket not found after add-change-request")
}

func TestRunAddChangeRequest_MissingArgs(t *testing.T) {
	d := openTestDB(t)
	err := runAddChangeRequest(d, []string{"only", "two"}, strings.NewReader("text"))
	if err == nil {
		t.Error("expected error when args are missing, got nil")
	}
}

// ===== runCloseChangeRequest =====

func TestRunCloseChangeRequest(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangeRequest("proj/ticket", "main.go:42", "alice", "please fix"); err != nil {
		t.Fatal(err)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatal(err)
	}
	var crID string
	for _, u := range units {
		if u.Identifier == "proj/ticket" && len(u.ChangeRequests) > 0 {
			crID = u.ChangeRequests[0].ID
			break
		}
	}
	if crID == "" {
		t.Fatal("no change request ID found")
	}

	if err := runCloseChangeRequest(d, []string{crID}); err != nil {
		t.Fatalf("runCloseChangeRequest returned error: %v", err)
	}

	units, err = d.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range units {
		if u.Identifier == "proj/ticket" && len(u.ChangeRequests) > 0 {
			if u.ChangeRequests[0].Status != "closed" {
				t.Errorf("expected status 'closed', got %q", u.ChangeRequests[0].Status)
			}
			return
		}
	}
	t.Error("change request not found after close-change-request")
}

func TestRunCloseChangeRequest_MissingArg(t *testing.T) {
	d := openTestDB(t)
	err := runCloseChangeRequest(d, []string{})
	if err == nil {
		t.Error("expected error when ID is missing, got nil")
	}
}

// ===== runDismissChangeRequest =====

func TestRunDismissChangeRequest(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangeRequest("proj/ticket", "main.go:42", "alice", "please fix"); err != nil {
		t.Fatal(err)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatal(err)
	}
	var crID string
	for _, u := range units {
		if u.Identifier == "proj/ticket" && len(u.ChangeRequests) > 0 {
			crID = u.ChangeRequests[0].ID
			break
		}
	}
	if crID == "" {
		t.Fatal("no change request ID found")
	}

	if err := runDismissChangeRequest(d, []string{crID}); err != nil {
		t.Fatalf("runDismissChangeRequest returned error: %v", err)
	}

	units, err = d.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range units {
		if u.Identifier == "proj/ticket" && len(u.ChangeRequests) > 0 {
			if u.ChangeRequests[0].Status != "dismissed" {
				t.Errorf("expected status 'dismissed', got %q", u.ChangeRequests[0].Status)
			}
			return
		}
	}
	t.Error("change request not found after dismiss-change-request")
}

func TestRunDismissChangeRequest_MissingArg(t *testing.T) {
	d := openTestDB(t)
	err := runDismissChangeRequest(d, []string{})
	if err == nil {
		t.Error("expected error when ID is missing, got nil")
	}
}

// ===== runCommand =====

func TestRunCommand_UnknownSubcommand(t *testing.T) {
	err := runCommand("no-such-command", []string{})
	if err == nil {
		t.Error("expected error for unknown subcommand, got nil")
	}
}
