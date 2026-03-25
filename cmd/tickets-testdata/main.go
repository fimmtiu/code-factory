// tickets-testdata generates a realistic set of fake projects and tickets in the
// .tickets/ directory of a git repository. It is intended for manual testing
// and local development of the tickets tools.
//
// Usage:
//
//	tickets-testdata [flags]
//	  -seed int     random seed (default: current time)
//	  -target dir   path inside the target git repository (default: ".")
//	  -reset        remove all existing .tickets/ content before generating
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/fimmtiu/tickets/internal/db"
	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/storage"
)

// ---------------------------------------------------------------------------
// Word lists
// ---------------------------------------------------------------------------

var adjectives = []string{
	"amber", "blazing", "cobalt", "crimson", "crystal", "dawn", "deep",
	"drifting", "ember", "fallen", "final", "fleeting", "frozen", "gilded",
	"glass", "golden", "granite", "hollow", "iron", "jade", "lunar", "marble",
	"midnight", "molten", "mossy", "obsidian", "onyx", "pale", "phantom",
	"radiant", "rusted", "scarlet", "silent", "silver", "slate", "smoldering",
	"sterling", "stone", "sunken", "swift", "tangled", "tarnished", "tidal",
	"twilight", "velvet", "verdant", "waning", "woven",
}

var nouns = []string{
	"anchor", "anvil", "arc", "beacon", "bridge", "canvas", "chapel",
	"circuit", "citadel", "compass", "conduit", "crater", "delta", "dune",
	"echo", "ember", "engine", "epoch", "flare", "forge", "fountain",
	"gateway", "harbor", "hearth", "horizon", "keystone", "lantern", "lattice",
	"ledger", "mantle", "mirror", "nexus", "node", "obelisk", "orbit",
	"outpost", "parapet", "pinnacle", "portal", "prism", "relay", "ridge",
	"ripple", "signal", "spire", "summit", "tempest", "threshold", "tower",
	"vault", "vein", "vessel",
}

// fake Go source file paths for code location generation
var fakeFilePaths = []string{
	"cmd/server/main.go",
	"cmd/worker/main.go",
	"internal/auth/handler.go",
	"internal/auth/middleware.go",
	"internal/cache/store.go",
	"internal/config/loader.go",
	"internal/db/migrations.go",
	"internal/db/queries.go",
	"internal/export/csv.go",
	"internal/export/json.go",
	"internal/ingestion/parser.go",
	"internal/ingestion/worker.go",
	"internal/metrics/collector.go",
	"internal/notify/dispatcher.go",
	"internal/ratelimit/limiter.go",
	"internal/scheduler/cron.go",
	"internal/search/index.go",
	"internal/session/manager.go",
	"internal/storage/reader.go",
	"internal/storage/writer.go",
	"pkg/retry/backoff.go",
	"pkg/util/slices.go",
	"pkg/util/strings.go",
}

// fake author names for comment generation
var fakeAuthors = []string{
	"Ada Okonkwo", "Ben Hartley", "Celia Marsh", "Diego Fuentes",
	"Elena Sorokina", "Felix Gruber", "Gina Tran", "Hugo Lefevre",
	"Ingrid Holm", "Jae-won Oh", "Kira Patel", "Luca Ferretti",
	"Maya Lindqvist", "Nate Osei", "Orla Byrne", "Priya Nair",
}

// comment text fragments for realistic review comments
var commentTexts = []string{
	"This will panic on an empty slice — add a bounds check before indexing.",
	"We covered this pattern in the design review; please use the shared helper in pkg/util instead.",
	"The error message here loses the original context. Wrap with fmt.Errorf and %%w.",
	"Nit: the variable name doesn't convey intent. Something like `deadline` would read better.",
	"This duplicates the logic in internal/storage/writer.go — worth extracting to avoid drift.",
	"Good catch on the potential race. LGTM once the mutex scope is narrowed a bit.",
	"This function is doing too much. Consider splitting validation from persistence.",
	"The TODO comment predates the ticket system. Can we track this properly and remove the comment?",
	"Looks correct, but a table-driven test would make the edge cases much easier to see.",
	"Minor: prefer an early return here over the deep nesting — easier to follow.",
	"This log line fires at ERROR level but it's a normal operational event. Should be DEBUG.",
	"Context is threaded through the call chain here but dropped before the DB call. Please propagate.",
	"We'll need to revisit this before the multi-tenant rollout — the assumption of a single tenant is baked in.",
	"The retry interval is hardcoded. Should come from config to allow tuning in production.",
	"Happy with this approach. Nice and readable.",
	"Can we add a comment explaining why we need the second mutex here? Non-obvious.",
	"This branch is unreachable given the earlier guard — safe to delete.",
	"Approved, but please resolve the other thread on this file before merging.",
	"The benchmark shows a 3× regression on the hot path. Worth profiling before we land this.",
	"Straightforward fix. Thanks for the clear commit message too.",
}

// sentence parts for lorem-ipsum-style descriptions
var subjects = []string{
	"The authentication layer", "The caching subsystem", "The data pipeline",
	"The export service", "The ingestion worker", "The metrics collector",
	"The migration script", "The notification module", "The parsing library",
	"The persistence layer", "The query optimizer", "The rate limiter",
	"The reconciliation job", "The rendering engine", "The retry mechanism",
	"The schema validator", "The search index", "The session manager",
	"The sync daemon", "The task scheduler", "The telemetry reporter",
	"The token service", "The webhook dispatcher", "The worker pool",
}

var predicates = []string{
	"currently lacks pagination support and returns all records in a single response",
	"does not handle transient network errors and propagates them directly to callers",
	"duplicates several responsibilities that should be centralised in a shared package",
	"has grown beyond its original scope and needs to be split into focused components",
	"has no test coverage for its error paths, leaving several edge cases untested",
	"ignores context cancellation and continues processing after the parent request ends",
	"leaks goroutines when connections are dropped during in-flight operations",
	"logs at the wrong level, flooding production output with debug information",
	"needs to be extended to support the new multi-tenant authentication scheme",
	"performs redundant database round-trips that should be batched or cached",
	"reads configuration at startup but does not pick up live changes",
	"relies on a deprecated third-party package that has known security issues",
	"requires a full restart to pick up changes to its configuration file",
	"serialises objects in a format incompatible with the downstream consumers",
	"silently drops messages when its internal buffer is full under high load",
	"stores sensitive values in plaintext that should be encrypted at rest",
	"uses a naive linear scan where an indexed lookup would reduce latency",
	"uses global mutable state that makes it unsafe for concurrent use",
	"was written against the v1 API and must be updated for the v2 contract",
	"writes directly to disk without atomic operations, risking partial writes",
}

var resolutions = []string{
	"Introduce a cursor-based pagination API and update all call sites.",
	"Add retry logic with exponential backoff and a configurable attempt limit.",
	"Extract the shared logic into a new internal package consumed by both callers.",
	"Decompose it into three single-responsibility components with a clear interface.",
	"Add table-driven tests covering every documented error condition.",
	"Propagate context through all internal call chains and respect cancellation.",
	"Track goroutine lifetimes explicitly and ensure all paths reach a clean shutdown.",
	"Audit all log statements and align them to the project's severity conventions.",
	"Implement the new scheme behind a feature flag and roll it out incrementally.",
	"Batch the queries and add a short-lived in-process cache keyed by request scope.",
	"Watch the config file for changes and reload without restarting the process.",
	"Replace the dependency with the maintained successor and update all usages.",
	"Move configuration reload into a SIGHUP handler so no restart is required.",
	"Switch to the agreed canonical serialisation format and add a migration tool.",
	"Replace the bounded buffer with backpressure signalling to the producer.",
	"Encrypt sensitive values using the project's key-management service.",
	"Add a secondary index to the relevant table and update the query planner hints.",
	"Encapsulate the state behind a thread-safe type with explicit locking.",
	"Port the integration to the v2 API surface and remove the v1 compatibility shim.",
	"Wrap all writes in a write-to-temp-then-rename sequence.",
}

// ---------------------------------------------------------------------------
// Generator
// ---------------------------------------------------------------------------

type projectNode struct {
	identifier  string
	description string
	children    []*projectNode // subprojects
	tickets     []ticketSpec
}

type ticketSpec struct {
	identifier  string
	description string
	depIndex    int // index into sibling tickets, -1 = no dep
}

// depIdentifier returns the identifier of the ticket this spec depends on,
// using the project's ticket list to resolve the index. Returns "" if there
// is no dependency.
func (p *projectNode) depIdentifier(spec ticketSpec) string {
	if spec.depIndex < 0 {
		return ""
	}
	return p.tickets[spec.depIndex].identifier
}

type generator struct {
	rng  *rand.Rand
	db   *db.DB
	used map[string]bool // prevent identifier collisions
}

func newGenerator(rng *rand.Rand, d *db.DB) *generator {
	return &generator{rng: rng, db: d, used: map[string]bool{}}
}

func (g *generator) slug() string {
	for {
		adj := adjectives[g.rng.Intn(len(adjectives))]
		noun := nouns[g.rng.Intn(len(nouns))]
		slug := adj + "-" + noun
		if !g.used[slug] {
			g.used[slug] = true
			return slug
		}
	}
}

func (g *generator) description() string {
	subj := subjects[g.rng.Intn(len(subjects))]
	pred := predicates[g.rng.Intn(len(predicates))]
	res := resolutions[g.rng.Intn(len(resolutions))]
	return fmt.Sprintf("%s %s. %s", subj, pred, res)
}

func (g *generator) projectDescription() string {
	verbs := []string{"Overhaul", "Extend", "Refactor", "Harden", "Migrate", "Decompose", "Modernise"}
	areas := []string{
		"the authentication and session-management stack",
		"the core data-persistence layer",
		"the event-streaming and notification pipeline",
		"the internal metrics and observability infrastructure",
		"the job scheduling and worker-pool subsystem",
		"the legacy export and import workflows",
		"the primary read-path query infrastructure",
		"the public API surface and its serialisation contracts",
		"the rate-limiting and quota-enforcement middleware",
		"the search and indexing subsystem",
	}
	goals := []string{
		"to improve reliability under high concurrency.",
		"to reduce operational toil for the on-call team.",
		"to support the upcoming multi-tenant requirements.",
		"to bring the codebase in line with current best practices.",
		"to eliminate the backlog of known correctness issues.",
		"to unblock the dependent platform migration.",
	}
	verb := verbs[g.rng.Intn(len(verbs))]
	area := areas[g.rng.Intn(len(areas))]
	goal := goals[g.rng.Intn(len(goals))]
	return fmt.Sprintf("%s %s %s", verb, area, goal)
}

// buildTree constructs a random project hierarchy with 5–7 total projects
// and at least 24 tickets across them.
func (g *generator) buildTree() []*projectNode {
	// We target 5–7 total projects; subprojects live inside a top-level project.
	totalProjects := 5 + g.rng.Intn(3) // 5, 6, or 7
	numTopLevel := 3 + g.rng.Intn(2)   // 3 or 4
	numSub := totalProjects - numTopLevel

	roots := g.buildRoots(numTopLevel)
	g.distributeSubprojects(roots, numSub)
	g.fillTickets(roots)
	return roots
}

func (g *generator) buildRoots(n int) []*projectNode {
	roots := make([]*projectNode, n)
	for i := range roots {
		roots[i] = &projectNode{
			identifier:  g.slug(),
			description: g.projectDescription(),
		}
	}
	return roots
}

// distributeSubprojects spreads numSub subprojects across top-level roots,
// capping at 2 subprojects per root to keep the tree balanced.
func (g *generator) distributeSubprojects(roots []*projectNode, numSub int) {
	for i := 0; i < numSub; i++ {
		for {
			idx := g.rng.Intn(len(roots))
			if len(roots[idx].children) < 2 {
				sub := &projectNode{
					identifier:  roots[idx].identifier + "/" + g.slug(),
					description: g.projectDescription(),
				}
				roots[idx].children = append(roots[idx].children, sub)
				break
			}
		}
	}
}

// fillTickets assigns 2–5 tickets per project until the tree holds at least 24.
func (g *generator) fillTickets(roots []*projectNode) {
	allProjects := g.flattenProjects(roots)
	totalTickets := 0
	for totalTickets < 24 {
		for _, p := range allProjects {
			n := 2 + g.rng.Intn(4) // 2–5 tickets per project
			g.assignTickets(p, n)
			totalTickets += n
			if totalTickets >= 28 {
				return
			}
		}
	}
}

func (g *generator) flattenProjects(roots []*projectNode) []*projectNode {
	var all []*projectNode
	var walk func(*projectNode)
	walk = func(p *projectNode) {
		all = append(all, p)
		for _, c := range p.children {
			walk(c)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return all
}

// assignTickets adds n tickets to p, with roughly one dependency chain per project.
func (g *generator) assignTickets(p *projectNode, n int) {
	slugPrefix := p.identifier + "/"
	for i := 0; i < n; i++ {
		spec := ticketSpec{
			identifier:  slugPrefix + g.slug(),
			description: g.description(),
			depIndex:    -1,
		}
		// ~30% chance of depending on the previous ticket, once we have one.
		if i > 0 && g.rng.Float32() < 0.30 {
			spec.depIndex = i - 1
		}
		p.tickets = append(p.tickets, spec)
	}
}

// write persists the generated tree into the SQLite database.
func (g *generator) write(roots []*projectNode) error {
	for _, root := range roots {
		if err := g.writeProject(root); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) writeProject(p *projectNode) error {
	if err := g.db.CreateProject(p.identifier, p.description, nil); err != nil {
		return fmt.Errorf("project %s: %w", p.identifier, err)
	}
	if err := g.writeTickets(p); err != nil {
		return err
	}
	for _, child := range p.children {
		if err := g.writeProject(child); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) writeTickets(p *projectNode) error {
	for i, spec := range p.tickets {
		var deps []string
		if dep := p.depIdentifier(spec); dep != "" {
			deps = []string{dep}
		}
		if err := g.db.CreateTicket(spec.identifier, spec.description, deps); err != nil {
			return fmt.Errorf("ticket %d (%s): %w", i, spec.identifier, err)
		}
		for _, thread := range g.generateCommentThreads() {
			for _, comment := range thread.Comments {
				if err := g.db.AddComment(spec.identifier, thread.CodeLocation, comment.Author, comment.Text); err != nil {
					return fmt.Errorf("comment for ticket %s: %w", spec.identifier, err)
				}
			}
		}
	}
	return nil
}

// threadID returns a random 16-character hex string using the generator's rng.
func (g *generator) threadID() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = byte(g.rng.Intn(256))
	}
	return fmt.Sprintf("%x", b)
}

// fakeCommitHash returns a short fake commit hash using the generator's rng.
func (g *generator) fakeCommitHash() string {
	b := make([]byte, 4)
	for i := range b {
		b[i] = byte(g.rng.Intn(256))
	}
	return fmt.Sprintf("%x", b)
}

// codeLocation returns a random "file:line" string.
func (g *generator) codeLocation() string {
	file := fakeFilePaths[g.rng.Intn(len(fakeFilePaths))]
	line := 1 + g.rng.Intn(300)
	return fmt.Sprintf("%s:%d", file, line)
}

// generateComments returns n comments with dates spread over the past 60 days,
// sorted oldest-first.
func (g *generator) generateComments(n int, newest time.Time) []models.Comment {
	comments := make([]models.Comment, n)
	for i := range comments {
		// Each subsequent comment is newer than the previous (spread over 60 days).
		offsetHours := g.rng.Intn(60 * 24)
		comments[i] = models.Comment{
			Date:   newest.Add(-time.Duration(offsetHours) * time.Hour),
			Author: fakeAuthors[g.rng.Intn(len(fakeAuthors))],
			Text:   commentTexts[g.rng.Intn(len(commentTexts))],
		}
	}
	// Sort ascending by date so the thread reads chronologically.
	for i := 1; i < len(comments); i++ {
		for j := i; j > 0 && comments[j].Date.Before(comments[j-1].Date); j-- {
			comments[j], comments[j-1] = comments[j-1], comments[j]
		}
	}
	return comments
}

// generateCommentThreads returns a random set of comment threads for a ticket,
// or nil (~60% probability) when the ticket should have no comments.
func (g *generator) generateCommentThreads() []models.CommentThread {
	if g.rng.Float32() >= 0.4 {
		return nil
	}
	numThreads := 1 + g.rng.Intn(3) // 1–3 threads
	now := time.Now().UTC()
	threads := make([]models.CommentThread, numThreads)
	usedLocations := map[string]bool{}
	for i := range threads {
		// Pick a code location not already used by an open thread.
		var loc string
		for {
			loc = g.codeLocation()
			if !usedLocations[loc] {
				break
			}
		}

		status := models.ThreadOpen
		if g.rng.Float32() < 0.35 {
			status = models.ThreadClosed
		}
		if status == models.ThreadOpen {
			usedLocations[loc] = true
		}

		threads[i] = models.CommentThread{
			ID:           g.threadID(),
			CommitHash:   g.fakeCommitHash(),
			CodeLocation: loc,
			Status:       status,
			Comments:     g.generateComments(1+g.rng.Intn(4), now),
		}
	}
	return threads
}

// ---------------------------------------------------------------------------
// Summary printing
// ---------------------------------------------------------------------------

func printSummary(roots []*projectNode) {
	var projectCount, ticketCount int
	var printNode func(p *projectNode, indent string)
	printNode = func(p *projectNode, indent string) {
		projectCount++
		fmt.Printf("%s[project] %s\n", indent, p.identifier)
		for _, t := range p.tickets {
			ticketCount++
			dep := ""
			if id := p.depIdentifier(t); id != "" {
				dep = fmt.Sprintf("  (depends on %s)", id)
			}
			fmt.Printf("%s  [ticket]  %s%s\n", indent, t.identifier, dep)
		}
		for _, child := range p.children {
			printNode(child, indent+"  ")
		}
	}

	for _, root := range roots {
		printNode(root, "")
	}

	fmt.Printf("\n%d projects, %d tickets\n", projectCount, ticketCount)
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	seedFlag := flag.Int64("seed", time.Now().UnixNano(), "random seed for reproducibility")
	targetFlag := flag.String("target", ".", "path inside the target git repository")
	resetFlag := flag.Bool("reset", false, "remove existing .tickets/ content before generating")
	flag.Parse()

	if err := run(*seedFlag, *targetFlag, *resetFlag); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(seed int64, target string, reset bool) error {
	repoRoot, err := storage.FindRepoRoot(target)
	if err != nil {
		return err
	}

	ticketsDir := storage.TicketsDirPath(repoRoot)

	if reset {
		if err := os.RemoveAll(ticketsDir); err != nil {
			return fmt.Errorf("removing .tickets/: %w", err)
		}
		fmt.Println("Removed existing .tickets/ directory.")
	}

	if err := storage.InitTicketsDir(repoRoot); err != nil {
		return fmt.Errorf("initialising .tickets/: %w", err)
	}

	d, err := db.Open(ticketsDir, repoRoot)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer d.Close()

	gen := newGenerator(rand.New(rand.NewSource(seed)), d)
	roots := gen.buildTree()

	if err := gen.write(roots); err != nil {
		return fmt.Errorf("writing test data: %w", err)
	}

	fmt.Printf("Generated test data (seed %d):\n\n", seed)
	printSummary(roots)
	fmt.Printf("\nData written to: %s\n", ticketsDir)
	return nil
}
