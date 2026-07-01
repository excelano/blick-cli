# Blick CLI Backlog

Friction observed in real use. Items here have bitten in practice — speculative ideas don't earn a slot. Order within sections is rough; what bubbles to the top is whatever's hurting most that week.

---

## Next up

_(empty — pick the next item from the sections below.)_

---

## Compose & messaging extensions

### Drafts list / resume / delete

`drafts list` shows the timestamped files in `~/.config/blick/drafts/` written by `saveDraftCopy` on send failure. `drafts resume <id>` reopens the compose flow pre-filled. `drafts delete <id>` removes one. Closes the loop on the existing save-on-failure behavior.

---

## Inbox triage

### Delete or move to folder

`delete N` (moves to Deleted Items via `POST /me/messages/{id}/move`) and `move N <folder>` for archiving. Folder name resolves to ID via `/me/mailFolders`, cached per session. Both are Graph one-shot calls.

### Search

`search --from alice` and `search --text "X"` via Graph's `/me/messages?$search=` (KQL syntax). Results render the same way as the unread queue. Useful when scrolling unread isn't enough — e.g., "where's that email from the bank?"

### Show full thread

`thread N` expands the conversation containing message N — Graph `/me/messages?$filter=conversationId eq '...'`. Renders messages in reverse-chronological with sender + date + body preview each.

---

## Calendar actions

### "Running late" quick message

`late N` against a meeting row sends a templated chat to the meeting organizer ("Running a few minutes late") via the existing chat plumbing. Requires `enable_teams: true`.

---

## Presence control

Lower priority — David's note. Captured for future.

### DND timer

`dnd 30m`, `dnd 1h`, etc. — convenience wrapper that sets DND with a finite expiration via the same `setPresence` call but with a shorter `expirationDuration`. Useful for focus blocks.

---

## Operational

### Undo a bulk action

When bulk verbs (mark-all-read, future bulk-flag, future bulk-restore) land, a session-local undo stack lets `undo` revert the last one. iOS uses an 8-second floating banner; CLI uses an explicit verb with no time limit.

---

## iOS parity gaps

Features the iOS Blick app has where the CLI does not. Roughly ordered by how often they'd hit at the keyboard.

### Mark email back to unread

Inverse of `mark N`. iOS has it on the preview sheet. CLI gap: once you mark-read by accident, you reopen the message in Outlook to undo. New verb `unmark N` against the previous-refresh list.

### Bulk mark-read variants

iOS exposes mark-read-by-sender, by-subject, focused/other inbox, meeting notices (`meetingMessageType` filter), mailing lists (`List-Unsubscribe` header), and external senders (domain mismatch). On the CLI these would be REPL submenu under `mark`, e.g. `mark sender N` to mark all visible from the same sender as item N. Graph `$batch` POST for the bulk calls, capped at 20 ops per batch with selective revert on partial failure.

### Flag / unflag emails

`flag N` and `unflag N` against the unread queue. Plus bulk variants matching the mark-read flow. iOS uses a swipe; CLI uses verbs. Display in the unread list with a small flag indicator next to the sender.

### Restore today's items to unread

`restore mail today` and `restore chat today` — fetch messages and chats from local midnight to now that are currently read, batch-flip them unread, refresh. Useful when you triaged too aggressively before coffee.

### RSVP to meeting invites

Three verbs `accept N`, `tentative N`, `decline N` against either the calendar view or the unread queue (when N is an invite email). Graph's RSVP endpoints on `/me/events/{id}` with `sendResponse=true`. Auto-marks matching invite emails read after RSVP, mirroring iOS.

### Conflict detection in the calendar view

Compute overlap across the 10-event window in `TodaysMeetings` results. Mark conflicts with a small indicator in the `today` view; new verb `conflicts` lists overlapping pairs. No resolution flow — the CLI is keyboard, not sheet-based; resolution is `decline N` against the conflict.

### Custom Teams status message

`status set "..."` and `status clear` REPL verbs. Graph's `setStatusMessage`. Shown in the dashboard chrome alongside presence.

### Out of office toggle

`ooo on` / `ooo off` REPL verbs. Graph mailbox settings for automatic replies; default message preserved across toggles. Needs the `MailboxSettings.ReadWrite` scope — first new scope since v0.7.0 — so gated behind a config opt-in similar to `enable_teams`.

### Show "+N more unread" past the cap

iOS shows "Show all N" when unread exceeds the 20-message cap. CLI could surface `mail all` to lift the cap for one render, persisting nothing.

### Refresh failure banner

Today an error during refresh prints inline and may scroll past. A persistent one-line banner above the dashboard, cleared by the next successful refresh, mirrors iOS's orange "Couldn't reach Microsoft" affordance.

### Copy sender address / chat link

`copy sender N` and `copy chat N` write to the system clipboard via `xclip -selection clipboard` (Linux) or `pbcopy` (macOS). Useful when you need to paste the address into another tool.
