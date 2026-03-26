---
name: cf-project
description: "Break a large project description down into a series of smaller feature projects. Use when starting a new project. Triggers on: create a project, write project for, plan this project, requirements for, spec out, /cf-project."
user-invocable: true
---

# Projects Planner

Takes a specification for a large proposed change and breaks it down into a series of Code Factory projects, each containing detailed Code Factory tickets that are clear, actionable, and suitable for implementation.

---

**FIXME FIXME FIXME**:
- [ ] Make priority more flexible. Projects should be able to be worked on in parallel when they don't depend on each other.
- [ ] Tickets should contain the project context somehow. Do we duplicate it, or do we have `code-factory` read context from the parent projects as well? I strongly prefer the latter.
- [ ] Go over everything, ensure that terminology is correct and we're not mis-using "project".
- [ ] Ensure all references to "grundor" are removed.

## The Job

1. Read a user-provided file containing a specification for the work to be done
2. Divide the work into logical phases, each with a short descriptive name (preferably less than 30 characters). Each of these will become a Code Factory project.
3. Order the phases in order of priority. A phase should never depend on work in a later phase.
4. For each phase:
  4a. Prompt the user with up to 5 essential clarifying questions (with lettered options)
  4b. Generate a structured PRD based on answers
  4c. Create a project with `tickets create-project <project-name>`, piping the PRD into its standard input.
  4d. For each user story in the project, create a ticket for it with `tickets

**Important:** Do NOT start implementing. Just create the PRDs in the `grundor/` directory.

---

## How to Divide the Project

Each phase should either describe an individual feature of the project or lay necessary groundwork for implementing future features. Each output PRD must be small enough that it's easily read and worked on in a single context window. If the phase encompasses a very broad, general feature, you may break that phase into separate phases with their own short descriptive names and PRDs.

## How to Order the Project

* Each phase has a unique number from 1..(total number of phases) indicating the order they should be implemented in.
* Each phase must be independent — no phase may depend on functionality implemented by a later phase. If you encounter a circular dependency situation, resolve it by splitting out common functionality into a separate phase that's implemented first, then renumber as needed.

## How to Generate a PRD

### Step 1: Ask Clarifying Questions

Ask only critical questions where the initial prompt is ambiguous. If the work to be done is clear, don't ask any questions. Focus on:

- **Problem/Goal:** What problem does this solve?
- **Core Functionality:** What are the key actions?
- **Scope/Boundaries:** What should it NOT do?
- **Success Criteria:** How do we know it's done?

Don't put the questions in your output. Prompt the user directly for an answer before project context or tickets are generated.

#### Format Questions Like This:

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

---

### Step 2: Generate Structured PRD

Generate the PRD with these sections:

#### 1. Introduction/Overview
Brief description of the feature and the problem it solves.

#### 2. Goals
Specific, measurable objectives (bullet list).

#### 3. User Stories
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

#### 4. Functional Requirements
Numbered list of specific functionalities:
- "FR-1: The system must allow users to..."
- "FR-2: When a user clicks X, the system must..."

Be explicit and unambiguous.

#### 5. Non-Goals (Out of Scope)
What this feature will NOT include. Critical for managing scope.

#### 6. Design Considerations (Optional)
- UI/UX requirements
- Link to mockups if available
- Relevant existing components to reuse

#### 7. Technical Considerations (Optional)
- Known constraints or dependencies
- Integration points with existing systems
- Performance requirements

#### 8. Success Metrics
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
- [ ] Saved to `grundor/prd-[NN]-[feature-name].md`
