---
name: cf-review
description: Thoroughly review a changeset from a variety of different perspectives. Trigger on `/review` or `/cf-review`.
user-invocable: true
---

# Code Review

We need to carefully review the changes on this branch to ensure that they meet our high standards for security, performance, readability, and reliability.

Trigger on `/review` or `/cf-review`.

## Prerequisites

Before starting, run through ALL of the following steps in order:

1. **Find the ticket identifier**: It should have been specified as part of the skill invocation (e.g., `/review TICKET-123`). If not provided, ask the user for it before proceeding. Store this as `TICKET_ID`.
2. **You must be on a branch**: Run `git branch --show-current`. If the result is `master` or `main`, tell the user "You must be on a feature branch to run a review" and stop.
3. **Identify default branch**: Detect the repo's default branch (main or master):
   ```bash
   git branch -l main master --format='%(refname:short)' | head -1
   ```
   Store the output as `DEFAULT_BRANCH`.
4. **Identify branchpoint**: Locate the commit where this branch diverged from the default branch. Replace `DEFAULT_BRANCH` with the value from step 3 — you MUST prepend `origin/` as shown:
   ```bash
   git merge-base origin/DEFAULT_BRANCH HEAD
   ```
   Store the output as `BRANCHPOINT`.
5. **Changes must exist on this branch**: Replace `BRANCHPOINT` with the value from step 4:
   ```bash
   git diff --stat BRANCHPOINT
   ```
   If there's no output, tell the user "No changes found on this branch" and stop.
6. **What does it do?**: Examine the commit messages between HEAD and BRANCHPOINT and generate a summary of what you think the branch does. Store the result as `PURPOSE`.

## Running the Phases

Launch **all five phases in parallel** as separate Task subagents. They are independent — they all take the same `BRANCHPOINT` and `PURPOSE` inputs and produce the same JSON output format.

In every subagent prompt below, replace `BRANCHPOINT` with the value from prerequisite step 4, and `PURPOSE` with the value from prerequisite step 6. These are literal text substitutions — the subagent receives the actual values, not the placeholder names.

### Phase 1: Fitness for purpose

Spawn a **Task subagent** with:
  - `description`: "Review branch for fitness for purpose"
  - `subagent_type`: `generalPurpose`
  - `prompt`: the text between ---BEGIN PROMPT--- and ---END PROMPT--- below, with BRANCHPOINT and PURPOSE substituted:

---BEGIN PROMPT---
You are an experienced senior software developer, and you're reviewing changes on a branch which you've never seen before. The author describes the purpose of this branch as: "PURPOSE"

Run `git diff BRANCHPOINT` to examine the changes. Decide whether the code changes actually fulfill the stated purpose. (Whitespace changes are an exception -- we can include whitespace changes in any unrelated PR.)

Use the Unblocked MCP (`unblocked_context_engine` tool) to gather context about the modified code.

If there are changes that look unrelated to the purpose, or there are changes that don't make sense to you, describe up to five of them in Markdown format. You MUST include a guess at what you think the unrelated change is trying to do. ONLY describe changes that are unclear or don't make sense.

Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual finding; do not include reasoning, analysis notes, or confirmations inside the JSON.

Each object must have exactly these three keys:
- "filename": the file path
- "line_number": the line number as a string
- "description": the issue text, prefixed with **Fitness for purpose:**

Example: [{"filename": "cmd/main.go", "line_number": "42", "description": "**Fitness for purpose:** This change adds logging unrelated to the stated goal of fixing auth"}]
---END PROMPT---

**Post-processing (parent agent does this, not the subagent):** Parse the JSON array returned by the subagent. For each object in the array, pipe the `description` value into standard input of:

    tickets add-change-request TICKET_ID FILENAME:LINE_NUMBER cf-review

Replace `TICKET_ID` with the value from prerequisite step 1. Replace `FILENAME` and `LINE_NUMBER` with values from the JSON object.

### Phase 2: Security review

Spawn a **Task subagent** with:
  - `description`: "Security review for current branch"
  - `subagent_type`: `generalPurpose`
  - `prompt`: the text between ---BEGIN PROMPT--- and ---END PROMPT--- below, with BRANCHPOINT substituted:

---BEGIN PROMPT---
You are an experienced, insightful security researcher, and you're hunting for security holes in the recent changelog of this codebase.

Run `git diff BRANCHPOINT` to examine the changes. Determine whether they potentially introduce any security holes that an attacker could use to leak data, impersonate users, escalate privileges, run untrusted commands, or cause any other unwanted security violation.

Use the Unblocked MCP (`unblocked_context_engine` tool) to gather information about the codebase.

If you find anything potentially worrying, describe it in Markdown format. You MUST include an explanation of why it worries you and a description of how an attacker could make use of it.

Only report an issue if you can describe a concrete, realistic attack: what specific input or action the attacker takes, what system boundary they cross, and what they gain. Speculative chains of events ("could potentially lead to...") do not qualify.

Code quality concerns, error handling gaps, and reliability issues are NOT security findings unless they directly enable one of the listed violations with no additional assumptions.

For each candidate finding, argue against it: what would have to be true for this to actually be exploitable? If the preconditions are implausible or outside an attacker's control, discard it.

Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual finding; do not include reasoning, analysis notes, or confirmations inside the JSON.

Each object must have exactly these three keys:
- "filename": the file path
- "line_number": the line number as a string
- "description": the issue text, prefixed with **Security:**

Example: [{"filename": "api/auth.go", "line_number": "87", "description": "**Security:** User-supplied input is passed directly to SQL query without parameterization"}]
---END PROMPT---

**Post-processing (parent agent does this, not the subagent):** Parse the JSON array returned by the subagent. For each object, pipe `description` into standard input of:

    tickets add-change-request TICKET_ID FILENAME:LINE_NUMBER cf-review

### Phase 3: Performance analysis

Spawn a **Task subagent** with:
  - `description`: "Performance analysis for current branch"
  - `subagent_type`: `generalPurpose`
  - `prompt`: the text between ---BEGIN PROMPT--- and ---END PROMPT--- below, with BRANCHPOINT substituted:

---BEGIN PROMPT---
You are an experienced, insightful senior developer, and you're examining the changes on this branch to determine if they could introduce performance issues.

Run `git diff BRANCHPOINT` to examine the changes. Determine whether they potentially introduce any performance issues that would be significant enough for a human to notice.

In particular, look out for:
- Inefficient procedures (O(n^2) algorithms, duplicated work, etc.)
- Database queries that are likely to be problematic
- Excessive queries to databases or APIs which could be batched into fewer queries
- Memory leaks, or large memory allocations that don't get freed by garbage collection
- Anything else that could be unexpectedly slow or resource-intensive

Use the Unblocked MCP (`unblocked_context_engine` tool) to gather information about the codebase. Assume that this code is for a large, busy application with thousands of users and billions of total database records.

If you find anything potentially worrying, describe it in Markdown format. You MUST include an explanation of why it worries you, and the details of a scenario where it could cause a problem.

Only report an issue if you can describe a concrete, realistic performance issue: what specific input would cause the problem, what would make it slow, and whether the impact would be severe enough that a human could notice it. Speculative chains of events ("could potentially lead to...") do not qualify.

For each candidate finding, argue against it: what would have to be true for this to cause performance issues in production? If the preconditions are implausible, discard it.

Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual finding; do not include reasoning, analysis notes, or confirmations inside the JSON.

Each object must have exactly these three keys:
- "filename": the file path
- "line_number": the line number as a string
- "description": the issue text, prefixed with **Performance:**

Example: [{"filename": "db/queries.go", "line_number": "112", "description": "**Performance:** This loop issues one SELECT per item; with 10k items this will take minutes. Batch into a single IN query."}]
---END PROMPT---

**Post-processing (parent agent does this, not the subagent):** Parse the JSON array returned by the subagent. For each object, pipe `description` into standard input of:

    tickets add-change-request TICKET_ID FILENAME:LINE_NUMBER cf-review

### Phase 4: Code quality

Spawn a **Task subagent** with:
  - `description`: "Code quality analysis for current branch"
  - `subagent_type`: `generalPurpose`
  - `prompt`: the text between ---BEGIN PROMPT--- and ---END PROMPT--- below, with BRANCHPOINT substituted:

---BEGIN PROMPT---
You are an experienced, insightful senior developer, and you're inspecting the changes on this branch to find code quality issues. You want to use this as a teaching opportunity for the code author.

Run `git diff BRANCHPOINT` to examine the changes. Determine whether the code quality is up to our high standards. Particular areas to consider include, but are not limited to:

- Unclear code, where the intent isn't obvious from the code itself and no comments explain it
- Useless comments that just explain what the code is doing without explaining why
- Misspellings in variable names or comments
- Excessively large modules, or modules with more than one responsibility
- Repetitive changes that could be abstracted into a helper class or method
- Duplicated code — does the codebase already have a way to do this that we're not using?
- Ignoring established patterns that already exist in the codebase
- Places where necessary error handling is missing, or errors are being unnecessarily suppressed

Use the Unblocked MCP (`unblocked_context_engine` tool) to gather information about the codebase.

If you find anything potentially worrying, describe it in Markdown format. You MUST include an explanation of why it worries you, and a suggestion for how to improve that code.

Only report an issue if you can describe a concrete, realistic code quality issue. For each candidate finding, argue against it: what would have to be true for this to be a deliberate choice by a reasonable developer? If the preconditions are implausible, discard it.

Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual finding; do not include reasoning, analysis notes, or confirmations inside the JSON.

Each object must have exactly these three keys:
- "filename": the file path
- "line_number": the line number as a string
- "description": the issue text, prefixed with **Code quality:**

Example: [{"filename": "internal/service.go", "line_number": "55", "description": "**Code quality:** This error is silently swallowed; callers have no way to know the operation failed"}]
---END PROMPT---

**Post-processing (parent agent does this, not the subagent):** Parse the JSON array returned by the subagent. For each object, pipe `description` into standard input of:

    tickets add-change-request TICKET_ID FILENAME:LINE_NUMBER cf-review

### Phase 5: Abstraction

Spawn a **Task subagent** with:
  - `description`: "Abstraction analysis for current branch"
  - `subagent_type`: `generalPurpose`
  - `prompt`: the text between ---BEGIN PROMPT--- and ---END PROMPT--- below, with BRANCHPOINT substituted:

---BEGIN PROMPT---
You are an experienced, insightful senior developer, and you're inspecting the changes on this branch to decide whether the abstractions it uses make sense. You want to use this as a teaching opportunity for the code author.

Run `git diff BRANCHPOINT` to examine the changes. For each change, consider:

- Is this the right place for this change to live?
- Does this code own the data that it's modifying?
- If we're passing parameters, would it make more sense for them to be member data of the object that's being operated on instead?
- Does this change increase the number of files we'll have to touch the next time this functionality has to change?
- Is there a deeper structural change we could make that would make this change simpler?

Use the Unblocked MCP (`unblocked_context_engine` tool) to gather information about the codebase.

If you find anything potentially worrying, describe it in Markdown format. You MUST include an explanation of why it worries you, and a suggestion for how to improve that code.

Only report an issue if you can describe a concrete, realistic change that would improve this code. For each candidate finding, argue against it: what would have to be true for this to be a deliberate choice by a reasonable developer? If the preconditions are implausible, discard it.

Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual finding; do not include reasoning, analysis notes, or confirmations inside the JSON.

Each object must have exactly these three keys:
- "filename": the file path
- "line_number": the line number as a string
- "description": the issue text, prefixed with **Abstraction:**

Example: [{"filename": "internal/handler.go", "line_number": "30", "description": "**Abstraction:** This formatting logic doesn't belong in the HTTP handler; extract it to the model layer so it can be reused"}]
---END PROMPT---

**Post-processing (parent agent does this, not the subagent):** Abstraction findings are structural suggestions, so they are filed as **change requests**. Parse the JSON array returned by the subagent. For each object, pipe `description` into standard input of:

    tickets add-change-request TICKET_ID FILENAME:LINE_NUMBER cf-review

## Completion

After all five phases have finished and their results have been posted to the ticket:

1. Count the total number of findings across all phases.
2. Report a summary to the user: how many findings per category (fitness for purpose, security, performance, code quality, abstraction), and the total.
3. If the total is zero, tell the user: "No issues found — the branch looks good."
