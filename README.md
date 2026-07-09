![Logo](frontend/src/assets/images/logo-universal.png)

# aulycmail - A Lightweight E-Mail Client

Maintained by: @aulyc

![screenshot](docs/ss.png)

## Why?

Most desktop e-mail clients are either tied to a large platform stack or carry
too much legacy UI and background overhead. aulycmail focuses on a lightweight,
local desktop experience with a modern mail workflow.

## Summary

aulycmail is a standalone e-mail client inspired by
[Geary](https://wiki.gnome.org/Apps/Geary), focused on:

- Resource Efficiency - Minimal CPU, RAM, and battery consumption
- Modern UX - Clean interface with dark mode support
- Keyboard & Mouse Friendly - Keyboard navigation with vim-style shortcuts
- Independence - No dependency on Gnome Online Accounts or other system services
- Search That Works - Local and IMAP search for everyday mail

## OS Support

This checkout currently targets the local macOS desktop build:

- macOS

Build and install flows in this repository are maintained for macOS only.

## Features

- Multiple Accounts
- Providers: (experimental entries are marked)
  - Generic IMAP/SMTP
  - GMail
  - Microsoft 365 / Outlook
  - Yahoo
  - Proton Mail (via Proton Bridge)
  - iCloud Mail
  - Mailfence
  - Murena
  - Fastmail (experimental)
  - Zoho Mail (experimental)
  - AOL Mail (experimental)
  - GMX Mail
  - Mail.com (experimental)
  - Mailbox.org
- Unified Inbox with account color coding
- Conversation threads
- Basic removal of tracking elements in mail content
- WYSIWYG detachable composer powered by [TipTap Editor](https://github.com/ueberdosis/tiptap)
- WYSIWYG signatures powered by [TipTap Editor](https://github.com/ueberdosis/tiptap)
- Local contacts and autocomplete through the built-in Contacts extension
- Basic local and IMAP search
- Notifications that focus the related e-mail when clicked
- Auto-sync when the system wakes from sleep
- Multiple color themes
- First-party extension system with bundled Contacts extension
- [Keyboard Shortcuts](docs/KEYBOARD_SHORTCUTS.md)

## Installation

- [Official Installation Guide](https://github.com/aulyc/aulycmail)

## Documentation

- [Official Documentation](https://github.com/aulyc/aulycmail)
- [Build Guide](docs/BUILD.md)
- [Extension Guide](docs/EXTENSIONS.md)
- [Security Policy](SECURITY.md)

## Tech Stack

aulycmail is built with [Wails](https://wails.io) and
[Svelte](https://svelte.dev/).

aulycmail is CASA Tier 2 Certified by Google's preferred
[authorized assessor](https://appdefensealliance.dev/casa/casa-assessors):
[TAC Security](https://tacsecurity.com/).

## News

- 2026-03-11 - Microsoft Verified
- 2026-04-16 - CASA Tier 2 Certified
- 2025-04-25 - Google Verified

## Roadmap

Confirmed future features:

- Post quantum ready encryption
- Mailfence and StartMail setup presets

Potential future features:

- Customizable shortcut keys
- Advanced search
- AI assisted composition

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## Maintainer

Current repository maintained by [@aulyc](https://github.com/aulyc).

## Terms of Use & Privacy Policy

- [Terms of Use](docs/TERMS.md)
- [Privacy Policy](docs/PRIVACY.md)
