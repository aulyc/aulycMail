# Database Rollback Guide

aulycmail database migrations are forward-only. The application never downgrades
the schema in place, and this repository no longer ships SQL scripts that
recreate removed historical schemas.

## What This Means

- If a newer aulycmail writes schema version `N`, older builds that only know an
  earlier version will refuse to open that database.
- Removed remote-contact, token, and extension-secret tables are not preserved
  by current migrations. After the cleanup migration has run, those SQLite
  objects are gone.
- To inspect or restore data from a removed schema, use a database backup made
  before the cleanup migration ran.

## Before Downgrading

1. Quit aulycmail completely.
2. Back up the current database file.
3. Restore a database backup created by the older version you want to run.
4. Launch the older aulycmail build.

Default database locations:

- Linux: `~/.local/share/aulycmail/aulycmail.db`
- macOS: `~/Library/Application Support/aulycmail/aulycmail.db`
- Windows: `%LOCALAPPDATA%\aulycmail\aulycmail.db`

## If You See A Schema-Version Error

The database was written by a newer build than the one currently running. Use a
newer aulycmail build, or restore an older database backup. Do not hand-edit the
`migrations` table; that hides the version mismatch without reshaping the schema
and can corrupt local mail/contact state.
