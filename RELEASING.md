# Releasing blick

The release loop lives in `~/notes/releasing.md` — the ordered steps, the apt
step, the winget submission, the spent-tag rule, and the standing facts about
tokens and secrets. Failure recipes are in `~/notes/build_release_gotchas.md`.
This file carries what is true of blick and not of its siblings.

| | |
|---|---|
| Loop | goreleaser |
| `apt-ship` argument | `blick-cli` |
| winget package | `Excelano.blick` |
| Windows asset | `blick_<version>_windows_amd64.zip` |

**The repository is `blick-cli` and the command is `blick`.** Everything
downstream — the binary, the archive names, the Debian package, the Homebrew
formula, the winget identifier — uses the short name; only the clone URL and the
release download path carry the `-cli` suffix. That mismatch shows up twice in
the loop: `apt-ship` takes the repository name, and the winget `--urls` value
has `blick-cli` in the path and `blick` in the filename.

**The release builds** platform archives for Linux and macOS on both
architectures plus Windows x64, the two `.deb` packages, `checksums.txt`, the
Homebrew formula, and the GitHub Release, all in one job.

**Windows is x64 only.** goreleaser ignores the `windows/arm64` combination, so
there is one Windows archive and one winget installer entry. Adding the target
later means the winget manifest grows a second `Installers` block.

`install.sh` and `uninstall.sh` are attached to the release as extra files, which
is what lets a user pin an install to a release URL instead of the rolling `main`
branch. They ship as-is from the tagged commit, so a fix to either only reaches
pinned installs on the next release.

**The release workflow has no `workflow_dispatch` fallback,** so it only ever
runs from a real tag push. If a tag lands without triggering it, the fix is to
delete and re-push the tag by hand, or add the dispatch input.

**Test against a real tenant before tagging.** blick reads live Microsoft 365
mail, chats, and calendar over Graph, and the unit tests cannot see a token. A
release that builds cleanly can still be broken against the API, so run the
commands you touched against your own account first. Anything that writes
deserves the extra pass.
