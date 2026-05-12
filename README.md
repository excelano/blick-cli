# checkin

Check unread Outlook emails, Teams chats, and your next meeting from the terminal.

## Install

```bash
./install.sh
```

Builds the binary and copies it to `~/bin/checkin`.

## Setup

### Option A: Automated (requires Azure CLI)

```bash
az login
./setup.sh
```

### Option B: Manual (Azure portal)

1. Go to [Azure Portal](https://portal.azure.com) → Azure Active Directory → App registrations
2. **New registration**
   - Name: `checkin`
   - Supported account types: Accounts in this organizational directory only
3. **Authentication** → Add a platform → Mobile and desktop applications
   - Check `https://login.microsoftonline.com/common/oauth2/nativeclient`
   - Enable **Allow public client flows** (required for device code flow)
   - Save
4. **API permissions** → Add a permission → Microsoft Graph → Delegated permissions:
   - `User.Read`
   - `Mail.ReadWrite`
   - `Mail.Send`
   - `Calendars.Read`
   - `Chat.ReadWrite` ← requires admin consent
5. Copy **Application (client) ID** and **Directory (tenant) ID** from the Overview page

Create the config file:

```bash
mkdir -p ~/.config/checkin
cat > ~/.config/checkin/config.json << 'EOF'
{
    "client_id": "YOUR_CLIENT_ID_HERE",
    "tenant_id": "YOUR_TENANT_ID_HERE",
    "enable_teams": true
}
EOF
```

### Admin Consent

Most enterprise tenants require admin consent for all permissions. Ask your IT admin to grant consent:

```bash
az ad app permission grant --id YOUR_CLIENT_ID --api 00000003-0000-0000-c000-000000000000 --scope "User.Read Mail.ReadWrite Mail.Send Calendars.Read Chat.ReadWrite"
```

Without admin consent, checkin won't be able to authenticate at all in locked-down tenants. If only `Chat.ReadWrite` is blocked, set `"enable_teams": false` in config to use email and calendar without Teams.

## Usage

```
$ checkin

  📅 Standup with Tony — in 47 min — Online

  📧 unread emails (3):
    1. Christa — "D13 deck revisions"         (10 min ago)
    2. Art Acuna — "RE: Department contracts"  (1 hour ago)
    3. Newsletter — "Weekly digest"            (3 hours ago)

  💬 pending chats (2):
    4. Matt Hyde — "quick question on timeline"  (32 min ago)
    5. Tony — "can you check the D12 numbers"    (1 hour ago)

  Commands:
    <N>      view message      r<N>     reply
    d<N>     mark read (done)   x       mark all read & quit
    r        refresh            q       quit

checkin> 1
  (shows full email body)

checkin> r4
  Reply in Matt Hyde:
  > Should be ready by EOD tomorrow.
  Sent.

checkin> d3
  Marked as read: Weekly digest

checkin> x
  All emails marked as read.
  Note: Teams chats can't be marked as read via API.
```

## Files

- `~/.config/checkin/config.json` — client ID, tenant ID, and enable_teams flag
- `~/.config/checkin/token.json` — cached OAuth token (auto-created)

## Permissions

| Permission | What it does | Admin consent? |
|---|---|---|
| User.Read | Verify authentication | No |
| Mail.ReadWrite | Read and mark-read emails | No |
| Mail.Send | Reply to emails | No |
| Calendars.Read | Show next meeting | No |
| Chat.ReadWrite | Read/reply Teams chats | Yes |
