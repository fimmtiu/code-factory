---
name: cf-respond
description: Implement change requests on a ticket. Trigger on `/respond` or `/cf-respond`.
user-invocable: true
---

# Respond to code review change requests

Read actionable code review change requests from the `tickets` system and address them one by one.

Trigger on `/respond` or `/cf-respond`.

## Prerequisites

1. **Identify the ticket**: The ticket identifier (e.g. `my-project/my-ticket`) must be provided as an argument after the trigger command (e.g. `/cf-respond my-project/my-ticket`). If no identifier was provided, ask the user for it and stop.
2. **Verify you are on a feature branch**: If the current branch is `master` or `main`, tell the user and stop.

## Step 1: Get change requests for the ticket

Run `tickets open-change-requests <ticket-identifier>` to get a JSON dump of all change requests. If no change requests are returned by this command, report that there is nothing to do and stop.

## Step 2: Respond to each open change request

For each open change request, in order:

1. Read the `code_location` and `description` fields to understand what needs to change.
2. Write or update tests first to cover the requested change, then modify the implementation code.
3. Run the project's lint and test commands. Check `CLAUDE.md` or the project's Makefile for the correct commands. If you don't find them there, consult project conventions or infer the correct commands from the language.
4. Commit the changes in a single commit. The commit message must include:
   - A summary of what was changed and why.
   - The change request ID (the `id` field from the JSON) and ticket identifier, e.g. `"Addresses change request #42 on my-project/my-ticket"`.
5. Run `tickets close-change-request <id>` where `<id>` is the numeric `id` field from the change request JSON.
