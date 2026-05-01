---
name: cf-project
description: "Use when starting a new project that needs to be decomposed into smaller work units for parallel implementation. Triggers on: create a project, write project for, plan this project, requirements for, spec out, /cf-project."
user-invocable: true
---

# Projects Planner

Takes a specification for a large proposed change and breaks it down into a series of Code Factory projects, each containing detailed Code Factory tickets that are clear, actionable, and suitable for implementation.

Terminology for the `cf-tickets` system:
- A "work unit" means a project or ticket.
- A "project" is a collection of tickets that share a single goal.
- A "subproject" is a project nested under another project.
- A "ticket" is a single piece of work, small enough to be accomplished by a single agent in under ten minutes. They often, but not always, belong to projects — you can have individual tickets at the top level for very small pieces of work that don't belong anywhere else.
- An "identifier" is the name of a work unit in kebab-case, hierarchically structured like `parent-project/subproject` or `parent-project/subproject/ticket-name`.

**Identifier rules:**
- Each slash-separated segment must match `[a-z][a-z0-9-]*` (lowercase, starts with a letter, hyphens allowed, no underscores or uppercase)
- A child work unit's identifier is formed by prefixing the parent's identifier with a slash: if project `auth` has a ticket for login, the ticket identifier is `auth/login`
- The system derives parent-child relationships from this prefix structure, so getting it wrong breaks the hierarchy
- Dependencies must also use full identifiers (e.g., `auth/login-endpoint`, not just `login-endpoint`)

## The Job

0. Run `cf-tickets init` in the root directory of the git repository we're in
1. **Ask the user for a target branch.** This is the branch that all project changes will be merged into when complete (the `parent_branch` of the top-level project). Prompt:

   > What branch should this project's changes merge into when complete? (default: `main` or `master`)

   If the user specifies a branch:
   - Check whether it exists: `git branch --list <branch-name>`
   - If it does NOT exist, create it from the current HEAD: `git branch <branch-name>`

   Store the answer for use in Step 9 below. If the user accepts the default (empty response), leave `parent_branch` unset.

2. Read the user-provided specification for the work to be done
3. **Survey existing utilities in the repo.** Build a written "utility inventory" you will refer back to in Steps 6 and 7. Without this, you will repeatedly assume helpers don't exist and end up with multiple tickets independently inventing the same `exec.go` / `http.go` / `retry.go`.
   - List the top-level utility roots that exist (any of `internal/`, `pkg/`, `lib/`, `util/`, `common/`, `tools/`).
   - For each subdirectory under those roots, note its purpose by reading the package doc comment (e.g. `head -n 30 <pkg>/*.go` and look at the `// Package x ...` line, or read the `README.md` if one is present).
   - Record a one-line summary per package: identifier, what it provides, exported entry points the planner is most likely to reuse.
   - Also note conventions visible at the repo root: language, build tool, test layout. The decomposition will inherit them.

   This step is a fact-finding pass. Don't decide anything yet — just know what already exists.
4. Choose a **top-level project name**: a short, descriptive kebab-case identifier that captures what the specification is trying to accomplish (e.g., `task-priority`, `auth-overhaul`, `csv-export`). Shorter is better — this name is prepended to every descendant identifier. Aim for 2–3 words / under 20 characters.
5. Divide the work into logical pieces, each with a short identifier (preferably less than 30 characters, following the identifier rules above). Each of these will become a subproject of the top-level project.
6. **Extract shared infrastructure.** Walk the checklist below. For each entry, answer (a) does the repo already have it? (use the inventory from Step 3) and (b) does any ticket from Step 5 need it? If (a) is no and (b) is yes, the shared code MUST become its own ticket (or subproject) that the relevant downstream tickets depend on. If (a) is yes, every ticket that uses it MUST reference the existing package by name in its description (e.g. "use `internal/cmdexec` to run shell commands") so the implementing agent doesn't reinvent it.

   Common shared-infrastructure patterns to check, in order:

   - **Command / process runner** — any ticket shells out to an external command (`git`, `bundle`, `npm`, `make`, `docker`, etc.).
   - **HTTP / API client** — any ticket makes outbound HTTP calls or talks to a third-party API.
   - **Retry / backoff helper** — any ticket has to tolerate a flaky network or filesystem operation.
   - **Error types & wrapping** — multiple tickets surface the same error category (e.g. "command failed", "fetch failed").
   - **Config / settings loader** — multiple tickets read or write the same configuration source.
   - **Logging / output wrapper** — multiple tickets need consistent stdout/stderr/JSON-log formatting.
   - **File / directory / atomic-write helpers** — multiple tickets do path manipulation, atomic file replacement, or recursive copy/walk.
   - **Parsing / serialisation** — multiple tickets parse the same format (JSON, YAML, TOML, INI, a domain DSL, gemspecs, package.json, etc.).
   - **Domain model types** — multiple tickets define the same struct, interface, or enum (e.g. `Package`, `Version`, `Dependency`).
   - **Test helpers / fixtures** — multiple tickets need the same test scaffolding (fake-command runners, golden files, table-driven harnesses).

   Do NOT hand the same shared pattern to two unrelated tickets and trust them to converge. They will each invent their own version, in different files, and the merge will be unrecoverable. When in doubt, extract.

7. Collect all clarifying questions across all projects and present them to the user in a single batch (see "How to ask clarifying questions" below). Wait for answers before proceeding. If nothing is ambiguous, skip this step.
8. Determine the dependencies between projects
9. Create the **top-level project** first. Its description should be a brief overview of the entire specification — what is being built and why. It has no dependencies. If the user specified a target branch in Step 1, include `"parent_branch"` in the JSON.
  ```bash
  cf-tickets create-project <top-level-name> <<'TICKET_JSON'
  {
    "description": "High-level overview of the entire specification...",
    "parent_branch": "<target-branch-from-step-1>"
  }
  TICKET_JSON
  ```
  Omit the `"parent_branch"` field entirely if the user accepted the default.
10. For each subproject, in dependency order (parents and dependencies first):
  10a. Generate a structured PRD, guided by the user's answers from Step 7
  10b. Create the subproject by piping a JSON description into `cf-tickets create-project`. The identifier MUST include the full path from the top-level project down to this subproject. A subproject can live directly under the top-level project or nested under another subproject:
  ```bash
  # Direct child of the top-level project:
  cf-tickets create-project <top-level-name>/<subproject> <<'TICKET_JSON'
  { ... }
  TICKET_JSON

  # Nested under another subproject:
  cf-tickets create-project <top-level-name>/<parent-subproject>/<child-subproject> <<'TICKET_JSON'
  { ... }
  TICKET_JSON
  ```
  The JSON body is the same in both cases:
  ```json
  {
    "dependencies": [
      "<top-level-name>/path/to/a-dependency",
      "<top-level-name>/path/to/another-dependency"
    ],
    "write_scope": [
      "path/to/package/",
      "path/to/specific_file.go"
    ],
    "description": "Full PRD content here (see below for format)..."
  }
  ```
  Omit `dependencies` or pass `[]` if the subproject has none.
  10c. For each user story in the PRD, create a ticket. Derive the ticket identifier from the user story title, not its number. The ticket identifier is the parent subproject's full identifier plus the ticket name. Examples:
  - Story "US-001: Add priority field" in subproject `task-priority/models` → ticket `task-priority/models/add-priority-field`
  - Story "US-002: Validate input" in nested subproject `task-priority/api/validation` → ticket `task-priority/api/validation/validate-input`
  ```bash
  cf-tickets create-ticket <full-parent-path>/<ticket-name> <<'TICKET_JSON'
  {
    "dependencies": [
      "<top-level-name>/path/to/other-ticket"
    ],
    "write_scope": [
      "path/to/package/",
      "path/to/specific_file.go"
    ],
    "description": "Full user story content here..."
  }
  TICKET_JSON
  ```

11. **Cross-validate write scopes.** Review every ticket and project you just created. For each pair of sibling work units (work units that do NOT have a dependency relationship between them), verify their `write_scope` entries do not overlap. If they do, fix the decomposition before finishing: either add a dependency or extract a shared ticket. **An empty `write_scope` on a ticket that creates or modifies files is a bug** — fill it in before checking for overlaps. This is the last step — do not skip it.

**Important:** Do NOT start implementing. Your job ends when all projects and tickets have been created with `cf-tickets` and write scopes have been cross-validated. Do not write any code, modify any source files, or begin work on any ticket.

---

## How to divide the project

Each subproject should either describe an individual feature of the proposed change or lay necessary groundwork for implementing future features. Each output PRD must be small enough that it's easily read and worked on in a single context window. All subprojects and tickets live under the top-level project created in Step 9.

### Preventing duplicate code and merge conflicts

Every ticket and subproject must declare a non-empty **write scope** (`write_scope` in the JSON): the set of packages or files it will create or modify. Write scopes must not overlap between sibling work units (work units that do NOT have a dependency relationship). If two work units both need to create or modify the same file or package, resolve it by one of:

1. **Adding a dependency** — make one work unit depend on the other so it uses the other's output rather than reimplementing it.
2. **Extracting a shared ticket** — pull the common functionality into a new ticket that both depend on.

When in doubt, prefer extraction. A small "utility" ticket that two others depend on is far cheaper than a merge conflict.

**Watch for implicit shared code.** If two tickets both say "use X" or "import X" and X does not exist yet, someone must create it. If no ticket in their dependency chain creates X, each implementing agent will invent it independently — producing an add/add merge conflict. The Step 6 checklist (command runner, HTTP client, retry/backoff, error types, config loader, logging wrapper, file/path helpers, parsing, domain types, test helpers) is what catches these — work it patiently. The Step 3 inventory tells you which ones already exist; the rest are decisions you have to make explicit before tickets are created.

If a subproject encompasses a very broad, general feature, you may break it into further nested subprojects with their own short descriptive names and PRDs. For example, if the top-level project is `task-priority` and the subproject `api` is too broad, you could create `task-priority/api/routes` and `task-priority/api/validation` as children of `task-priority/api`. There are no hard limits on nesting below the top-level project, but if you've gone deeper than three levels of nesting (including the top-level project) then something is wrong and you're probably making the projects too fine-grained.

## How to choose project dependencies

You should aim to split up the work into subprojects such that we can parallelize as much work as possible. Any changes (libraries, shared code, foundational functionality, etc.) which will be used by other subprojects MUST be marked as a dependency for those subsequent subprojects. A work unit MUST ONLY depend on functionality implemented by work units that are in its `dependencies` list, or functionality implemented by work units that the work units in the `dependencies` list themselves depend on. (It's a tree; the work you rely on must be in one of the ancestors.)

If you encounter a circular dependency situation, resolve it by splitting out common functionality into a separate subproject that's implemented first. Other subprojects can depend on that common functionality instead of depending on each other.

## How to order the work units

The work units must be created in order such that you MUST create a work unit before you create any work units that depend on it, or which are children of it. Before creating tickets or projects, order them so that the parents and/or dependencies of each work unit occur earlier in the order.

### How to ask clarifying questions

Ask only critical questions where the initial prompt is ambiguous. If the work to be done is clear, or if one of the answers is much more likely than the rest, don't ask the question. Focus on making these points clear:

- **Problem/Goal:** What problem does this solve?
- **Core Functionality:** What are the key actions?
- **Scope/Boundaries:** What should it NOT do?
- **Success Criteria:** How do we know it's done?

NEVER put the questions in the output we send to `cf-tickets`. Prompt the user directly for an answer before projects or tickets are generated.

#### Format questions like this:

```
1. What is the primary goal of this feature?
   A. Improve user onboarding experience
   B. Increase user retention
   C. Reduce support burden
   D. Other: [please specify]

2. Who is the target user?
   A. New users only
   B. Existing users only
   C. All users
   D. Admin users only

3. What is the scope?
   A. Minimal viable version
   B. Full-featured implementation
   C. Just the backend/API
   D. Just the UI
```

This lets users respond with "1A, 2C, 3B" for quick iteration. Remember to indent the options.

## How to Generate a PRD

A PRD should be in Markdown format. It must have these sections:

### 1. Introduction/Overview
Brief description of the feature and the problem it solves.

### 2. Goals
Specific, measurable objectives (bullet list).

### 3. User Stories
Each story needs:
- **Title:** Short descriptive name
- **Description:** "As a [user], I want [feature] so that [benefit]"
- **Acceptance Criteria:** Verifiable checklist of what "done" means

Each story should be small enough to implement in one focused session.

**Format:**
```markdown
### US-001: [Title]
**Description:** As a [user], I want [feature] so that [benefit].

**Acceptance Criteria:**
- [ ] Specific verifiable criterion
- [ ] Another criterion
- [ ] Typecheck/lint passes
- [ ] **[UI stories only]** Verify in browser using dev-browser skill
```

**Important:**
- Acceptance criteria must be verifiable, not vague. "Works correctly" is bad. "Button shows confirmation dialog before deleting" is good.
- **For any story with UI changes:** Always include "Verify in browser using dev-browser skill" as acceptance criteria. This ensures visual verification of frontend work.

### 4. Functional Requirements
Numbered list of specific functionalities:
- "FR-1: The system must allow users to..."
- "FR-2: When a user clicks X, the system must..."

Be explicit and unambiguous. Each requirement should reference the specific package or file it affects (e.g., "FR-1: In `internal/models/`, add a Priority type..."). This helps ensure write scopes are correct and non-overlapping.

### 5. Non-Goals (Out of Scope)
What this feature will NOT include. Critical for managing scope.

### 6. Design Considerations (Optional)
- UI/UX requirements
- Link to mockups if available
- Relevant existing components to reuse

### 7. Technical Considerations (Optional)
- Known constraints or dependencies
- Integration points with existing systems
- Performance requirements

### 8. Success Metrics
How will success be measured?
- "Reduce time to complete X by 50%"
- "Increase conversion rate by 10%"

---

### Writing for Junior Developers

The PRD reader may be a junior developer or AI agent. Therefore:

- Be explicit and unambiguous
- Avoid jargon or explain it
- Provide enough detail to understand purpose and core logic
- Number requirements for easy reference
- Use concrete examples where helpful

---

### Example PRD

```markdown
# PRD: Task Priority System

## Introduction

Add priority levels to tasks so users can focus on what matters most. Tasks can be marked as high, medium, or low priority, with visual indicators and filtering to help users manage their workload effectively.

## Goals

- Allow assigning priority (high/medium/low) to any task
- Provide clear visual differentiation between priority levels
- Enable filtering and sorting by priority
- Default new tasks to medium priority

## User Stories

### US-001: Add priority field to database
**Description:** As a developer, I need to store task priority so it persists across sessions.

**Acceptance Criteria:**
- [ ] Add priority column to tasks table: 'high' | 'medium' | 'low' (default 'medium')
- [ ] Generate and run migration successfully
- [ ] Typecheck passes

### US-002: Display priority indicator on task cards
**Description:** As a user, I want to see task priority at a glance so I know what needs attention first.

**Acceptance Criteria:**
- [ ] Each task card shows colored priority badge (red=high, yellow=medium, gray=low)
- [ ] Priority visible without hovering or clicking
- [ ] Typecheck passes
- [ ] Verify in browser using dev-browser skill

### US-003: Add priority selector to task edit
**Description:** As a user, I want to change a task's priority when editing it.

**Acceptance Criteria:**
- [ ] Priority dropdown in task edit modal
- [ ] Shows current priority as selected
- [ ] Saves immediately on selection change
- [ ] Typecheck passes
- [ ] Verify in browser using dev-browser skill

### US-004: Filter tasks by priority
**Description:** As a user, I want to filter the task list to see only high-priority items when I'm focused.

**Acceptance Criteria:**
- [ ] Filter dropdown with options: All | High | Medium | Low
- [ ] Filter persists in URL params
- [ ] Empty state message when no tasks match filter
- [ ] Typecheck passes
- [ ] Verify in browser using dev-browser skill

## Functional Requirements

- FR-1: Add `priority` field to tasks table ('high' | 'medium' | 'low', default 'medium')
- FR-2: Display colored priority badge on each task card
- FR-3: Include priority selector in task edit modal
- FR-4: Add priority filter dropdown to task list header
- FR-5: Sort by priority within each status column (high to medium to low)

## Non-Goals

- No priority-based notifications or reminders
- No automatic priority assignment based on due date
- No priority inheritance for subtasks

## Technical Considerations

- Reuse existing badge component with color variants
- Filter state managed via URL search params
- Priority stored in database, not computed

## Success Metrics

- Users can change priority in under 2 clicks
- High-priority tasks immediately visible at top of lists
- No regression in task list performance
```

---

### Checklist

Before saving the PRD:

- [ ] Asked clarifying questions with lettered options
- [ ] Incorporated user's answers
- [ ] User stories are small and specific
- [ ] Functional requirements are numbered and unambiguous
- [ ] Non-goals section defines clear boundaries
