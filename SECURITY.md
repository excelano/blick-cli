# Security Policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately through GitHub Security Advisories at https://github.com/excelano/blick-cli/security/advisories/new. If you would rather not use GitHub, email david.anderson@excelano.com instead. I aim to respond within seven days.

Please do not open public issues for security problems.

## Supported versions

The latest release receives security fixes. Older versions are not supported. Fixes ship in a new tagged release; update the way you installed. There are no maintained release branches.

## What blick-cli can access

blick-cli is a CLI that runs locally on your machine. When you sign in via the device-code flow, it requests the following delegated Microsoft Graph permissions: `User.Read`, `Mail.ReadWrite`, `Mail.Send`, `Calendars.Read`, `Presence.ReadWrite`, `People.Read`, `Chat.ReadWrite`, and `Chat.Create`. Those permissions scope blick-cli to the mailbox, calendar, presence session, contact suggestions, and Teams chats your account already has access to; it cannot escalate permissions or operate outside what your account can do in Outlook, the Teams client, or the Microsoft 365 web UI. All operations are attributable to the signing user in Microsoft 365 audit logs, exactly as if you had performed them through any other Microsoft client. blick-cli does not implement administrative or tenant-wide operations.

All of these scopes are user-consentable by default per Microsoft's stock Graph policy. Individual tenants can require admin consent for any of them. If a specific scope is blocked, the corresponding feature degrades: set `"enable_teams": false` to run without Teams features, skip `Presence.ReadWrite` to run without `blick presence`, or hand-edit `~/.config/blick/contacts.json` if `People.Read` is unavailable for the address-book seed.

## What blick-cli stores

blick-cli stores its configuration at `~/.config/blick/config.json` (client ID, tenant ID, and the `enable_teams` flag); nothing in it is a secret. The cached OAuth token, at `~/.config/blick/token.json`, and the address book, at `~/.config/blick/contacts.json` (built from `People.Read` suggestions or entries you add by hand, and holding the names and addresses of people you correspond with), are both encrypted at rest through [atrest](https://github.com/excelano/atrest), under a key your operating system holds rather than one stored beside the file: DPAPI on Windows, your desktop's D-Bus Secret Service on Linux (or the kernel's per-user keyring where no such session is reachable), and your login Keychain on macOS. Where none of these is reachable, both files fall back to plaintext, protected only by their file mode, 0600 in a 0700 directory.

A draft saved to `~/.config/blick/drafts/` after a failed send is plaintext by design: it holds the message you were composing, and you need to be able to open it in an ordinary editor to resend it. Nothing else is written there.

That is everything: no telemetry, no analytics, no remote logging. blick-cli talks only to Microsoft's identity and Graph endpoints (`login.microsoftonline.com` and `graph.microsoft.com`).

## App registration

blick-cli does not ship with a published app registration. Each user creates their own single-tenant registration in Azure AD and writes the client and tenant IDs into `~/.config/blick/config.json`. See the README for the automated (`setup.sh` + Azure CLI) and manual (Azure portal) procedures. This model keeps audit log attribution and conditional access policy inside the user's tenant from day one.
