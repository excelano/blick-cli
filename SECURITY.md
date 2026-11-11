# Security Policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately through GitHub Security Advisories at https://github.com/excelano/blick-cli/security/advisories/new. If you would rather not use GitHub, email david.anderson@excelano.com instead. I aim to respond within seven days.

Please do not open public issues for security problems.

## Supported versions

The latest release receives security fixes. Older versions are not supported. A fix ships in a new release; update the way you installed.

## What blick-cli can access

blick-cli is a CLI that runs locally on your machine. When you sign in via the device-code flow, it requests the delegated Microsoft Graph permissions listed in the README's [Permissions](README.md#permissions) table. Those permissions scope blick-cli to the mailbox, calendar, presence session, contact suggestions, and Teams chats your account already has access to; it cannot escalate permissions or operate outside what your account can do in Outlook, the Teams client, or the Microsoft 365 web UI. All operations are attributable to the signing user in Microsoft 365 audit logs, exactly as if you had performed them through any other Microsoft client. blick-cli does not implement administrative or tenant-wide operations.

All of these scopes are user-consentable by default per Microsoft's stock Graph policy, and a tenant can require admin consent for any of them. The README's [Admin Consent](README.md#admin-consent) section says what still works when a scope is blocked.

## What blick-cli stores

blick-cli stores its configuration at `~/.config/blick/config.json` (client ID, tenant ID, and the `enable_teams` flag); nothing in it is a secret. The cached OAuth token, at `~/.config/blick/token.json`, and the address book, at `~/.config/blick/contacts.json` (built from `People.Read` suggestions or entries you add by hand, and holding the names and addresses of people you correspond with), are both encrypted at rest through [atrest](https://github.com/excelano/atrest), under a key your operating system holds rather than one stored beside the file: DPAPI on Windows, your desktop's D-Bus Secret Service on Linux (or the kernel's per-user keyring where no such session is reachable), and your login Keychain on macOS. Where none of these is reachable, both files fall back to plaintext, protected only by their file mode, 0600 in a 0700 directory. A file sealed under a keyring that is locked when blick runs, as after a reboot on a machine that signs in automatically, makes blick raise the desktop's own prompt to unlock it; if the prompt is dismissed or there is no display to show it on, blick signs in again and leaves the address book closed rather than overwrite it.

A draft saved to `~/.config/blick/drafts/` after a failed send is plaintext by design: it holds the message you were composing, and you need to be able to open it in an ordinary editor to resend it.

blick-cli writes a few other files in plaintext. The dashboard prompt keeps its command history in `~/.config/blick/history`; it holds the commands you typed, which include contact keys and search terms, but not message bodies. `blick contacts edit` hands your editor a plaintext copy of the address book, `contacts-edit-*.json` in `~/.config/blick/` at mode 0600, removed once the edit is saved or abandoned and left in place if the edited copy does not parse and you decline to fix it; `blick contacts seed` does the same with a `blick-seed-*.json` file in the system temporary directory. An attachment you save goes where you name it, and one you open goes to a `blick` folder under `$XDG_RUNTIME_DIR` (or the system temporary directory) for the operating system to clear; both are written at mode 0644. On Linux, `blick tray` logs to `~/.cache/blick/tray.log`, and `blick tray --autostart yes` writes `blick-tray.desktop` to `~/.config/autostart/`.

blick-cli sends no telemetry and does no analytics or remote logging, and talks only to Microsoft's identity and Graph endpoints (`login.microsoftonline.com` and `graph.microsoft.com`).

## Verifying releases

Every GitHub release includes a `checksums.txt` file listing SHA-256 hashes of all binary archives. Verify any download before running it:

    sha256sum -c checksums.txt --ignore-missing

Release artifacts are built by `build-release`, the release program in excelano/shipping, from a tagged commit using the goreleaser configuration in this repository (`.goreleaser.yml`).

## App registration

blick-cli does not ship with a published app registration. Each user creates their own single-tenant registration in Microsoft Entra ID and writes the client and tenant IDs into `~/.config/blick/config.json`. See the README for the automated (`setup.sh` + Azure CLI) and manual (Azure portal) procedures. This model keeps audit log attribution and conditional access policy inside the user's tenant from day one.
