# blick

Check unread Outlook emails, Teams chats, and your next meeting from the terminal.

## What blick is

blick replaces as much day-to-day Outlook and Teams usage as fits the keyboard. The aperture is deliberately narrow: it surfaces the things that need action right now — unread mail, unread chats, the meeting that's about to start — and gives you keyboard verbs to reply, mark read, compose, or join. It is not where you go to search history, organize folders, browse Teams channels, or take a call. Teams stays the place for calls and meetings, permanently.

Scope grows by friction, not by speculation. If something missing bites in real use it goes on the list; if it doesn't, it doesn't. The Graph API is the outer limit on what blick can do, but blick deliberately doesn't try to expose everything Graph offers.

## Install

### Debian and Ubuntu

Add the [Excelano apt repository](https://excelano.com/apt/) once (one-time setup):

```sh
curl -fsSL https://excelano.com/apt/setup.sh | sudo sh
```

Then install it, so `apt upgrade` keeps it current:

```sh
sudo apt install blick
```

### Homebrew

On macOS or Linux, tap and trust the repository once — Homebrew gates third-party taps behind explicit trust (one-time setup):

```sh
brew tap excelano/tap
brew trust excelano/tap
```

Then install it, so `brew upgrade` keeps it current:

```sh
brew install blick
```

### Prebuilt binary (Linux and macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/excelano/blick-cli/main/install.sh | sh
```

This downloads the latest release binary for your platform, verifies the SHA-256 checksum, and installs it to `/usr/local/bin` (or `~/.local/bin` if `/usr/local/bin` isn't writable). Override the destination with `BLICK_INSTALL_DIR=$HOME/bin sh`; pin to a specific tag with `BLICK_VERSION=v0.4.0 sh`. To uninstall: `curl -fsSL https://raw.githubusercontent.com/excelano/blick-cli/main/uninstall.sh | sh` (or `sudo apt remove blick` if installed via apt).

### Build from source

```bash
git clone https://github.com/excelano/blick-cli
cd blick-cli
go build -o blick .
mv blick ~/bin/
```

Requires Go 1.25+.

## Setup

You'll need an Azure app registration before `blick` can authenticate. Both options below assume you've already installed the binary.

### Option A: Automated (requires Azure CLI)

```bash
curl -fsSL https://raw.githubusercontent.com/excelano/blick-cli/main/setup.sh -o setup.sh
chmod +x setup.sh
az login
./setup.sh
```

(Or if you installed via apt, `setup.sh` ships at `/usr/share/doc/blick/setup.sh`.)

### Option B: Manual (Azure portal)

1. Go to [Azure Portal](https://portal.azure.com) → Azure Active Directory → App registrations
2. **New registration**
   - Name: `blick`
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
   - `Presence.ReadWrite`
   - `People.Read`
   - `Chat.ReadWrite` (optional, Teams chat support)
   - `Chat.Create` (optional, Teams chat support)
5. Copy **Application (client) ID** and **Directory (tenant) ID** from the Overview page

Create the config file:

```bash
mkdir -p ~/.config/blick
cat > ~/.config/blick/config.json << 'EOF'
{
    "client_id": "YOUR_CLIENT_ID_HERE",
    "tenant_id": "YOUR_TENANT_ID_HERE",
    "enable_teams": true,
    "presence_heartbeat": true
}
EOF
```

### Admin Consent

By default, none of these Graph permissions require admin consent — they are all user-consentable, including the Chat scopes. Some tenants tighten the default and require admin consent for one or more of these; if yours does, ask your IT admin to grant consent:

```bash
az ad app permission admin-consent --id YOUR_CLIENT_ID
```

If a specific scope is blocked, the corresponding feature degrades: set `"enable_teams": false` to skip Teams chat, `"presence_heartbeat": false` to skip the presence nudge. The address book seed (`blick contacts seed`) needs `People.Read`; if that's blocked, hand-edit `~/.config/blick/contacts.json` instead.

## Usage

```
$ blick

  📅 Standup with Tony — in 47 min · 10:30 AM — Online

  📧 unread emails (3):
    1. Alex K. — "Deck revisions"              (10 min ago · 9:42 AM)
    2. Jordan R. — "RE: Contract draft"         (1 hour ago · 8:53 AM)
    3. Newsletter — "Weekly digest"             (3 hours ago · 6:48 AM)

  💬 unread chats (2):
    4. Sam P. — "quick question on timeline"    (32 min ago · 9:20 AM)
    5. Riley T. — "can you check the numbers"   (1 hour ago · 8:51 AM)

  Commands:
    <N>      view               r<N>     reply
    d<N>     done               r        refresh
    t        show today         x        exit (mark all read)
    H        help               q        quit

blick> 1
  (shows full email body)

blick> r4
  Reply in Sam P.:
  (end with `.` on a line by itself, or Ctrl-C to cancel)
  > Should be ready by EOD tomorrow.
  > Let me know if you want to walk through it on a call.
  > .
  Sent.

blick> d3
  Marked as read: Weekly digest

blick> x
  All marked as read.
```

Each short command has a full-word equivalent — `reply 4`, `done 3`, `refresh`, `exit`, `quit`, `help`, `today`. Type `H` (or `help`) at the prompt for the full reference.

`t` (or `today`) shows the full calendar for the day, with past events dimmed and the current event highlighted:

```
$ blick today

  Tuesday, June 9, 2026

      9:00 AM – 9:30 AM    Daily standup            Online
     10:30 AM – 11:00 AM   Tony 1:1                 Online
      1:00 PM – 2:00 PM    Project review · now     Online
      4:00 PM – 5:00 PM    Demo prep                Conf Room A

  4 events · 4h scheduled
```

The same view is available inside the REPL by typing `today`.

## Files

- `~/.config/blick/config.json` — client ID, tenant ID, and feature flags
- `~/.config/blick/token.json` — cached OAuth token (auto-created)

## Permissions

All scopes are user-consentable by default per Microsoft's stock Graph policy. Individual tenants can require admin consent for any of them — see the [Admin Consent](#admin-consent) section.

| Permission | What it does |
|---|---|
| User.Read | Verify authentication |
| Mail.ReadWrite | Read and mark-read emails |
| Mail.Send | Send new emails and reply-all to existing ones |
| Calendars.Read | Show next meeting and today's calendar |
| Presence.ReadWrite | Nudge presence Away → Available on run |
| People.Read | Seed the address book from frequently-contacted people |
| Chat.ReadWrite | Read/reply Teams chats |
| Chat.Create | Start new 1:1 chats with address-book contacts |

## Presence heartbeat

When you run `blick`, the tool reads your current Microsoft 365 presence. If
you're showing as Away — typically because Teams' idle timer fired — blick
registers itself as an active session with availability `Available` for one
hour. Subsequent runs reset the hour.

The mechanic is Graph's `presence: setPresence` endpoint, which is a
*session* (not an override). Microsoft aggregates across sessions with the
precedence DoNotDisturb > Busy > Available > Away, so our Available wins
over Teams' idle-driven Away, but a user-set Do Not Disturb still wins over
our Available. We never touch Busy, In a meeting, or Out of office — those
are real signals and stay as-is.

Opt out with `"presence_heartbeat": false` in config. With the heartbeat
off, the `Presence.ReadWrite` scope is also not requested.
