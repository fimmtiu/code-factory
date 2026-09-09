---
name: cf-major-refactor
description: "Refactor a codebase to try to minimize the amount of tokens and cognitive overhead required to work with it. Trigger with `/cf-major-refactor`."
---

# Major Refactoring

Code which is frequently modified by multiple agents is often verbose, token-heavy, and full of duplicated code. We want to refactor the current codebase such that, without changing behaviour at all, we reduce the number of tokens required for an agent to understand it and simplify the code to make it easier for humans to understand.

## Prerequisites

Run ALL of these steps before starting the refactoring loop. **If a "Pre-detected environment" block was provided in the prompt**, use its values for `DEFAULT_BRANCH`, `BRANCHPOINT`, `BUILD_CMD`, `TEST_CMD`, and `LINT_CMD` directly and skip the corresponding detection steps below. Only run a detection step if the value you need was not pre-detected.

1. **Branch check**: If the current branch is `main` or `master`, tell the user "You must be on a feature branch to refactor. Check out a branch and try again." and stop.

2. **Default branch detection** *(skip if pre-detected)*:
   ```bash
   git branch -l main master --format='%(refname:short)' | head -1
   ```
   Store the output as `DEFAULT_BRANCH`. If the output is empty (neither `main` nor `master` exists locally), ask the user: "What is the default branch name?" and use their answer.

3. **Detect project tooling** *(skip if pre-detected)*: Determine `BUILD_CMD`, `TEST_CMD`, and `LINT_CMD` by checking these sources in priority order:

   a. **Workspace rules** (CLAUDE.md, .cursorrules, AGENTS.md, etc.) — these take highest priority.
   b. **Makefile** — look for `build`, `test`, and `lint` targets. If found, use `make build` / `make test` / `make lint`.
   c. **package.json** — check `scripts.build`, `scripts.test`, `scripts.lint`. If found, use `npm run build` / `npm test` / `npm run lint`.
   d. **pyproject.toml / pytest.ini** — use `python -m py_compile` for build, `pytest` for tests, check for `ruff` or `flake8` for linting.
   e. **Cargo.toml** — use `cargo build` / `cargo test` / `cargo clippy`.
   f. **go.mod** — use `go build ./...` / `go test ./...` / `gofmt -w .` (or `make lint` if Makefile exists).

   If a command is not found for a given variable, set it to empty string. Print the detected commands.

## Determine Relevant Files

Filter to source code files only. Exclude:
- Binary files and images
- Lockfiles: `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `Cargo.lock`, `go.sum`, `Gemfile.lock`, `poetry.lock`, etc.
- Generated files: files with "generated" in their name or a generation header comment (e.g., `// Code generated ... DO NOT EDIT.`)
- Config/data-only files: `.json`, `.yaml`, `.yml`, `.toml`, `.xml`, `.env` (UNLESS they contain logic, e.g., `Makefile` is kept)
- Vendor/dependency directories: `vendor/`, `node_modules/`, `third_party/`

When in doubt about whether a file is source code, include it — it is better to scan unnecessarily than to miss code. Store the filtered list as `CHANGED_FILES`.

## Scope Rules

These rules are ABSOLUTE and apply to every phase:

1. **Write scope**: ONLY modify files that appear in `CHANGED_FILES`. When a refactoring creates a NEW file (e.g., Extract Class), add it to `CHANGED_FILES` for subsequent phases.
2. **Read scope**: You MAY read any file in the repository for context (understanding class hierarchies, finding callers, checking conventions).
3. **Deferred refactorings**: If a refactoring would require modifying files outside `CHANGED_FILES` (e.g., updating callers in untouched files), note it in the summary as a "deferred refactoring" but do NOT make the change.
4. **No backtracking**: Files created by a refactoring in phase N are considered "clean" for smells 1 through N. Do not re-check earlier smells on newly created files.
5. **Language-agnostic**: Adapt all treatments to the specific language and conventions of the project. The smell descriptions are language-agnostic concepts.

## Refactoring Loop

Perform each of the following refactor steps on the code.

If you break this task into multiple sub-agents for processing, you MUST:
* give each agent a non-overlapping subset of files to work on, to prevent conflicts
  - Break up the tasks by files, not by steps — each agent should run all steps
  - Include the tests for each code file in the subset of files to work on — each agent should be able to fix both the code and the tests
* instruct them not to use `git` commands which modify global repository state, like `stash`

### Step 1: Remove dead code

Is there any code that you can verify is never called? If so, remove it and any associated tests that are specific to the dead code.

### Step 2: Winnow tests

Are there any tests which test things that are no longer relevant? If so, delete the tests or merge them into existing tests.

Are there any tests in the same test file which are substantially identical, and which are testing similar or related concepts? If so, merge them.

### Step 3: Trim comments

Agent-generated code tends to be filled with verbose comments that describe WHAT the code does rather than WHY, include irrelevant historical information, and clutter up the codebase.

The only comments we want to preserve are:
* Those which explain, in as few words as possible, why the code exists or why it was implemented a certain way (business rules, workarounds, necessary historical context)
* Comments which document complex algorithms after all other simplification has been exhausted

Any comments which do not meet this standard should be edited or deleted outright.

#### Remove unnecessary historical context

Purely historical information which future developers won't care about should be deleted outright.

Example:
```python
# Slackbot (USLACKBOT) was the sole sender until Slack's 2026-06-17 breaking
# change (https://docs.slack.dev/changelog/2026/06/17/system-notifications/),
# which moved system-generated notifications — channel membership changes (e.g.
# "you were removed from #channel" when the bot is kicked), User Group updates,
# Slack Connect alerts, retention policy notices — onto a dedicated "Slack"
# system user (USLACK). USLACKBOT is NOT retired: user-authored DMs to Slackbot
# are explicitly unaffected, and older notices still reference it. So USLACK is
# an ADDITIONAL id to drop, not a replacement — hence a set of both rather than
# a swap. Nothing needs either id on its own, so the set is the only constant.
SYSTEM_USER_IDS: frozenset[str] = frozenset({"USLACKBOT", "USLACK"})
```
None of this is relevant to future changes — from now on, users only need to know that there are two `SYSTEM_USER_IDS`, not why or how it changed. The entire comment can be deleted.

#### Simplify comments

Comments should be as brief as possible without losing meaning. Edit verbose comments to be as simple as possible, trimming out information that's obvious from context and shortening verbose sentences.

Example of a verbose comment:
```python
# Bounded FETCH range for a no-explicit-range Sheets read — the Sheets analog of
# the Drive media lane's MAX_FILE_BYTES guard below. Without a bound,
# `spreadsheets.values.get` on a bare tab title pulls the WHOLE tab into memory
# before the render is trimmed to SHEET_MAX_CONTENT_CHARS — the same
# download-everything-then-truncate problem MAX_FILE_BYTES guards against, only
# for a native Sheet. But Google-native Sheets report NO byte size via Drive
# metadata (Drive `size` is 0 for native docs), so a literal byte pre-check like
# the media lane's isn't available; we translate the intent ("don't download an
# unbounded amount") into a cap on the number of CELLS fetched — bytes → bounded
# cell range. Generous enough to comfortably cover realistic tabs (the incident
# cell was C51); a tab larger than the cap is fetched only up to it and the
# render carries a truncation note. When gridProperties report the tab's true
# dims we request only up to those, so a small tab isn't padded to a needlessly
# large range.
SHEET_MAX_FETCH_ROWS: int = 5000
```

The same comment after a good edit:
```python
# Bounded FETCH range for a no-explicit-range Sheets read. We cap by the number of
# cells read because Sheets' metadata doesn't include byte size like Drive does.
SHEET_MAX_FETCH_ROWS: int = 5000
```

### Step 4: Simplify documentation

The same requirements apply to code-level documentation, such as Python doc strings at the top of classes and methods. Edit them to be simpler, and remove any text which is obvious from context or explains historical context that isn't required to understand the current code.

### Step 5: Report

Print a short summary of how many bytes changed in each file you edited.
