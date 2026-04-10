sqlite

Create a separate branch for this work. We're going to completely re-do how
`cf-tickets` stores data.

## Changes to data storage

We're going to move all the work unit data out of JSON files and into a SQLite
database in `.code-factory/data.sqlite`. We'll use `database/sql` with the
`mattn/go-sqlite3` driver to access it.

It should have a schema similar to this:

```sql
CREATE TABLE "tickets" (
  "id" integer PRIMARY KEY,
  "identifier" text NOT NULL UNIQUE,
  "description" text NOT NULL,
  "phase" text NOT NULL DEFAULT "plan",
  "status" text NOT NULL DEFAULT "idle",
  "claimed_by" integer DEFAULT NULL,
  "last_updated" integer DEFAULT NULL,   -- Unix timestamp in seconds
  "project_id" integer DEFAULT NULL
);

CREATE TABLE "projects" (
  "id" integer PRIMARY KEY,
  "identifier" text NOT NULL UNIQUE,
  "description" text NOT NULL
);

CREATE TABLE "dependencies" (
  "id" integer PRIMARY KEY,
  "work_unit_type" integer NOT NULL,  -- 1 for a ticket_id, 2 for a project_id
  "work_unit_id" integer NOT NULL,
  "dependency_type" integer NOT NULL,  -- 1 for a ticket_id, 2 for a project_id
  "dependency_id" integer NOT NULL
);

CREATE TABLE "comments" (
  "id" integer PRIMARY KEY,
  "ticket_id" integer NOT NULL,
  "filename" text NOT NULL,
  "line_number" integer NOT NULL,
  "comment" text NOT NULL
);
```

Add any indexes that you can prove that you'll need. Add foreign keys for `tickets.project_id` and `comments.ticket_id` — tickets should be deleted if their parent project is deleted, and comments should be deleted if their parent ticket is deleted.

## New shared code

Add shared code in `internal` for reading and writing the SQLite database. All commands currently handled by `ticketsd` should be rewritten to read/store the data in SQLite instead:

- `status` should read ticket, project, comment, and dependency information from the database
- `create-project` should insert into the `projects` and `dependencies` tables
- `create-ticket` should insert into  the `tickets` and `dependencies` tables
- `set-status` should update the `tickets` table
- ...and so forth.

This should be a clean encapsulation boundary — the callers MUST NOT know or care how the data is being stored.

All updates must be atomic and take place within a single transaction.

## Changes to `ticketsd` command

The `ticketsd` command is no longer needed, since SQLite can handle concurrent
access for us. Delete it. (Most of its code will end up being moved to `internal`.)

## Changes to `cf-tickets` command

The `cf-tickets` command will now read/write from the SQLite database instead of talking through a socket to another process. It must use the code in `internal` to access the SQLite database; the `cmd/cf-tickets` directory should contain no SQLite-related code.
