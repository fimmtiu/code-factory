---
name: refactorizer
description: "Scan branch changes for code smells and aggressively refactor to remove them using Martin Fowler-style techniques. Trigger with `/refactorizer`."
---

# Refactorizer

Aggressively scan the changes on the current branch for code smells and refactor to remove them, applying Martin Fowler-style refactoring techniques.

Trigger with `/refactorizer`.

## Prerequisites

Run ALL of these steps before starting the refactoring loop:

1. **Branch check**: If the current branch is `main` or `master`, tell the user "You must be on a feature branch to refactor. Check out a branch and try again." and stop.

2. **Default branch detection**:
   ```bash
   git branch -l main master --format='%(refname:short)' | head -1
   ```
   Store the output as `DEFAULT_BRANCH`.

3. **Branchpoint identification**:
   ```bash
   git merge-base origin/<DEFAULT_BRANCH> HEAD
   ```
   Substitute the value from step 2 for `<DEFAULT_BRANCH>`. You MUST prepend `origin/` to the branch name. Store the output as `BRANCHPOINT`.

4. **Branch purpose**: Read the commit messages between `BRANCHPOINT` and `HEAD`:
   ```bash
   git log --oneline <BRANCHPOINT>..HEAD
   ```
   Generate a one-sentence summary of what this branch does. Store as `BRANCH_PURPOSE`. Print it so the user can see it.

5. **Detect project tooling**: Check for a test runner and linter:
   - Look for `package.json` (scripts.test, scripts.lint), `Makefile`, `pytest.ini`/`pyproject.toml`, `Cargo.toml`, etc.
   - Store the test command as `TEST_CMD` and linter command as `LINT_CMD`. If not found, set to empty.

## Find changes

If unspecified, prompt the user for what they want to refactor:
   1. All code in the repository (recurse over the filesystem to fetch the names of all files)
   2. All changes on this branch (use `git diff --stat <BRANCHPOINT>` to fetch the names of the changed files)
   3. Uncommitted changes only (use `git diff --stat` to fetch the names of the changed files)

Filter this list to source code files only — exclude binary files, images, lockfiles (package-lock.json, yarn.lock, Cargo.lock, etc.), generated files, and non-code config files. Store the filtered list as `CHANGED_FILES`.

## Scope Rules

These rules are ABSOLUTE and apply to every phase:

1. **Write scope**: ONLY modify files that appear in `CHANGED_FILES`. When a refactoring creates a NEW file (e.g., Extract Class), add it to `CHANGED_FILES` for subsequent phases.
2. **Read scope**: You MAY read any file in the repository for context (understanding class hierarchies, finding callers, checking conventions).
3. **Change focus**: Focus on code that was added or modified on this branch. Pre-existing code in unchanged sections should not be refactored UNLESS it is directly entangled with changed code (e.g., extracting a method from a function where new lines are intermixed with old).
4. **Deferred refactorings**: If a refactoring would require modifying files outside `CHANGED_FILES` (e.g., updating callers in untouched files), note it in the summary as a "deferred refactoring" but do NOT make the change.
5. **No backtracking**: Files created by a refactoring in phase N are considered "clean" for smells 1 through N. Do not re-check earlier smells on newly created files.
6. **Language-agnostic**: Adapt all treatments to the specific language and conventions of the project. The smell descriptions are language-agnostic concepts.

## Refactoring Loop

Process the 23 smells below IN ORDER, one at a time. For each smell, follow the Phase Template.

### Phase Template

For smell N of 23:

1. **Announce**: Print `[N/23] Scanning for <SMELL_NAME>...`

2. **Re-diff**: Get the current state of all changed files:
   ```bash
   git diff <BRANCHPOINT> -- <space-separated CHANGED_FILES>
   ```
   Also read the full content of each changed file to understand context.

3. **Detect**: Examine each changed file for instances of this smell, using the description and detection criteria from the catalog entry below.

4. **Filter**: For each instance found, check the "When to Ignore" criteria from the catalog entry. If an ignore criterion applies, skip this instance and record why.

5. **Refactor**: For each remaining instance, apply one or more of the listed treatments:
   - Be AGGRESSIVE: apply the refactoring unless there is a clear, specific reason not to.
   - Prefer the first listed treatment as the default choice.
   - Follow existing naming conventions in the codebase.
   - Update all references within `CHANGED_FILES` when moving/renaming.

6. **Verify**: After refactoring, ensure the code still parses. If `LINT_CMD` is set, run it on modified files and fix any issues.

7. **Report**: Print one of:
   - `[N/23] <SMELL_NAME>: Found <X> instances, refactored <Y>, ignored <Z> (<reasons>)`
   - `[N/23] <SMELL_NAME>: Clean`

---

## Smell Catalog

### Pass 1 — Dispensables

Remove noise, dead weight, and unnecessary abstractions first. This reduces the surface area for all subsequent analysis.

---

#### [1/23] Dead Code

**What to look for**: Variables, parameters, fields, methods, or classes that are no longer used — typically made obsolete by evolving requirements or bug fixes.

**Treatments**:
- Delete unused code entirely
- Inline Class or Collapse Hierarchy for unnecessary classes
- Remove Parameter for unneeded parameters

**When to ignore**: n/a — dead code should always be removed.

---

#### [2/23] Duplicate Code

**What to look for**: Two or more code fragments that look almost identical, or different-looking code that performs equivalent functions. Check within the same file, across changed files, and between sibling subclasses.

**Treatments**:
- Extract Method (same class)
- Pull Up Field / Pull Up Constructor Body (sibling subclasses)
- Form Template Method (subclasses with similar algorithms)
- Extract Superclass or Extract Class (unrelated classes)
- Consolidate Conditional Expression / Consolidate Duplicate Conditional Fragments

**When to ignore**: In rare cases where combining identical fragments would significantly reduce clarity. You must articulate the specific reason.

---

#### [3/23] Speculative Generality

**What to look for**: Unused abstract classes, unnecessary delegation, unused method parameters, or fields created "just in case" for anticipated future features that aren't being used.

**Treatments**:
- Collapse Hierarchy (unused abstract classes)
- Inline Class (unnecessary delegation)
- Inline Method (unused methods with no callers beyond one)
- Remove Parameter (unused parameters)
- Delete unused fields

**When to ignore**: If the code is part of a framework or library where external consumers may need the functionality. Verify elements aren't used only by unit tests before deleting.

---

#### [4/23] Lazy Class

**What to look for**: Classes that don't do enough to justify their existence — reduced to minimal functionality over time, or designed for future work that never materialized.

**Treatments**:
- Inline Class (merge its content into the class that uses it)
- Collapse Hierarchy (if it's an unnecessary level in an inheritance chain)

**When to ignore**: If the class intentionally marks planned future development AND there is evidence (comments, tickets, TODOs) of that intent.

---

#### [5/23] Comments (as code smell)

**What to look for**: Methods filled with explanatory comments that mask unclear code. Comments that describe WHAT the code does rather than WHY. Comments that could be replaced by clearer code structure.

**Treatments**:
- Extract Variable (give complex expressions a self-documenting name)
- Extract Method (turn commented code sections into well-named methods)
- Rename Method (make the method name describe its purpose)
- Introduce Assertion (replace comments about required state with actual assertions)

**When to ignore**: Comments that explain WHY something is implemented a particular way (business rules, workarounds, historical context). Comments documenting complex algorithms after all other simplification has been exhausted.

---

#### [6/23] Data Class

**What to look for**: Classes that contain only fields and crude getter/setter methods, functioning as passive data containers with no behavior operating on their own data.

**Treatments**:
- Encapsulate Field (make fields private, add accessors if not present)
- Encapsulate Collection (return read-only views for collection fields)
- Move Method / Extract Method (find operations on this class's data that live elsewhere and migrate them into the class)
- Remove Setting Method / Hide Method (restrict access after adding behavior)

**When to ignore**: n/a — data classes almost always benefit from encapsulation and behavior migration.

---

### Pass 2 — Bloaters

Shrink oversized constructs. These are the most common refactorings and create the building blocks that later passes need.

---

#### [7/23] Long Method

**What to look for**: Methods that are excessively long. As a rule of thumb, any method longer than ~10-15 lines should be examined. Look for methods with multiple responsibilities, deeply nested logic, or extensive inline comments explaining sections.

**Treatments**:
- Extract Method (the primary treatment — extract cohesive blocks into well-named methods)
- Replace Temp with Query (eliminate temporary variables that exist only to cache an expression)
- Introduce Parameter Object (when many parameters exist because the method does too much)
- Preserve Whole Object (pass an object instead of pulling several values from it)
- Replace Method with Method Object (when local variables prevent extraction — turn the method into its own class)
- Decompose Conditional (extract complex conditional logic into named methods)

**When to ignore**: n/a — long methods should always be decomposed.

---

#### [8/23] Large Class

**What to look for**: Classes with too many fields, methods, or lines of code. Classes that have accumulated responsibilities over time. Classes where some fields/methods are only used in certain scenarios.

**Treatments**:
- Extract Class (split responsibilities into separate classes)
- Extract Subclass (when features are used only in specific cases)
- Extract Interface (when a client only uses a subset of the class's interface)
- Duplicate Observed Data (for GUI-heavy classes, separate domain data from presentation)

**When to ignore**: n/a — large classes should be decomposed.

---

#### [9/23] Long Parameter List

**What to look for**: Methods with more than 3-4 parameters. Parameter lists that are hard to understand or easy to get wrong.

**Treatments**:
- Replace Parameter with Method Call (if a value can be obtained from an object the method already has access to)
- Preserve Whole Object (pass the object instead of extracting several values from it)
- Introduce Parameter Object (group related parameters into a new class)

**When to ignore**: When eliminating a parameter would create an unwanted dependency between classes. Prefer maintaining separation of concerns over shorter parameter lists.

---

#### [10/23] Primitive Obsession

**What to look for**: Use of primitives instead of small objects for simple tasks (money/currency, ranges, phone numbers, special strings for patterns). Constants used for encoding type information. String constants as field names.

**Treatments**:
- Replace Data Value with Object (wrap the primitive in a meaningful class)
- Introduce Parameter Object (group co-traveling primitives)
- Preserve Whole Object (pass the wrapper object)
- Replace Type Code with Class / Subclasses / State-Strategy
- Replace Array with Object (when arrays hold heterogeneous data)

**When to ignore**: n/a — primitives used to represent domain concepts should be wrapped.

---

#### [11/23] Data Clumps

**What to look for**: Identical groups of variables appearing together in multiple places — as fields in different classes, parameters in multiple method signatures, or local variables in several methods. If removing one variable would break the others' meaning, they're a clump.

**Treatments**:
- Extract Class (when the clump appears as fields in a class)
- Introduce Parameter Object (when the clump appears in method parameters)
- Preserve Whole Object (pass the containing object instead of its parts)

**When to ignore**: When passing the whole object would create an undesirable dependency between otherwise unrelated classes.

---

### Pass 3 — Object-Orientation Abusers

Fix misuse of OO features. Process these after Bloaters because Extract Method/Class from the previous pass may have surfaced new patterns.

---

#### [12/23] Switch Statements

**What to look for**: Complex switch/case statements or chains of if/else-if that dispatch on type codes or class names. Especially problematic when the same switch appears in multiple places.

**Treatments**:
- Extract Method + Move Method (isolate the switch and place it in the class that owns the data)
- Replace Type Code with Subclasses (create a subclass for each case)
- Replace Type Code with State/Strategy (when subclassing isn't possible)
- Replace Conditional with Polymorphism (the primary goal)
- Replace Parameter with Explicit Methods (when cases select very different behavior)
- Introduce Null Object (when one case is a null/empty check)

**When to ignore**: Simple switch statements with straightforward actions (no complex logic per case). Factory design patterns that use switch to select which class to instantiate.

---

#### [13/23] Temporary Field

**What to look for**: Object fields that are only populated under certain circumstances and sit empty the rest of the time. Often used to pass data between methods instead of using parameters.

**Treatments**:
- Extract Class (move the temporary field and all code that uses it into a new class)
- Introduce Null Object (provide a default object for the "empty" case)

**When to ignore**: n/a — temporary fields confuse readers and should be extracted.

---

#### [14/23] Refused Bequest

**What to look for**: Subclasses that use only some inherited methods/properties, or that override inherited methods to throw exceptions or do nothing. The inheritance hierarchy doesn't reflect a genuine "is-a" relationship.

**Treatments**:
- Replace Inheritance with Delegation (change from "is-a" to "has-a")
- Extract Superclass (when inheritance is appropriate but the hierarchy is wrong)

**When to ignore**: n/a — if a subclass doesn't want its parent's interface, delegation is more appropriate.

---

#### [15/23] Alternative Classes with Different Interfaces

**What to look for**: Two classes that perform identical or very similar functions but have different method names, parameter orders, or interfaces.

**Treatments**:
- Rename Methods to align interfaces
- Move Method / Add Parameter / Parameterize Method to make signatures consistent
- Extract Superclass (when only partial functionality overlaps)
- Delete the redundant class (when fully equivalent)

**When to ignore**: When the classes exist in separate libraries or packages that you don't control.

---

### Pass 4 — Change Preventers

Improve modularity so changes don't cascade across the codebase.

---

#### [16/23] Divergent Change

**What to look for**: A single class that needs to be modified for many different unrelated reasons. When adding feature A you change methods X and Y, but adding feature B changes methods Z and W — all in the same class.

**Treatments**:
- Extract Class (split each axis of change into its own class)
- Extract Superclass / Extract Subclass (if the axes of change follow an inheritance pattern)

**When to ignore**: n/a — a class that changes for multiple reasons violates SRP.

---

#### [17/23] Shotgun Surgery

**What to look for**: A single logical change requires small modifications scattered across many different classes. The opposite of Divergent Change — the responsibility is fragmented across too many places.

**Treatments**:
- Move Method / Move Field (consolidate scattered behavior into a single cohesive class)
- Create a new class if no existing class is the right home
- Inline Class (collapse classes that became too thin after consolidation)

**When to ignore**: n/a — scattered responsibilities should be consolidated.

---

#### [18/23] Parallel Inheritance Hierarchies

**What to look for**: Creating a subclass in one hierarchy requires creating a corresponding subclass in another hierarchy. Class name prefixes often match across the two hierarchies.

**Treatments**:
- Restructure so one hierarchy's instances reference the other hierarchy's instances
- Move Method / Move Field to eliminate the redundant hierarchy

**When to ignore**: When eliminating the parallel hierarchy would degrade code quality or introduce worse coupling. Accept the duplication if the alternative is genuinely worse.

---

### Pass 5 — Couplers

Reduce excessive coupling between objects. Process these last because class structures should be stable by now.

---

#### [19/23] Feature Envy

**What to look for**: A method that accesses data from another object more than its own data. The method "envies" another class and probably belongs there.

**Treatments**:
- Move Method (relocate the method to the class it envies)
- Extract Method (if only part of the method envies another class, extract that part and move it)

**When to ignore**: When the behavior is intentionally separated from data, as in Strategy, Visitor, or other behavioral design patterns. Also ignore when the method uses data from multiple classes equally.

---

#### [20/23] Inappropriate Intimacy

**What to look for**: Classes that reach into each other's private fields and methods. Bidirectional dependencies. Classes that know too much about each other's implementation details.

**Treatments**:
- Move Method / Move Field (put things where they're actually used)
- Extract Class + Hide Delegate (formalize the relationship through a clean interface)
- Change Bidirectional to Unidirectional Association
- Replace Delegation with Inheritance (when one class truly IS a specialization of the other)

**When to ignore**: n/a — tight coupling between classes should be reduced.

---

#### [21/23] Message Chains

**What to look for**: Long chains of method calls like `a.getB().getC().getD().doThing()`. The client is coupled to the entire chain of object relationships.

**Treatments**:
- Hide Delegate (have intermediary objects expose the needed functionality directly)
- Extract Method + Move Method (identify what the end of the chain provides and move that logic closer to the client)

**When to ignore**: When adding delegation methods would create a Middle Man (the next smell). Balance chain hiding with meaningful responsibility.

---

#### [22/23] Middle Man

**What to look for**: A class where the majority of its methods do nothing but delegate to another class. The class exists only to forward calls without adding value.

**Treatments**:
- Remove Middle Man (have clients talk to the delegate directly)

**When to ignore**: When the middle man serves a legitimate purpose: dependency management (preventing unwanted coupling), or design patterns like Proxy or Decorator that require indirection.

---

#### [23/23] Incomplete Library Class

**What to look for**: A library class that doesn't provide a method you need, and you can't modify the library source.

**Treatments**:
- Introduce Foreign Method (add a utility method that takes the library object as a parameter — for a small number of missing methods)
- Introduce Local Extension (create a wrapper or subclass for substantial additions)

**When to ignore**: When extending the library would create excessive maintenance burden from having to track library updates.

---

## Commit Strategy

After completing all 23 smell phases:

1. Review all changes made across all phases.
2. Create ONE commit per smell category that had changes (up to 5 commits):

   - `refactor: remove dispensable code (dead code, duplicates, speculative generality)`
   - `refactor: decompose bloated constructs (long methods, large classes, parameter lists)`
   - `refactor: fix OO abuse patterns (switch statements, refused bequest, temp fields)`
   - `refactor: improve change resilience (divergent change, shotgun surgery)`
   - `refactor: reduce coupling (feature envy, message chains, middle man)`

3. Each commit message body should list the specific smells addressed and a count of refactorings applied.
4. Skip categories where no changes were made.

**CRITICAL:** Generate the commit message body by reading `git diff --cached`, not by recalling planned changes. If a change you intended to include is not shown by `git diff --cached`, either stage it first or omit it from the message. Only describe changes that are staged.

## Post-Refactoring Verification

After all commits:

1. Show total impact:
   ```bash
   git diff --stat <BRANCHPOINT>
   ```

2. If `TEST_CMD` was detected, run the test suite.

3. If tests fail:
   a. Analyze which refactoring likely caused the failure.
   b. Attempt to fix the test or the refactored code.
   c. If unfixable after a reasonable attempt, offer to revert the most recent category's commit.

4. If `LINT_CMD` was detected, run the linter and fix any issues.

5. Present a summary table:

```
| Category           | Smells Checked | Instances Found | Refactored | Ignored | Deferred |
|--------------------|----------------|-----------------|------------|---------|----------|
| Dispensables       | 6              | ...             | ...        | ...     | ...      |
| Bloaters           | 5              | ...             | ...        | ...     | ...      |
| OO Abusers         | 4              | ...             | ...        | ...     | ...      |
| Change Preventers  | 3              | ...             | ...        | ...     | ...      |
| Couplers           | 5              | ...             | ...        | ...     | ...      |
| **TOTAL**          | **23**         | ...             | ...        | ...     | ...      |
```

6. List any "deferred refactorings" — changes that would improve code quality but require modifying files outside the branch scope.
