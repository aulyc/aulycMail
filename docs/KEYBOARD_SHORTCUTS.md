# aulycMail Keyboard Shortcuts

Complete reference of all keyboard shortcuts in aulycMail.

On macOS, `Command` is the primary application modifier. The tables use
`Command` for macOS shortcuts; `Ctrl` is accepted where the frontend explicitly
supports both modifiers.

All aulycMail-specific navigation and action shortcuts are controlled by
**Settings → General → Enhanced keyboard navigation**. The setting defaults to
on and applies only after Save. Turning it off removes the orange region
indicator and leaves custom navigation/actions to native WebKit behavior.
`Command+F` and the complete search flow remain available.

## Global Shortcuts

These shortcuts work when enhanced keyboard navigation is on unless marked
always available.

### Application

| Shortcut | Action |
|----------|--------|
| `Command+F` | Open global search (always available, including when enhanced keyboard navigation is off) |
| `Command+Q` | Quit application |
| `Command+N` | Compose new message |
| `Command+Shift+A` | Sync all accounts |
| `Command+Shift+S` | Sync selected folder |
| `Command+Tab` | Switch to next rail pane (Mail / Contacts) when delivered to the webview |
| `` Command+` `` | Switch to previous rail pane when delivered to the webview |
| `Tab` / `Shift+Tab` | Move forward/backward through the orange-marked major regions |
| `Shift+F10` | Open every visible action in the current region |

### Pane Navigation

| Shortcut | Action |
|----------|--------|
| `Option+Left` / `Option+H` | Focus previous pane (viewer -> list -> sidebar) |
| `Option+Right` / `Option+L` | Focus next pane (sidebar -> list -> viewer) |

### Sidebar Navigation

| Shortcut | Action |
|----------|--------|
| `Option+Up` / `Option+K` | Navigate to the previous sidebar item, wrapping at the top |
| `Option+Down` / `Option+J` | Navigate to the next sidebar item, wrapping at the bottom |
| `Option+Enter` | Expand/collapse focused account folder tree |
| `Option(L)+Option(R)` | Brings up context menu for the focused folder |

### Message Actions (when message is selected/focused)

| Shortcut | Action |
|----------|--------|
| `Command+R` | Reply to last message (requires viewed conversation) |
| `Command+Shift+R` | Reply All to last message (requires viewed conversation) |
| `Command+U` | Mark as read (keyboard-focused or checked messages) |
| `Command+Shift+U` | Mark as unread (keyboard-focused or checked messages) |
| `Command+K` | Archive (keyboard-focused or checked messages) |
| `Command+J` | Mark as spam (keyboard-focused or checked messages) |
| `Command+L` | Load remote images in viewed message |
| `Command+Shift+L` | Open "Always Load Images" dropdown |
| `Shift+F` | Toggle focus mode for the message |

---

## Pane-Specific Shortcuts

These shortcuts depend on which pane is focused. They are disabled when typing in input fields.

### Sidebar (Folder List)

| Shortcut | Action |
|----------|--------|
| `Up` / `K` | Enter or leave the Compose/Sync action group toward the previous sidebar item; wraps at the top |
| `Down` / `J` | Enter or leave the Compose/Sync action group toward the next sidebar item; wraps at the bottom |
| `Left` / `Right` | Switch between Compose and Sync while the action group is highlighted |
| `Enter` / `Space` | Activate the highlighted Compose or Sync action |
| `Option + Enter` / `Space` | Expand/collapse account (when account header is focused) |
| `Option(L) + Option(R)` | Brings up context menu for the focused folder |

### Message List

| Shortcut | Action |
|----------|--------|
| `Up` / `K` | Select previous conversation |
| `Down` / `J` | Select next conversation |
| `Shift+Up` / `Shift+K` | Select previous + toggle checkbox |
| `Shift+Down` / `Shift+J` | Select next + toggle checkbox |
| `Space` | Toggle checkbox on current conversation |
| `Enter` / `V` | Open selected conversation in viewer |
| `D` | Delete selected/checked message(s) — move to Trash (same as `Delete`) |
| `Command+A` | Select all messages in folder |
| `Option(R)` | Brings up context menu for the selected message(s) |

### Conversation Viewer

| Shortcut | Action |
|----------|--------|
| `Up` / `K` | Scroll up |
| `Down` / `J` | Scroll down |
| `Delete` / `Backspace` | Delete focused message when focused on conversation viewer |
| `Command + A` | Select all text of message in viewport |
| `Option(R)` | Brings up context menu for the message focused |
| `F` | Toggles focus mode on the current thread (conversation) |
| `Shift+F10` | Open viewer buttons, per-message expand/collapse, copy-address, receipt, and attachment actions |

---

## Single-Key Shortcuts

These work when not in an input field. They apply to checked messages (bulk) or the keyboard-focused message in the list.

| Shortcut | Action |
|----------|--------|
| `S` | Toggle star |
| `Backspace` / `Delete` | Move to trash |
| `Shift+Backspace` / `Shift+Delete` | Permanently delete |
| `Escape` | Clear checkboxes (first press), close conversation (second press) |

---

## Composer Shortcuts

These only work when the composer is open.

| Shortcut | Action |
|----------|--------|
| `Command+Enter` | Send message |
| `Option+T` | Activate/Deactivate toolbar mode |
| `Option+A` | Attach a file |
| `Command+D` | Pop out/detach composer to separate window |
| `Shift+F10` | Open every visible composer action without adding buttons to the Tab cycle |
| `Escape` | Close composer (prompts to save draft if unsaved) |

---

### Text Formatting

| Shortcut | Action |
|----------|--------|
| `Command+B` | Bold |
| `Command+I` | Italic |
| `Command+U` | Underline |
| `Option+T`  | Enter toolbar mode; use arrows and Enter, or the displayed hints |

---

## Quick Reference Card

```
NAVIGATION
  Tab / Shift + Tab   Move between orange-marked major regions
  Option + Arrow Keys Pane focus (Left/Right) or Folder nav (Up/Down)
  Option + H/J/K/L    Vim-style: pane (H/L) or folder (J/K)
  Option(L)+Option(R) Brings context menu up for the focused folder
  Option + Enter      Expand/collapse account
  Command + Tab       Switch to next rail pane when delivered to the webview
  Command + `         Switch to previous rail pane when delivered to the webview
  Arrow Keys / HJKL   Navigate within focused pane
  Enter               Open conversation / Expand account
  Space               Toggle checkbox / Expand account
  Shift + F10         Current region actions

COMPOSE & REPLY
  Command + N         New message
  Command + R         Reply
  Command + Shift + R Reply All
  Command + Enter     Send (in composer)
  Command + D         Detach composer
  Option + T          Toggle format toolbar mode
SELECTION
  Command + A         Select all messages (list) / text (viewer)

MESSAGE ACTIONS
  S                   Star/Unstar
  Command + U         Mark read
  Command + Shift + U Mark unread
  Command + K         Archive
  Command + J         Spam
  V                   View message
  Delete / D          Trash
  Shift + Delete      Permanent delete
  Option(R)           Context Menu

OTHER
  Command + F         Global search (always available)
  Command + Shift + A Sync all accounts
  Command + Shift + S Sync selected folder
  Command + L         Load images
  Command + Q         Quit
  Escape              Clear/Close
```

---

## Behavior Notes

### Pane Focus Model

The main window has four keyboard regions:

1. **Feature rail** — Mail, Contacts, and Settings
2. **Sidebar** — account/folder or contact-source list
3. **Message/contact list**
4. **Conversation/contact detail**

`Tab` and `Shift+Tab` move only between these regions. The active region is
shown by one orange top border. Tab never degrades into traversing every child
button. Within a region, arrows move its logical selection, Enter/Space
activate the selected item, and `Shift+F10` opens a searchable list of the
region's visible actions.

On compact Apple keyboards, `Fn+Left` and `Fn+Right` normally arrive as
`Home` and `End`; lists that support first/last-item movement therefore work
without requiring dedicated Home/End keys. Depending on the macOS function-key
setting, `Shift+F10` may need to be pressed as `Fn+Shift+F10`.

### Folder Navigation

`Option+Up/Down` navigates through all folders in visual order:
1. Unified Inbox (All Inboxes)
2. Individual account inboxes under Unified Inbox
3. Account 1 header
4. Account 1 folders (if expanded)
5. Account 2 header
6. Account 2 folders (if expanded)
7. ... and so on

Collapsed accounts show only their header (not folders) in navigation.

### Message Actions Hierarchy

Action shortcuts (Delete, Archive, Spam, Star, Read/Unread) follow this priority:

1. **Checked messages** - If any messages are checked (via Space or Shift+navigation), actions apply to ALL checked messages
2. **Keyboard-focused message** - Otherwise, actions apply to the message that's currently focused in the message list (the one with keyboard highlight from j/k navigation)
3. The message being viewed in the conversation pane is independent - you can navigate to and delete a different message without opening it first

First `Escape` clears checkboxes, second `Escape` closes the conversation viewer.

### Composer Blocking

When the composer is open:
- `Command+R` and `Command+Shift+R` are blocked to prevent accidental replies.
- `Command+F` still opens global search.
- Composer shortcuts obey the enhanced-keyboard setting; native editing and IME
  behavior do not.

### Rail Navigation

`Command+Tab` cycles forward through the rail items when macOS delivers it to
the webview: Mail → Contacts → Mail. `` Command+` `` cycles backward. The
active rail pane is persisted across launches. The feature rail remains the
reliable in-app route because macOS may reserve these combinations.

Composer state is preserved across switches: switching to Contacts and back does not unmount or clear the composer.

### Unified Inbox

When viewing Unified Inbox and replying:
- Reply uses the account associated with the selected message
- This ensures replies come from the correct email address

---

## Contacts Pane

Contacts shortcuts only fire when Contacts is the active rail pane (selected
from the feature rail, or via `Command+Tab` / `` Command+` `` when delivered to
the webview). They never trigger while Mail is active, so shortcuts that
overlap with Mail's are routed by the active rail pane.

Pane-local navigation (Up/Down/J/K, Enter, Space, Option+H/L for pane cycling,
Option+Up/Down for the sidebar) uses the same kit-shared predicates Mail does.

**Sidebar navigation (works from any pane)**

Mirrors mail's "Folder Navigation" shortcuts. These fire regardless of which contacts pane currently has keyboard focus, so you can move through contact groups while the list or detail pane is focused.

| Shortcut | Action |
|----------|--------|
| `Option+Up` / `Option+K` | Move to previous group in the sidebar |
| `Option+Down` / `Option+J` | Move to next group in the sidebar |

**Pane cycling**

| Shortcut | Action |
|----------|--------|
| `Option+Left` / `Option+H` | Focus previous pane (detail → list → sidebar) |
| `Option+Right` / `Option+L` | Focus next pane (sidebar → list → detail) |

**Actions**

| Shortcut | Action |
|----------|--------|
| `E` | Edit the currently-focused contact |
| `Command+N` | Open the new-contact dialog |

> Within a focused pane, `Up`/`K` and `Down`/`J` cycle rows (contact list, sidebar sources) and `Enter` opens / activates — same kit predicates Mail's list and folder tree use.
