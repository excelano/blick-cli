# Blick CLI Backlog

Friction observed in real use. Items here have bitten in practice — speculative ideas don't earn a slot. Order within sections is rough; what bubbles to the top is whatever's hurting most that week.

---

## Next up

### Hide quoted reply history in email view

The iOS Blick app strips quoted reply history from the initial body render and exposes a tap to unfold it. On the CLI, `view N` today dumps the entire thread, which buries the new content under accumulated history. Elide quoted blocks on the initial render — lines starting with `>` after the HTML strip, plus reply-separator markers like "On {date}, {name} wrote:" and Outlook's "From: / Sent: / To: / Subject:" header chunk — with a one-line `[N quoted lines hidden]` summary and a verb to unfold (TBD: standalone `expand` after the initial view, or a `view N full` flag).

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

---

## Attachments

Attachment handling as a category is too large to scope all at once. Captured here as smaller, independent slices.

### Attachment indicator in the unread list

Add a small paperclip glyph next to messages where Graph's `hasAttachments` is true. One field added to the `$select` list, one column-render tweak in `display.go`. No new endpoints. Cheap and immediately useful — lets the user see at a glance which messages will be heavier to deal with.

### List attachments on a message

`attach N` against the unread list calls `/me/messages/{id}/attachments` and prints `[1] name.pdf (124 KB)` style rows. No download. Read-only inspection.

### Save an attachment to disk

`attach save N <index>` writes the attachment to the current working directory (or `--to <path>`). Graph returns base64-encoded content for file attachments; decode and write at mode 0644. Refuse silently on item attachments (calendar invites, embedded messages) — those need different handling.

### Open an attachment in the default app

`attach open N <index>` writes to a temp file in `$XDG_RUNTIME_DIR` (or `/tmp`) and shells out to `xdg-open`. File is left for the OS to clean up on session end. Useful for one-off "let me just look at the PDF" without polluting `~/Downloads`.

### Send an attachment

`email alice --attach file.pdf` (and `--attach file1.pdf --attach file2.png` for multiple) on the compose flow. Reads the file, base64-encodes, includes in the `attachments` array of the `/me/sendMail` payload. Graph caps file attachments at ~3 MB total per message; for larger files the upload-session flow is needed — defer that to a separate slice. Plain files only — no inline images, no item attachments.

### Strip attachments when forwarding

When `forward N` (in the Compose & messaging section) lands, default to not including attachments from the original — Graph's `/forward` endpoint does include them, but bandwidth + accidental forwarding of confidential files makes the opt-in version safer. `--with-attachments` flag re-enables the default Graph behavior.

---

## Compose & messaging extensions

### Forward an email

`forward N` against the unread list — opens a compose flow pre-loaded with the original subject (`Fwd: ...`) and quoted body, prompts for recipients. Graph endpoint `/me/messages/{id}/forward`.

### CC / BCC on email compose

`--cc alice,bob` and `--bcc carol` flags on `email`. Parse extends `parseEmailArgs`; recipients resolve through the same `store.Resolve` flow.

### Quote original in reply

When `reply N` opens, pre-fill the body with `> ` -prefixed quoted lines from the original message (truncated to first ~20 lines), cursor positioned above the quote. Optional — many users prefer top-posting clean. Could be a config toggle `quote_in_reply: true`.

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

### Manual presence set

`presence available`, `presence busy`, `presence dnd`, `presence brb`, `presence away`. Uses Graph's `setUserPreferredPresence` (1-day expiration) plus a session via `setPresence` (PT1H) so the override holds even when no other client is running. Mirrors iOS's presence menu exactly.

### DND timer

`dnd 30m`, `dnd 1h`, etc. — convenience wrapper that sets DND with a finite expiration via the same `setPresence` call but with a shorter `expirationDuration`. Useful for focus blocks.

---

## Operational

### Undo a bulk action

When bulk verbs (mark-all-read, future bulk-flag, future bulk-restore) land, a session-local undo stack lets `undo` revert the last one. iOS uses an 8-second floating banner; CLI uses an explicit verb with no time limit.
