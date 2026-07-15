# aulycMail Frontend

This directory contains the Svelte frontend used by the Wails desktop app.

## Commands

Run commands from this directory unless noted otherwise:

```bash
npm install
npm run dev
npm run check
npm run build
```

The full desktop build is managed from the repository root:

```bash
make build
make install-darwin
```

## Structure

- `src/` - core application UI, stores, themes, and i18n
- `wailsjs/` - generated Wails bindings
- `scripts/` - frontend build helpers
- `dist/` - generated frontend build output
