# Database Recovery Guide

aulycMail database migrations are forward-only. The application never downgrades
the schema in place.

## What This Means

- If a newer aulycMail build writes schema version `N`, older builds that only
  know an earlier version will refuse to open that database.
- To restore data from an older schema, use a database backup made before the
  newer migration ran.

## Before Downgrading

1. Quit aulycMail completely.
2. Back up the current database file.
3. Restore a database backup created by the older version you want to run.
4. Launch the older aulycMail build.

Default macOS database location:

```text
~/Library/Application Support/aulycmail/aulycmail.db
```

## If You See A Schema-Version Error

The database was written by a newer build than the one currently running. Use a
newer aulycMail build, or restore an older database backup. Do not hand-edit the
`migrations` table; that hides the version mismatch without reshaping the schema
and can corrupt local mail/contact state.
