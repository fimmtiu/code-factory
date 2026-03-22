// gen-testdata generates a realistic set of fake projects and tickets in the
// .tickets/ directory of a git repository. It is intended for manual testing
// and local development of the tickets tools.
//
// Usage:
//
//	gen-testdata [flags]
//	  -seed int     random seed (default: current time)
//	  -target dir   path inside the target git repository (default: ".")
//	  -reset        remove all existing .tickets/ content before generating
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

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

type generator struct {
	rng        *rand.Rand
	ticketsDir string
	used       map[string]bool // prevent identifier collisions
}

func newGenerator(rng *rand.Rand, ticketsDir string) *generator {
	return &generator{rng: rng, ticketsDir: ticketsDir, used: map[string]bool{}}
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
	// Decide how many top-level projects and subprojects to create.
	// We target 5–7 total projects; subprojects live inside a top-level project.
	totalProjects := 5 + g.rng.Intn(3) // 5, 6, or 7
	numTopLevel := 3 + g.rng.Intn(2)   // 3 or 4
	numSub := totalProjects - numTopLevel

	roots := make([]*projectNode, numTopLevel)
	for i := range roots {
		roots[i] = &projectNode{
			identifier:  g.slug(),
			description: g.projectDescription(),
		}
	}

	// Distribute subprojects among top-level projects (at most 2 per top-level).
	subHosts := make([]int, 0, numSub)
	for i := 0; i < numSub; i++ {
		// Pick a top-level project that has fewer than 2 subprojects.
		for {
			idx := g.rng.Intn(numTopLevel)
			if len(roots[idx].children) < 2 {
				subHosts = append(subHosts, idx)
				sub := &projectNode{
					identifier:  roots[idx].identifier + "/" + g.slug(),
					description: g.projectDescription(),
				}
				roots[idx].children = append(roots[idx].children, sub)
				break
			}
		}
	}
	_ = subHosts

	// Assign tickets. We need at least 24 total; aim for 3–5 per container.
	allProjects := g.flattenProjects(roots)
	totalTickets := 0
	for totalTickets < 24 {
		for _, p := range allProjects {
			n := 2 + g.rng.Intn(4) // 2–5 tickets per project
			g.assignTickets(p, n)
			totalTickets += n
			if totalTickets >= 28 {
				break
			}
		}
	}

	return roots
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

// write persists the generated tree into the .tickets/ directory.
func (g *generator) write(roots []*projectNode) error {
	var writeProject func(*projectNode) error
	writeProject = func(p *projectNode) error {
		// Create the project directory and .project.json.
		projDir := filepath.Join(g.ticketsDir, filepath.FromSlash(p.identifier))
		if err := os.MkdirAll(projDir, 0755); err != nil {
			return err
		}
		wu := models.NewProject(p.identifier, p.description)
		if err := storage.WriteWorkUnit(filepath.Join(projDir, ".project.json"), wu); err != nil {
			return err
		}

		// Write each ticket.
		for i, spec := range p.tickets {
			t := models.NewTicket(spec.identifier, spec.description)
			if spec.depIndex >= 0 {
				dep := p.tickets[spec.depIndex].identifier
				t.SetDependencies([]string{dep})
			}

			// Ticket files live alongside the project directory (one level up)
			// if the ticket belongs to a subproject, or directly under .tickets/
			// for top-level project tickets.
			//
			// Actually ticket files always live inside the project dir:
			// .tickets/<project>/<slug>.json
			slug := ticketSlug(spec.identifier)
			ticketPath := filepath.Join(projDir, slug+".json")
			if err := storage.WriteWorkUnit(ticketPath, t); err != nil {
				return fmt.Errorf("ticket %d (%s): %w", i, spec.identifier, err)
			}
		}

		// Recurse into subprojects.
		for _, child := range p.children {
			if err := writeProject(child); err != nil {
				return err
			}
		}

		return nil
	}

	for _, root := range roots {
		if err := writeProject(root); err != nil {
			return err
		}
	}
	return nil
}

// ticketSlug returns the final path component of a slash-separated identifier.
func ticketSlug(identifier string) string {
	parts := strings.Split(identifier, "/")
	return parts[len(parts)-1]
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
			if t.depIndex >= 0 {
				dep = fmt.Sprintf("  (depends on %s)", p.tickets[t.depIndex].identifier)
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

	repoRoot, err := storage.FindRepoRoot(*targetFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ticketsDir := storage.TicketsDirPath(repoRoot)

	if *resetFlag {
		if err := os.RemoveAll(ticketsDir); err != nil {
			fmt.Fprintln(os.Stderr, "error removing .tickets/:", err)
			os.Exit(1)
		}
		fmt.Println("Removed existing .tickets/ directory.")
	}

	if err := storage.InitTicketsDir(repoRoot); err != nil {
		fmt.Fprintln(os.Stderr, "error initialising .tickets/:", err)
		os.Exit(1)
	}

	rng := rand.New(rand.NewSource(*seedFlag))
	gen := newGenerator(rng, ticketsDir)
	roots := gen.buildTree()

	if err := gen.write(roots); err != nil {
		fmt.Fprintln(os.Stderr, "error writing test data:", err)
		os.Exit(1)
	}

	fmt.Printf("Generated test data (seed %d):\n\n", *seedFlag)
	printSummary(roots)
	fmt.Printf("\nData written to: %s\n", ticketsDir)
}
