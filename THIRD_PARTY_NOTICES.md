# Third-Party Notices

This file records license notices for upstream and third-party components
included in or used by aulycMail. It is not a license for the proprietary
aulycMail application as a whole.

## Aerion

aulycMail includes and modifies portions of Aerion:

- Project: Aerion
- Source: https://github.com/hkdb/aerion
- Copyright: Copyright 2024-2025 Aerion Contributors
- License: Apache License, Version 2.0

Those portions remain subject to Apache-2.0. The complete, unmodified upstream
license text and copyright notice are distributed as:

- source tree: `LICENSES/Aerion-Apache-2.0.txt`
- macOS application:
  `Contents/Resources/Legal/Aerion-Apache-2.0.txt`

The Aerion-derived portions have been modified by aulyc beginning in 2026.
The distributed `AERION_MODIFICATIONS.md` file identifies the origin and
material categories of change.

aulycMail is an independent product. It is not affiliated with, sponsored by,
or endorsed by the Aerion project or Aerion Contributors. No Aerion trademark
rights are granted by the Apache License.

## Bundled Font

The bundled Nunito font file is licensed under the SIL Open Font License,
Version 1.1. The complete copyright and license notice are distributed as:

- source tree: `frontend/src/assets/fonts/OFL.txt`
- macOS application: `Contents/Resources/Legal/Nunito-OFL.txt`

## Runtime and Dependency Components

aulycMail uses third-party runtime and build components, including Wails,
Svelte, Vite, TipTap, Iconify, bits-ui, Tailwind CSS utilities, Go e-mail
libraries, SQLite-related Go packages, and other transitive dependencies listed
in `go.mod`, `go.sum`, `frontend/package.json`, and
`frontend/package-lock.json`.

Those components remain governed by their respective upstream licenses. This
notice does not replace any license or attribution file that a component
requires to accompany redistribution.
