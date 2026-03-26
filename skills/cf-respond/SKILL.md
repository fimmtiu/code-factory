---
name: cf-respond
description: Thoroughly respond a changeset from a variety of different perspectives. Trigger on `/respond` or `/cf-respond`.
---

# Respond to code review comments

We will read actionable code review comments from the `tickets` system and address them one by one.

Trigger on `/respond` or `/cf-respond`.

## Prerequisites

Before starting, run through ALL of the following steps:

1. **Find the identifier of the ticket this is for**: It should have been specified as part of the skill invocation.
2. **You must be on a branch**: if the current branch is `master` or `main`, complain and exit.

Then run the following steps in order:

## Step 1: Get comments for the ticket

Run `tickets get-comments <ticket-identifier>` to get a JSON dump of the comments on the ticket.

## Step 2: Respond to comments

For each comment, make changes to address the issue it's describing. You must change the tests first, then change the code. Before committing, run linting and formatting commands to proofread your changes.

The work to address each comment should be committed in separate commits, which must include a detailed description of what you changed and why the change was made.

## Step 3: Clean up

Run `tickets close-comments <ticket-identifier>` to mark all comments as addressed.
