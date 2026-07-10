// Package backend is the host-linked implementation of the built-in Contacts
// pane. It exposes the Wails-bound ContactsBridge used by App.
//
// File map:
//   - api.go      — local contact API wrapper (Search/Get/List/Write)
//   - convert.go  — internal types → contactdto.Contact converters
//   - api_test.go — wrapper unit tests against an in-memory SQLite
//
// The pane's frontend lives at frontend/src/lib/contacts (Svelte
// components, stores, keyboard handlers, and locale files).
package contactpane
