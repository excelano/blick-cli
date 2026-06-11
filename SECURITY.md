# Security Policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately through GitHub Security Advisories at https://github.com/excelano/blick-cli/security/advisories/new. If you would rather not use GitHub, email david.anderson@excelano.com instead. I aim to respond within seven days.

Please do not open public issues for security problems.

## Supported versions

blick-cli is built from source on each install. Security fixes ship through `main`; pull and rebuild to apply them. There are no maintained release branches.

## What blick-cli can access

blick-cli is a CLI that runs locally on your machine. When you sign in via the device-code flow, it requests the following delegated Microsoft Graph permissions: `User.Read`, `Mail.ReadWrite`, `Mail.Send`, `Calendars.Read`, `Presence.ReadWrite`, and `Chat.ReadWrite`. Those permissions scope blick-cli to the mailbox, calendar, presence session, and Teams chats your account already has access to; it cannot escalate permissions or operate outside what your account can do in Outlook, the Teams client, or the Microsoft 365 web UI. All operations are attributable to the signing user in Microsoft 365 audit logs, exactly as if you had performed them through any other Microsoft client. blick-cli does not implement administrative or tenant-wide operations.

`Chat.ReadWrite` requires admin consent in most tenants. If your tenant blocks it, set `"enable_teams": false` in `~/.config/blick/config.json` and blick-cli runs without the Teams features using the remaining no-admin-consent permissions. `Presence.ReadWrite` is a user-consent scope; set `"presence_heartbeat": false` to opt out, and blick-cli will not request it.

## What blick-cli stores

blick-cli stores its configuration at `~/.config/blick/config.json` (client ID, tenant ID, the `enable_teams` flag, and the `presence_heartbeat` flag) and a cached OAuth token at `~/.config/blick/token.json`. That is everything: no telemetry, no analytics, no remote logging. The refresh token never leaves your machine. blick-cli talks only to Microsoft's identity and Graph endpoints (`login.microsoftonline.com` and `graph.microsoft.com`).

## App registration

blick-cli does not ship with a published app registration. Each user creates their own single-tenant registration in Azure AD and writes the client and tenant IDs into `~/.config/blick/config.json`. See the README for the automated (`setup.sh` + Azure CLI) and manual (Azure portal) procedures. This model keeps audit log attribution and conditional access policy inside the user's tenant from day one.
