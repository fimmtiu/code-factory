---
name: cf-decomment
description: "Strips unnecessary or excessively verbose comments from the codebase. Triggers on: remove comments, decomment, /cf-decomment."
user-invocable: true
---

# Decomment

We want to ensure that all comments and docstrings in the codebase are helpful and brief. Look at every comment and docstring in the codebase and decide whether it should be removed, reworded, or left alone. (The examples in this skill are in multiple languages, but all these principles apply to all languages.)

## Exceptions

Ignore the following types of comments, which should never be removed:
- Compiler and tool directives, such as //go:build, //go:generate, //go:embed, //nolint, # frozen_string_literal:, # rubocop:disable, # typed:, eslint-disable, @ts-expect-error, and shebangs
- License and copyright headers
- Any comments in generated and vendored files (node_modules/, vendor/, *.pb.go, files that contain Code generated ... DO NOT EDIT)
- Comments which are inside strings or heredocs

## Redundant comments

Comments which just restate the code, or explain what the code is doing when it would be obvious from context, should be completely removed. Comments which explain _why_ a piece of code is required can stay. Comments which explain _how_ the code works should be removed, unless they explain something that's difficult or non-obvious.

Examples of redundant comments we should remove:
```ruby
# only create events if our account is saved
create! if user.account && user.account.persisted?

# loop through all accounts
accounts.each { |account| account.do_stuff }

# Invariant part of the posted sitrep's header line.
SITREP_HEADER_TEXT = "Weekly sitrep — #"
```

## Verbose comments

Comments should be brief and concise, not conversational. For every comment you decide to keep, examine it to see if it can be reworded in a more concise manner. Remove unnecessary clauses and words, keeping only the meat of the meaning.

Examples of excessively verbose comments:
```python
# Lint, typecheck, and run the Python suite inside the production image (Dockerfile.worker `test`                                  │
# stage). The build starts `FROM` the DevCycle clio/python base image, pulled cross-account                                        │
# from its ECR (us-west-2). GitHub-hosted runners don't get that access for free, so the job                                       │
# assumes the pull-only CI role via OIDC to mint an ECR auth token. The role ARN comes from                                        │
# the bootstrap stack's CiRoleArn output (see cdk/lib/bootstrap-stack.ts).                                                         │
#                                                                                                                                  │
# Builds both images to prove they both build correctly. The test stage runs in the worker image                                   │
# because it carries the full package. The web image is built separately to catch COPY mistakes                                    │
# that would otherwise only surface at deploy time.                                                                                │
```

This could be replaced by simply:
```python
# Runs lint, typecheck, and tests in the worker image's `test` stage, and builds the web image.                                    │
# The base image is in a cross-account ECR, so the job assumes a pull-only CI role via OIDC.
```

## Outdated comments

Comments that describe how the codebase used to be — descriptions of old bugs, references to tracking tickets, mentions of how things have changed, and such — should be reworded to be in the present tense. Users shouldn't have to know the history of the project or refer to data on external sites to understand the code. Describe it as it currently stands.

Bad:
```javascript
// `rewrite_followup` runs defensively only when the planner ECHOED the user's original verbatim — otherwise
// trust the planner's divergent standalone (re-rewriting de-referenced text made it answer the question
// instead, PLA-464).

// The two shapes from PLA-513: a top-level message and a thread reply.
```

Good:
```javascript
// `rewrite_followup` runs defensively only when the planner ECHOED the user's original verbatim — otherwise
// trust the planner's divergent standalone (re-rewriting de-referenced text makes the model answer the
// question instead of rewriting it).

// The two link shapes: a top-level message and a thread reply.
```

## Prohibited vocabulary

If you see any of the following words in a comment, reword the comment to use different terminology.

* load-bearing
* gate
* gated
* seam
* belt-and-suspenders
* belt-and-braces
* byte-identical
