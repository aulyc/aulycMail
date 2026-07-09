// Package backend is the host-linked implementation of the built-in Contacts
// pane. It exposes the Wails-bound ContactsBridge and the rail registration
// function App.Startup calls.
//
// File map:
//   - register.go — rail tab registration
//   - api.go      — local contact API wrapper (Search/Get/List/Write)
//   - convert.go  — internal types → coreapi.Contact converters
//   - api_test.go — wrapper unit tests against an in-memory SQLite
//
// The pane's frontend lives at extensions/contacts/frontend (Svelte
// components, stores, keyboard handlers, and locale files).
package backend
