---
name: cf-review
description: Thoroughly review a changeset from a variety of different perspectives. Trigger on `/review` or `/cf-review`.
---

# Code Review

We need to carefully review the changes on this branch to ensure that they meet our high standards for security, performance, readability, and reliability.

Trigger on `/review` or `/cf-review`.

## Prerequisites

Before starting, run through ALL of the following steps:

1. **Find the identifier of the ticket this is for**: It should have been specified as part of the skill invocation.
2. **You must be on a branch**: if the current branch is `master` or `main`, complain and exit.
3. **Identify default branch**: detect the repo's default branch (main or master):
   ```bash
   git branch -l main master --format='%(refname:short)' | head -1
   ```
   Store this as `DEFAULT_BRANCH`.
4. **Identify branchpoint**: locate the commit that this branch is based off of:
   ```bash
   git merge-base origin/<DEFAULT_BRANCH> HEAD
   ```
   Substitute the value from step 2 for `<DEFAULT_BRANCH>`. You MUST ensure that `origin/` is prepended to the branch name as shown in the command above. Store the output as `BRANCHPOINT`.
5. **Changes must exist on this branch**:
   ```bash
   git diff --stat <BRANCHPOINT>
   ```
   Substitute the value from step 3 for `<BRANCHPOINT>`. If there's no output, complain and exit.
6. **What does it do?**: Examine the commit messages between HEAD and BRANCHPOINT and generate a summary of what you think the branch does. Ask the user whether your guess is correct; if not, prompt them to provide a short description of the purpose of these changes. Store the output as `PURPOSE`.

Then run the following phases:

## Phase 1: Fitness for purpose

Spawn a **Task subagent** with:
  - `description`: "Review current branch for fitness for purpose"
  - `subagent_type`: "Explore"
  - `prompt`: ```
    You are an experienced senior software developer, and you're reviewing changes on a branch which you've never seen before. The author describes the purpose of this branch as "<PURPOSE>".

    Examine the changes with `git diff <BRANCHPOINT>` and decide whether it looks like these code changes actually fulfill the stated purpose.

    Use the Unblocked MCP to gather context about the modified code.

    If there are changes that look unrelated to the purpose, or there are changes that don't make sense to you, describe up to five of them in Markdown format. You MUST include a guess at what you think the unrelated change is trying to do. ONLY describe changes that are unclear or don't make sense.

    **Output format:**
    ```json
    [
      // Include one entry like this for each change that worries you:
      {
        "filename": "<filename>",
        "line_number": "<line number>",
        "description": "**Fitness for purpose:** <text of issue in Markdown format>"
      }
    ]
    ```

    **CRITICAL: Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual complaint; do not include reasoning, analysis notes, or confirmations inside the JSON.**
    ```

For each issue found by this step, run `tickets add-comment <ticket-identifier> <filename>:<line_number> cf-review` and pipe `description` into `tickets`' standard input.

## Phase 2: Security review

Spawn a **Task subagent** with:
  - `description`: "Security review for current branch"
  - `subagent_type`: "Explore"
  - `prompt`: ```
    You are an experienced, insightful security researcher, and you're hunting for security holes in the recent changelog of this codebase.

    Examine the changes on this branch with `git diff <BRANCHPOINT>` and determine whether they potentially introduce any security holes that an attacker could use to leak data, impersonate users, escalate privileges, run untrusted commands, or cause any other unwanted security violation.

    Use the Unblocked MCP to gather information about the codebase.

    If you find anything potentially worrying, describe it in Markdown format. You MUST include an explanation of why it worries you and a description of how an attacker could make use of it.

    Only report an issue if you can describe a concrete, realistic attack: what specific input or action the attacker takes, what system boundary they cross, and what they gain. Speculative chains of events ("could potentially lead to...") do not qualify.

    Code quality concerns, error handling gaps, and reliability issues are NOT security findings unless they directly enable one of the listed violations with no additional assumptions.

    For each candidate finding, argue against it: what would have to be true for this to actually be exploitable? If the preconditions are implausible or outside an attacker's control, discard it.

    **Output format:**
    ```json
    [
      // Include one entry like this for each change that worries you:
      {
        "filename": "<filename>",
        "line_number": "<line number>",
        "description": "**Security:** <text of issue in Markdown format>"
      }
    ]
    ```

    **CRITICAL: Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual complaint; do not include reasoning, analysis notes, or confirmations inside the JSON.**
    ```

For each issue found by this step, run `tickets add-comment <ticket-identifier> <filename>:<line_number> cf-review` and pipe `description` into `tickets`' standard input.

## Phase 3: Performance analysis

Spawn a **Task subagent** with:
  - `description`: "Performance analysis for current branch"
  - `subagent_type`: "Explore"
  - `prompt`: ```
    You are an experienced, insightful senior developer, and you're examining the changes on this branch to determine if they could introduce performance issues.

    Examine the changes on this branch with `git diff <BRANCHPOINT>` and determine whether they potentially introduce any performance issues that would be significant enough for a human to notice.

    In particular, look out for:
    - Inefficient procedures (O(n^2) algorithms, duplicated work, etc.)
    - Database queries that are likely to be problematic
    - Excessive queries to databases or APIs which could be batched into fewer queries
    - Memory leaks, or large memory allocations that don't get freed by garbage collection
    - Anything else that could be unexpectedly slow or resource-intensive

    Use the Unblocked MCP to gather information about the codebase. Assume that this code is for a large, busy application with thousands of users and billions of total database records.

    If you find anything potentially worrying, describe it in Markdown format. You MUST include an explanation of why it worries you, and the details of a scenario where it could cause a problem.

    Only report an issue if you can describe a concrete, realistic performance issue: what specific input would cause the problem, what would make it slow, and whether the impact would be severe enough that a human could notice it. Speculative chains of events ("could potentially lead to...") do not qualify.

    For each candidate finding, argue against it: what would have to be true for this to cause performance issues in production? If the preconditions are implausible, discard it.

    **Output format:**
    ```json
    [
      // Include one entry like this for each change that worries you:
      {
        "filename": "<filename>",
        "line_number": "<line number>",
        "description": "**Performance:** <text of issue in Markdown format>"
      }
    ]
    ```

    **CRITICAL: Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual complaint; do not include reasoning, analysis notes, or confirmations inside the JSON.**
    ```

For each issue found by this step, run `tickets add-comment <ticket-identifier> <filename>:<line_number> cf-review` and pipe `description` into `tickets`' standard input.

## Phase 4: Code quality

Spawn a **Task subagent** with:
  - `description`: "Code quality analysis for current branch"
  - `subagent_type`: "Explore"
  - `prompt`: ```
    You are an experienced, insightful senior developer, and you're inspecting the changes on this branch to find code quality issues. You want to use this as a teaching opportunity for the code author.

    Examine the changes on this branch with `git diff <BRANCHPOINT>` and determine whether the code quality is up to our high standards. Particular areas to consider include, but are not limited to:

    - Unclear code, where the intent isn't obvious from the code itself and no comments explain it
    - Useless comments that just explain what the code is doing without explaining why
    - Misspellings in variable names or comments
    - Excessively large modules, or modules with more than one responsibility
    - Repetitive changes that could be abstracted into a helper class or method
    - Duplicated code — does the codebase already have a way to do this that we're not using?
    - Ignoring established patterns that already exist in the codebase
    - Places where necessary error handling is missing, or errors are being unnecessarily suppressed

    Use the Unblocked MCP to gather information about the codebase.

    If you find anything potentially worrying, describe it in Markdown format. You MUST include an explanation of why it worries you, and a suggestion for how to improve that code.

    Only report an issue if you can describe a concrete, realistic code quality issue. For each candidate finding, argue against it: what would have to be true for this to be a deliberate choice by a reasonable developer? If the preconditions are implausible, discard it.

    **Output format:**
    ```json
    [
      // Include one entry like this for each change that worries you:
      {
        "filename": "<filename>",
        "line_number": "<line number>",
        "description": "**Code quality:** <text of issue in Markdown format>"
      }
    ]
    ```

    **CRITICAL: Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual complaint; do not include reasoning, analysis notes, or confirmations inside the JSON.**
    ```

For each issue found by this step, run `tickets add-comment <ticket-identifier> <filename>:<line_number> cf-review` and pipe `description` into `tickets`' standard input.

## Phase 5: Abstraction

Spawn a **Task subagent** with:
  - `description`: "Code quality analysis for current branch"
  - `subagent_type`: "Explore"
  - `prompt`: ```
    You are an experienced, insightful senior developer, and you're inspecting the changes on this branch to decide whether the abstractions it uses make sense. You want to use this as a teaching opportunity for the code author.

    Examine the changes on this branch with `git diff <BRANCHPOINT>`. For each change, consider:

    - Is this the right place for this change to live?
    - Does this code own the data that it's modifying?
    - If we're passing parameters, would it make more sense for them to be member data of the object that's being operated on instead?
    - Does this change increase the number of files we'll have to touch the next time this functionality has to change?
    - Is there a deeper structural change we could make that would make this change simpler?

    Use the Unblocked MCP to gather information about the codebase.

    If you find anything potentially worrying, describe it in Markdown format. You MUST include an explanation of why it worries you, and a suggestion for how to improve that code.

    Only report an issue if you can describe a concrete, realistic change that would improve this code. For each candidate finding, argue against it: what would have to be true for this to be a deliberate choice by a reasonable developer? If the preconditions are implausible, discard it.

    **Output format:**
    ```json
    [
      // Include one entry like this for each change that worries you:
      {
        "filename": "<filename>",
        "line_number": "<line number>",
        "description": "**Abstraction:** <text of issue in Markdown format>"
      }
    ]
    ```

    **CRITICAL: Your entire response must be a valid JSON array and nothing else — no prose, no code fences, no explanation. Output [] if you found no issues. Every object in the array must be an actual complaint; do not include reasoning, analysis notes, or confirmations inside the JSON.**
    ```

For each issue found by this step, run `tickets add-change-request <ticket-identifier> <filename>:<line_number> cf-review` and pipe `description` into `tickets`' standard input.
