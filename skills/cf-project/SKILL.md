---
name: cf-project
description: "Use when starting a new project that needs to be decomposed into smaller work units for parallel implementation. Triggers on: create a project, write project for, plan this project, requirements for, spec out, /cf-project."
user-invocable: true
---

# Projects Planner

Takes a specification for a large proposed change and breaks it down into Code Factory projects and tickets that are clear, actionable, and suitable for parallel implementation.

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

The examples in this document use paths and declarations from one language. Follow the conventions of the repository you are planning for, which you record in Step 3.

---

## How work units share a git branch

Read this section before you plan anything. It controls what one agent can see of another agent's work.

The engine behaves like this:

- Each work unit gets its own git worktree and branch when you create it. The branch name is the identifier with each `/` replaced by `_`. The branch is cut from the repository's current HEAD **at planning time**.
- At the start of every phase, a ticket rebases onto its base branch. The base branch is its `parent_branch` if set, otherwise its parent project's branch.
- When a ticket finishes, it merges into that same base branch.

So merged work flows **up** a branch, never sideways or down. Two sibling subprojects are two isolated branch lineages, and nothing one of them merges will ever appear in the other while the project runs. An agent cannot read code that a sibling subproject wrote. It can only guess at the names, and guesses do not agree.

**Rule: every work unit you create must set `parent_branch` to the top-level project's branch name** (the top-level identifier, which has no slashes, so the branch name is the same string). This gives the whole project one shared branch. Subprojects then group PRDs for human reading without splitting the code. Each ticket rebases onto the shared branch at the start of each phase, so it reads every ticket that merged before it.

Two consequences to plan around:

- Merges into the shared branch are serialized by a lock. That is intended. Implementation still runs in parallel; only the merges queue up.
- A ticket that starts before another ticket merges will not see that work. Dependencies are what order them. A dependency is not paperwork — it is the only thing that makes one ticket's output visible to another.

---

## The Job

0. Run `cf-tickets init` in the root directory of the git repository we're in.

1. **Ask the user for a target branch.** This is the branch the whole project merges into when complete (the `parent_branch` of the top-level project). Prompt:

   > What branch should this project's changes merge into when complete? (default: `main` or `master`)

   If the user specifies a branch:
   - Check whether it exists: `git branch --list <branch-name>`
   - If it does NOT exist, create it from the current HEAD: `git branch <branch-name>`

   Store the answer for Step 10. If the user accepts the default (empty response), leave `parent_branch` unset **on the top-level project only**. Every other work unit still gets an explicit `parent_branch`, as described above.

2. Read the user-provided specification for the work to be done.

3. **Survey existing utilities in the repo.** Build a written "utility inventory" you will refer back to in Steps 5 and 6. Code that already exists must be reused by name, so you have to know what is there before you can name it.
   - List the top-level utility roots that exist (any of `internal/`, `pkg/`, `lib/`, `util/`, `common/`, `src/`, `tools/`).
   - For each subdirectory under those roots, note its purpose from the package or module doc comment, or the `README.md` if one is present.
   - Record a one-line summary per package: path, what it provides, the exported entry points the planner is most likely to reuse.
   - Record the conventions visible at the repo root: language, build tool, lint command, test command, test layout, how a module path or import is written. The decomposition inherits all of them.

   This step is fact-finding. Don't decide anything yet — just know what already exists.

4. Choose a **top-level project name**: a short, descriptive kebab-case identifier that captures what the specification is trying to accomplish (e.g. `task-priority`, `auth-overhaul`, `csv-export`). Shorter is better — this name is prepended to every descendant identifier, and it is also the shared branch name. Aim for 2–3 words and under 20 characters.

5. **Divide the work into vertical slices.** See "How to divide the project" below. Each slice delivers one behaviour end to end. Do not split by layer or by container.

6. **Build the contract list.** See "The contract list" below. Produce a table of every named thing that more than one ticket touches — type, function, error class, constant, config field, key name, module path — with exactly one owner and the exact reference an agent must write to use it.

7. **Plan the scaffold ticket.** See "The scaffold ticket" below. Every shared concept on the contract list belongs to the scaffold ticket, which commits real files with real names and empty bodies, and merges before any other ticket starts.

8. Collect all clarifying questions across all slices and present them to the user in a single batch (see "How to ask clarifying questions"). Wait for answers before proceeding. If nothing is ambiguous, skip this step.

9. Determine the dependencies between work units. Every implementation ticket depends on the scaffold ticket.

10. Create the **top-level project** first. Its description should be a brief overview of the entire specification — what is being built and why. It has no dependencies. If the user specified a target branch in Step 1, include `"parent_branch"` in the JSON.
  ```bash
  cf-tickets create-project <top-level-name> <<'TICKET_JSON'
  {
    "description": "High-level overview of the entire specification...",
    "parent_branch": "<target-branch-from-step-1>"
  }
  TICKET_JSON
  ```
  Omit the `"parent_branch"` field entirely if the user accepted the default.

11. **Create the scaffold ticket second**, as a direct child of the top-level project, with no dependencies:
  ```bash
  cf-tickets create-ticket <top-level-name>/scaffold <<'TICKET_JSON'
  {
    "parent_branch": "<top-level-name>",
    "write_scope": ["every/shared/file", "..."],
    "description": "Full scaffold ticket content — see 'The scaffold ticket' below..."
  }
  TICKET_JSON
  ```

12. For each subproject, in dependency order (parents and dependencies first):

  12a. Generate a structured PRD, guided by the user's answers from Step 8.

  12b. Create the subproject. The identifier MUST include the full path from the top-level project down to this subproject:
  ```bash
  cf-tickets create-project <top-level-name>/<subproject> <<'TICKET_JSON'
  {
    "parent_branch": "<top-level-name>",
    "dependencies": ["<top-level-name>/scaffold"],
    "write_scope": ["path/to/package/", "path/to/specific/file"],
    "description": "Full PRD content here (see below for format)..."
  }
  TICKET_JSON
  ```

  12c. For each user story in the PRD, create a ticket. Derive the ticket identifier from the user story title, not its number. Examples:
  - Story "US-001: Add priority field" in subproject `task-priority/models` → ticket `task-priority/models/add-priority-field`
  - Story "US-002: Validate input" in nested subproject `task-priority/api/validation` → ticket `task-priority/api/validation/validate-input`
  ```bash
  cf-tickets create-ticket <full-parent-path>/<ticket-name> <<'TICKET_JSON'
  {
    "parent_branch": "<top-level-name>",
    "dependencies": ["<top-level-name>/scaffold"],
    "write_scope": ["path/to/specific/file"],
    "description": "Full user story content, including the Contract and Standing rules blocks..."
  }
  TICKET_JSON
  ```
  Every ticket description MUST contain:
  - a **Contract** block: the rows of the contract list this ticket needs, copied exactly, with the reference the agent must write;
  - a **Files** block: the files this ticket may change, matching its `write_scope`;
  - the **Standing rules** block, copied word for word from below.

13. **Create a final integration ticket** that depends on every other ticket. See "The integration ticket" below.

14. **Run the three validation checks.** See "Validation" below. They are three different checks that find three different faults. Do not merge them, and do not skip them.

15. **Tell the user where the contract gate is.** In your closing message, state that the scaffold ticket runs alone and that its user-review is the point to check the contract table against the committed code, because every other ticket unblocks as soon as the scaffold merges. Include the contract table in that message.

**Important:** Do NOT start implementing. Your job ends when all work units are created, the three checks pass, and you have reported to the user. Do not write any code, modify any source files, or begin work on any ticket.

---

## The contract list

`write_scope` prevents merge conflicts. It does not prevent duplicated work: two tickets can build the same type in two different files, with no scope overlap at all and no conflict at merge time. The result is two versions with different behaviour, and callers split between them.

So the plan needs a second artifact. Build a table with one row per shared concept:

| Concept | Owner | Reference | Shape |
|---|---|---|---|
| task priority type | `task-priority/scaffold` | `internal/models.Priority` | string enum: `high`, `medium`, `low` |
| default priority | `task-priority/scaffold` | `internal/models.DefaultPriority` | `Priority` constant, value `medium` |
| priority badge | existing repo | `web/src/ui/Badge.tsx` → `Badge` | props `{ level, label }` |
| store read timeout | `task-priority/scaffold` | `internal/config.Config.StoreTimeout` | duration, default 5s |

Write the Reference column the way the repository's language writes it, so an agent can copy it into code without translating it.

What goes on the list:

- Every type, struct, record, or enum that two or more tickets read or build.
- Every function or method that two or more tickets call.
- Every error or exception class that crosses a ticket boundary.
- Every constant, timeout, limit, and retry count. If two tickets both name a number, that number has one owner and both tickets read it from there.
- Every config field, environment variable, and secret name, with its exact spelling.
- Every external key, queue, topic, table, and path name.
- Every module path and process entry point.
- Every shared test helper and fixture.
- Any startup or shutdown sequence that more than one process performs. An ordered sequence written twice will drift.

Use the Step 3 inventory first. If the repo already owns a concept, the Owner column names the existing package, and no ticket may write a second version. If the repo does not own it, the owner is the scaffold ticket.

Work the list patiently: a missing row costs a great deal, and an extra row costs almost nothing. These categories are where shared concepts hide — command or process runner, HTTP client, retry and backoff, error types and wrapping, config loader, logging and output, metrics, error reporting, file and path helpers, atomic writes, parsing and serialisation, domain types, data-store access, startup and shutdown, test helpers.

**Never give the same concept to two tickets and expect them to converge.** Each one writes its own version, and the merge keeps both.

---

## The scaffold ticket

A contract written only in prose does not hold. An agent that reads a description must still choose the names, and two agents choose differently. The scaffold ticket removes the choice: it commits the real files, so every later agent reads the names instead of inventing them.

The scaffold ticket must:

- Create every shared file at its final path, with its final module or package name.
- Declare the full public surface: every signature with complete types, every type and its fields, every error class, every constant with its real value, every config field with its real name.
- Leave every body unimplemented, using the language's idiom for work not yet written (a panic, a thrown error, a "not implemented" return). The scaffold writes no logic.
- Pass the repository's build, type check, and lint commands as recorded in Step 3.
- Add a **contract test** that names every symbol in the contract table: one test file that references each one and constructs each type. A misnamed or missing symbol then fails the build.
- Add a smoke test per process entry point that loads the real entry point and builds the real objects, with nothing mocked.
- Add the shared test helpers and fixtures, with unimplemented bodies where they need logic.

Write the scaffold ticket description as a file-by-file listing, giving the exact declaration of everything on the contract list. Do not write "a config loader"; write the module path, the type name, and every field name and type.

**The scaffold's file layout is also the parallelism plan.** Implementation tickets fill in bodies, so they write the scaffold's files. Lay the files out so each one has at most one implementation ticket filling it. Where two tickets must fill the same file, stack them (chain one as a dependency of the other) so they run serially.

---

## Standing rules for every ticket

Copy this block word for word into every implementation ticket description.

```markdown
## Standing rules

- The shared surface already exists on this branch. Read it first, and call into it. Do not rename anything in it, change a declared signature, or add a file to a shared package.
- Do not write fallback wiring. If an import, symbol, or field named in the Contract block is missing, that is a defect to report, not a condition to handle. No conditional import with a stub fallback, no dynamic lookup with a default value for a declared field, no untyped placeholder where the Contract names a type. A wiring fault must break the build.
- Do not re-declare a constant, timeout, limit, or default that the Contract already assigns to an owner. Read it from its owner.
- Mock only at process boundaries: the network, the clock, the data store, a third-party service. Never replace a first-party module with a fake. A test that stands in for a module will pass even when that module does not exist.
- Every file you change must appear in this ticket's Files block. If you need a change outside it, report it rather than making it.
- If the Contract is wrong or incomplete, follow the "Reporting a contract defect" steps below.

## Reporting a contract defect

1. Do not rename the symbol, declare your own version, or work around the gap.
2. Implement every part of this ticket that does not depend on the defect, with its tests, and commit that.
3. Begin the commit message subject with `CONTRACT:` and state, in the body, the reference you expected, what you found instead, and which acceptance criteria you could not complete.
4. Record it so later tickets read it before they start:
   `cf-memory add --scope <parent project identifier> --kind gotcha --source <this ticket identifier> "<one sentence naming the expected reference and the actual one>"`
5. Leave the unfinished criteria unchecked. The ticket then stops at its user-review, which is where a person decides whether to fix the contract or change this ticket.
```

---

## The integration ticket

Create one ticket that depends on every other ticket in the project. It writes no features. It must:

- Load every process entry point and every module for real.
- Start the application the way production starts it: the real entry-point command, the real configuration, the real container or process manifest.
- Run the whole test suite with nothing first-party mocked.
- Fail loudly on any unimplemented body left over from the scaffold.

This makes an integration fault visible inside the project rather than after the last merge.

---

## Validation

Three checks. Each finds a different fault.

**Check 1 — write-scope overlap (merge safety).** For every pair of work units with no dependency relationship between them, verify their `write_scope` entries do not overlap. An empty `write_scope` on a work unit that creates or changes files is a bug; fill it in before checking. Fix an overlap by one of:
- **Stack (chain) them** — add a dependency so they run serially. Prefer this when both genuinely need the same files and extracting a shared piece would be artificial (e.g. "add field X to the model", then "add field Y to the same model").
- **Extract a shared ticket** — move the common code into a ticket both depend on. Prefer this when the overlap is a distinct, reusable piece of infrastructure; in most cases it belongs in the scaffold instead.
- **Add a dependency** — if one work unit consumes the other's output, make that explicit.

**Check 2 — contract ownership (duplication safety).** Walk the contract list and verify every concept has exactly one owner. Then walk every ticket description and verify that each concept it uses is copied in with the owner's exact reference. A concept with two owners, or a ticket that describes a concept without naming its reference, will be built twice. Check 1 cannot detect this, because duplicates in different files do not overlap.

**Check 3 — reachability.** For every ticket, list what it calls from other work units. Verify each of those is owned by the scaffold, by an existing repo package, or by a work unit in this ticket's dependency chain. Merged work becomes visible only at the next rebase onto the shared branch, so if the owner is not a dependency, the calling ticket may start first and find nothing.

The three checks test the plan against itself. The build is what tests the plan against reality: the scaffold's contract test fails if any contract symbol is missing or misnamed, and the scaffold's user-review is the last point before the other tickets unblock.

---

## How to divide the project

Each subproject delivers one behaviour end to end, or lays groundwork that several later tickets need. Each PRD must be small enough to read and work on in a single context window.

**Slice vertically, not by layer.** Splitting by layer or by deployable unit makes the seam between layers the deliverable, and no ticket owns a seam. Every agent then has a neighbour whose names it must guess. A slice like "handle one event type from intake to storage" crosses several layers but has few edges to other slices. The scaffold ticket is the deliberate exception: it is the layer everyone shares, and it merges before anything else runs.

**Keep the parallel width small.** Parallel width should not exceed the number of slices that share no unfinished surface once the scaffold merges. Two truly independent slices are worth more than four overlapping ones, because overlap is paid back with interest at merge time.

If a subproject covers a very broad feature, nest further subprojects under it with their own PRDs. Nesting organises PRDs only; it does not affect branches, because every work unit sets `parent_branch` to the top-level project. If you have gone deeper than three levels (including the top-level project), the slices are too fine.

## How to choose project dependencies

Parallelize where you can, but treat a dependency as a mechanism rather than overhead: it is the only thing that makes one work unit's code visible to another. Any shared code, library, or foundational behaviour MUST be a dependency of everything that uses it. A work unit may rely only on functionality implemented by work units in its `dependencies` list, or by work units those dependencies themselves depend on.

If you find a circular dependency, split the common functionality into a work unit that runs first and have both sides depend on it. In most cases that common functionality belongs in the scaffold ticket.

## How to order the work units

Create a work unit before you create anything that depends on it or is a child of it. Order: top-level project, scaffold ticket, then subprojects and tickets in dependency order, then the integration ticket last.

### How to ask clarifying questions

Ask only critical questions where the specification is ambiguous. If the work is clear, or if one answer is much more likely than the rest, don't ask. Focus on these points:

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

A PRD should be in Markdown format. It must have these sections.

### 1. Introduction/Overview
Brief description of the feature and the problem it solves.

### 2. Goals
Specific, measurable objectives (bullet list).

### 3. Contract
The rows of the contract list this subproject uses, copied exactly. Also name anything this subproject owns that others will call.

### 4. User Stories
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
- [ ] Calls only the references listed in the Contract; adds no new shared module
- [ ] Build, typecheck, and lint pass
- [ ] **[UI stories only]** Verify in browser using dev-browser skill
```

Acceptance criteria must be verifiable. "Works correctly" is bad. "Shows a confirmation dialog before deleting" is good.

### 5. Functional Requirements
Numbered list of specific functionalities:
- "FR-1: The system must allow users to..."
- "FR-2: When a user clicks X, the system must..."

Be explicit and unambiguous. Each requirement must name the file it affects and the contract references it uses (e.g. "FR-1: In `internal/models/priority.go`, implement `ParsePriority` as declared by the scaffold"). This is what keeps write scopes correct and non-overlapping.

### 6. Non-Goals (Out of Scope)
What this feature will NOT include. Critical for managing scope.

### 7. Design Considerations (Optional)
- UI/UX requirements
- Link to mockups if available
- Existing components to reuse, by name

### 8. Technical Considerations (Optional)
- Known constraints or dependencies
- Integration points with existing systems
- Performance requirements

### 9. Success Metrics
How will success be measured?

---

### Writing for Junior Developers

The PRD reader may be a junior developer or an AI agent. Therefore:

- Be explicit and unambiguous
- Avoid jargon or explain it
- Give enough detail to understand purpose and core logic
- Number requirements for easy reference
- Use concrete examples
- Name real files, real symbols, and real references. Never write "a helper for X" when you can write the path and the signature.

---

### Example PRD

```markdown
# PRD: Task Priority System

## Introduction

Add priority levels to tasks so users can focus on what matters most. Tasks can be marked as high, medium, or low priority, with visual indicators and filtering.

## Goals

- Allow assigning priority (high/medium/low) to any task
- Provide clear visual differentiation between priority levels
- Enable filtering and sorting by priority
- Default new tasks to medium priority

## Contract

| Concept | Owner | Reference | Shape |
|---|---|---|---|
| priority type | `task-priority/scaffold` | `internal/models.Priority` | string enum: `high`, `medium`, `low` |
| default priority | `task-priority/scaffold` | `internal/models.DefaultPriority` | `Priority` constant, value `medium` |
| badge component | existing repo | `web/src/ui/Badge.tsx` → `Badge` | props `{ level, label }` |

This subproject owns nothing that other subprojects call.

## User Stories

### US-001: Store task priority
**Description:** As a developer, I need to store task priority so it persists across sessions.

**Acceptance Criteria:**
- [ ] Adds a `priority` column to the tasks table, typed by `models.Priority`, defaulting to `models.DefaultPriority`
- [ ] Migration runs successfully and is reversible
- [ ] Reads the default from `internal/models`; declares no second default
- [ ] Build, typecheck, and lint pass

### US-002: Display priority indicator on task cards
**Description:** As a user, I want to see task priority at a glance so I know what needs attention first.

**Acceptance Criteria:**
- [ ] Each task card shows a colored priority badge (red=high, yellow=medium, gray=low)
- [ ] Uses the existing `Badge` component; adds no new badge component
- [ ] Priority is visible without hovering or clicking
- [ ] Build, typecheck, and lint pass
- [ ] Verify in browser using dev-browser skill

### US-003: Filter tasks by priority
**Description:** As a user, I want to filter the task list to see only high-priority items.

**Acceptance Criteria:**
- [ ] Filter dropdown with options All | High | Medium | Low, built from `models.Priority`
- [ ] Filter state persists in URL params
- [ ] Empty state message when no tasks match
- [ ] Build, typecheck, and lint pass
- [ ] Verify in browser using dev-browser skill

## Functional Requirements

- FR-1: In `internal/db/migrations/`, add the `priority` column, typed by `models.Priority`
- FR-2: In `web/src/ui/TaskCard.tsx`, render `Badge` for the task's priority
- FR-3: In `web/src/ui/TaskList.tsx`, add the priority filter dropdown
- FR-4: In `web/src/ui/TaskList.tsx`, sort by priority within each status column

## Non-Goals

- No priority-based notifications or reminders
- No automatic priority assignment based on due date
- No priority inheritance for subtasks

## Success Metrics

- Users can change priority in under 2 clicks
- No regression in task list performance
```

---

### Checklist

Before you finish:

- [ ] Utility inventory written, including build, lint, and test commands (Step 3)
- [ ] Asked clarifying questions with lettered options, and incorporated the answers
- [ ] Contract list complete, with exactly one owner per concept
- [ ] Scaffold ticket created first, with exact declarations, a contract test, and a smoke test per entry point
- [ ] Every work unit sets `parent_branch` to the top-level project's branch
- [ ] Every implementation ticket depends on the scaffold and carries the Contract, Files, and Standing rules blocks
- [ ] Integration ticket created last, depending on everything
- [ ] Check 1 (write-scope overlap) passes
- [ ] Check 2 (contract ownership) passes
- [ ] Check 3 (reachability) passes
- [ ] Told the user that the scaffold's user-review is the contract gate, and included the contract table
