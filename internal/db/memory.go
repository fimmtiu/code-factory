package db

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/fimmtiu/code-factory/internal/models"
)

// DefaultMemoryLimit caps how many memories are injected into a single prompt
// so repository memory can't crowd out the rest of the agent's context.
const DefaultMemoryLimit = 20

// DefaultMaxPerScope caps how many memories are retained per scope. AddMemory
// enforces it on every insert (mirroring InsertLog's log pruning) so the store
// is self-limiting; PruneMemories applies it across all scopes on demand.
const DefaultMaxPerScope = 50

// Memory is a cross-ticket lesson, pattern, or note recorded by an agent and
// replayed into the prompts of later tickets within its scope.
type Memory struct {
	ID           int64     `json:"id"`
	Scope        string    `json:"scope"` // identifier prefix; "" = repository-global
	Kind         string    `json:"kind"`  // lesson | pattern | gotcha | note
	Text         string    `json:"text"`
	SourceTicket string    `json:"source_ticket"`
	CreatedAt    time.Time `json:"created_at"`
}

// AddMemory records a new memory. scope is an identifier prefix the memory
// applies to (empty for repository-global); kind defaults to "lesson" when
// blank. Returns the new row id.
func (d *DB) AddMemory(scope, kind, text, sourceTicket string) (int64, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, fmt.Errorf("AddMemory: text must not be empty")
	}
	if strings.TrimSpace(kind) == "" {
		kind = "lesson"
	}
	var id int64
	err := d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`INSERT INTO memories (scope, kind, text, source_ticket, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			scope, kind, text, sourceTicket, time.Now().Unix(),
		)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		// Keep the store self-limiting: drop the oldest entries in this scope
		// beyond the cap, the same way InsertLog prunes the logs table.
		_, err = tx.Exec(`
			DELETE FROM memories WHERE scope = ? AND id NOT IN (
				SELECT id FROM memories WHERE scope = ?
				ORDER BY created_at DESC, id DESC LIMIT ?
			)`, scope, scope, DefaultMaxPerScope)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("AddMemory: %w", err)
	}
	return id, nil
}

// MemoriesForIdentifier returns the memories in scope for the given ticket or
// project identifier: those whose scope is global (""), the identifier itself,
// or any ancestor in the identifier tree. Results are newest-first and capped
// at limit (DefaultMemoryLimit when limit <= 0).
func (d *DB) MemoriesForIdentifier(identifier string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = DefaultMemoryLimit
	}

	// Build the set of in-scope prefixes: global, self, and every ancestor.
	scopes := []string{"", identifier}
	for current := identifier; ; {
		parent, ok := models.ParentIdentifierOf(current)
		if !ok {
			break
		}
		scopes = append(scopes, parent)
		current = parent
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(scopes)), ",")
	args := make([]any, 0, len(scopes)+1)
	for _, s := range scopes {
		args = append(args, s)
	}
	args = append(args, limit)

	rows, err := d.db.Query(
		`SELECT id, scope, kind, text, source_ticket, created_at
		 FROM memories
		 WHERE scope IN (`+placeholders+`)
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("MemoriesForIdentifier: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// ListMemories returns all memories, newest-first. Intended for the CLI and
// curation; injection uses MemoriesForIdentifier.
func (d *DB) ListMemories() ([]Memory, error) {
	rows, err := d.db.Query(
		`SELECT id, scope, kind, text, source_ticket, created_at
		 FROM memories
		 ORDER BY created_at DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListMemories: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// DeleteMemory removes a memory by id. Returns an error if no row matched, so
// curation tooling can detect stale ids.
func (d *DB) DeleteMemory(id int64) error {
	res, err := d.db.Exec(`DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("DeleteMemory: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeleteMemory: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("DeleteMemory: no memory with id %d", id)
	}
	return nil
}

// PruneResult reports what a PruneMemories pass removed, broken down by the
// rule that triggered each removal.
type PruneResult struct {
	Duplicates int     `json:"duplicates"` // same scope + text as a newer memory
	AgedOut    int     `json:"aged_out"`   // older than maxAge
	OverCap    int     `json:"over_cap"`   // beyond maxPerScope within a scope
	Deleted    []int64 `json:"deleted"`    // ids removed (empty on a dry run)
}

// PruneMemories curates the store by removing, in order: exact duplicates
// (same scope and normalized text, keeping the newest), memories older than
// maxAge (when > 0), and memories beyond maxPerScope within each scope (when
// > 0, keeping the newest). When dryRun is true it reports the counts without
// deleting anything. Each removed memory is counted under a single rule.
func (d *DB) PruneMemories(maxPerScope int, maxAge time.Duration, dryRun bool) (PruneResult, error) {
	all, err := d.ListMemories() // newest-first
	if err != nil {
		return PruneResult{}, err
	}

	var res PruneResult
	deleting := make(map[int64]bool)

	// 1. Dedup: newest occurrence of each (scope, normalized text) wins.
	seen := make(map[string]bool)
	survivors := make([]Memory, 0, len(all))
	for _, m := range all {
		key := m.Scope + "\x00" + normalizeText(m.Text)
		if seen[key] {
			deleting[m.ID] = true
			res.Duplicates++
			continue
		}
		seen[key] = true
		survivors = append(survivors, m)
	}

	// 2. Age out survivors older than the cutoff.
	if maxAge > 0 {
		cutoff := time.Now().Add(-maxAge).Unix()
		kept := survivors[:0]
		for _, m := range survivors {
			if m.CreatedAt.Unix() < cutoff {
				deleting[m.ID] = true
				res.AgedOut++
				continue
			}
			kept = append(kept, m)
		}
		survivors = kept
	}

	// 3. Per-scope cap. survivors stays newest-first, so within each scope the
	// entries past the cap are the oldest.
	if maxPerScope > 0 {
		perScope := make(map[string]int)
		for _, m := range survivors {
			perScope[m.Scope]++
			if perScope[m.Scope] > maxPerScope {
				deleting[m.ID] = true
				res.OverCap++
			}
		}
	}

	if dryRun || len(deleting) == 0 {
		return res, nil
	}

	ids := make([]int64, 0, len(deleting))
	for id := range deleting {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	if err := d.withTx(func(tx *sql.Tx) error {
		for _, id := range ids {
			if _, err := tx.Exec(`DELETE FROM memories WHERE id = ?`, id); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return PruneResult{}, fmt.Errorf("PruneMemories: %w", err)
	}
	res.Deleted = ids
	return res, nil
}

// normalizeText canonicalizes memory text for duplicate detection: lowercased
// with runs of whitespace collapsed to single spaces.
func normalizeText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// scanMemories materializes a memories result set.
func scanMemories(rows *sql.Rows) ([]Memory, error) {
	var out []Memory
	for rows.Next() {
		var m Memory
		var createdAt int64
		if err := rows.Scan(&m.ID, &m.Scope, &m.Kind, &m.Text, &m.SourceTicket, &createdAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan memories: %w", err)
	}
	return out, nil
}
