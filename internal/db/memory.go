package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/fimmtiu/code-factory/internal/models"
)

// DefaultMemoryLimit caps how many memories are injected into a single prompt
// so repository memory can't crowd out the rest of the agent's context.
const DefaultMemoryLimit = 20

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
		id, err = res.LastInsertId()
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
