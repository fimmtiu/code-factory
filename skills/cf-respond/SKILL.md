---
name: cf-respond
description: Implement change requests on a branch from a variety of different perspectives. Trigger on `/respond` or `/cf-respond`.
user-invocable: true
---

# Respond to code review change requests

We will read actionable code review change requests from the `tickets` system and address them one by one.

Trigger on `/respond` or `/cf-respond`.

## Prerequisites

Before starting, run through ALL of the following steps:

1. **Find the identifier of the ticket this is for**: It should have been specified as part of the skill invocation.
2. **You must be on a branch**: if the current branch is `master` or `main`, complain and exit.

Then run the following steps in order:

## Step 1: Get change requests for the ticket

Run `tickets get-change-requests <ticket-identifier>` to get a JSON dump of the change requests on the ticket.

## Step 2: Respond to change requests

For each change request, implement changes to address the issue it's describing. You MUST write or update the tests first, then change the code. Before committing, run linting and formatting commands to proofread your changes.

The work to address each change request should be committed a separate commit, which must include a detailed description of what you changed and why the change was made.

After committing, run `tickets close-change-request <change-request-id>` to mark the change request as addressed.
