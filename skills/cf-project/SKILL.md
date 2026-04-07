---
name: cf-project
description: "Use when starting a new project that needs to be decomposed into smaller work units for parallel implementation. Triggers on: create a project, write project for, plan this project, requirements for, spec out, /cf-project."
user-invocable: true
---

# Projects Planner

Takes a specification for a large proposed change and breaks it down into a series of Code Factory projects, each containing detailed Code Factory tickets that are clear, actionable, and suitable for implementation.

Terminology for the `tickets` system:
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

1. Read the user-provided specification for the work to be done
2. Divide the work into logical pieces, each with a short identifier (preferably less than 30 characters, following the identifier rules above). Each of these will become a Code Factory project.
3. Collect all clarifying questions across all projects and present them to the user in a single batch (see "How to ask clarifying questions" below). Wait for answers before proceeding. If nothing is ambiguous, skip this step.
4. Determine the dependencies between projects
5. For each project, in dependency order (parents and dependencies first):
  5a. Generate a structured PRD, guided by the user's answers from Step 3
  5b. Create the project by piping a JSON description into `tickets create-project`:
  ```bash
  tickets create-project <project-identifier> <<'TICKET_JSON'
  {
    "dependencies": [
      "identifier-of-a-dependency",
      "another-dependency"
    ],
    "description": "Full PRD content here (see below for format)..."
  }
  TICKET_JSON
  ```
  Omit `dependencies` or pass `[]` if the project has none.
  5c. For each user story in the PRD, create a ticket. Derive the ticket identifier from the user story title, not its number — for a story titled "US-001: Add priority field" in project `task-priority`, the ticket identifier would be `task-priority/add-priority-field`.
  ```bash
  tickets create-ticket <project-identifier>/<ticket-name> <<'TICKET_JSON'
  {
    "dependencies": [
      "project-name/other-ticket"
    ],
    "description": "Full user story content here..."
  }
  TICKET_JSON
  ```

**Important:** Do NOT start implementing. Your job ends when all projects and tickets have been created with `tickets`. Do not write any code, modify any source files, or begin work on any ticket.

---

## How to divide the project

Each project should either describe an individual feature of the proposed change or lay necessary groundwork for implementing future features. Each output PRD must be small enough that it's easily read and worked on in a single context window. Tickets should not generate duplicate code -- if multiple tickets need a particular function, it should be extracted to a separate ticket that the others depend on. We want to minimize the chances of merge conflicts when we merge the tickets' output commits together.

If the project encompasses a very broad, general feature, you may break that project into multiple subprojects of that parent project with their own short descriptive names and PRDs. Subprojects may themselves contain subprojects. There are no hard limits on nesting, but if you've gone deeper than three levels of nesting then something is wrong and you're probably making the projects too fine-grained.

## How to choose project dependencies

You should aim to split up the work into projects such that we can parallelize as much work as possible. Any changes (libraries, shared code, foundational functionality, etc.) which will be used by other projects MUST be marked as a dependency for those subsequent projects. A work unit MUST ONLY depend on functionality implemented by work units that are in its `dependencies` list, or functionality implemented by work units that the work units in the `dependencies` list themselves depend on. (It's a tree; the work you rely on must be in one of the ancestors.)

If you encounter a circular dependency situation, resolve it by splitting out common functionality into a separate project that's implemented first. Other projects can depend on that common functionality instead of depending on each other.

## How to order the work units

The work units must be created in order such that you MUST create a work unit before you create any work units that depend on it, or which are children of it. Before creating tickets or projects, order them so that the parents and/or dependencies of each work unit occur earlier in the order.

### How to ask clarifying questions

Ask only critical questions where the initial prompt is ambiguous. If the work to be done is clear, or if one of the answers is much more likely than the rest, don't ask the question. Focus on making these points clear:

- **Problem/Goal:** What problem does this solve?
- **Core Functionality:** What are the key actions?
- **Scope/Boundaries:** What should it NOT do?
- **Success Criteria:** How do we know it's done?

NEVER put the questions in the output we send to `tickets`. Prompt the user directly for an answer before projects or tickets are generated.

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

Be explicit and unambiguous.

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
