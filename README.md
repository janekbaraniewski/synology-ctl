# synoctl

A TUI-first management tool for Synology DSM. Auto-discovers your NAS,
stores credentials in the macOS Keychain, and renders structured
drill-downs for every list view (no JSON dumps in the UI).

## Quick start

```bash
make build         # → ./bin/synoctl
make run           # build + launch the TUI

# First run auto-onboards: mDNS / subnet sweep → pick device → credentials.
# Subsequent runs jump straight into the workspace.
```

## Workspace

A persistent left sidebar groups views into sections; the main pane
renders the active view; an optional right inspector previews the
cursor'd row live (auto-hidden on narrow terminals).

```
synoctl   ●  192.168.1.36:5000      DSM 7.0.1-42218     up 169d 16h   20:21
─────────────────────────────────────────────────────────────────────────
OVERVIEW   │  Active Insight   1.2.0-1214   Synology Inc.  4.0 MiB  running
 Dashboard │  ...                                                          │  inspector
STORAGE    │                                                               │  preview
 Volumes   │                                                               │  of cursor'd
 Disks     │                                                               │  row
 Shares    │                                                               │
 Files     │                                                               │
 Usage     │                                                               │
APPS       │                                                               │
 Apps      │                                                               │
 Containers│                                                               │
...        │                                                               │
─────────────────────────────────────────────────────────────────────────
 t mode · ⏎ details · / filter · s start · x stop · U uninstall    ⇥ nav  : cmd  ? help  q quit
```

## Sidebar sections

| Section | Views |
|---|---|
| **Overview** | Dashboard — CPU/mem/net/disk gauges, sparklines, top processes, recent log activity |
| **Storage** | Volumes · Disks · Shares · Files · Usage Analyzer |
| **Apps** | Apps (Installed / Available / Services) · Containers |
| **Backup** | Hyper Backup · Active Backup |
| **Services** | Drive · Cameras (Surveillance Station) |
| **Security** | Certificates · Security Advisor · Firewall |
| **System** | Admin · Scheduled Tasks · DDNS |
| **Tools** | API Explorer |

## Headline views

| View | What it does | Local keys |
|---|---|---|
| **Apps** | Three modes via `t` (or `1` / `2` / `3`): **Installed** (start/stop/restart/uninstall), **Available** (browse the DSM catalogue and install — lazy-loaded with an elapsed-time counter; the catalog walk can take 30–120 s on a slow NAS), **Services** (enable/disable). | `t` mode · `s` `x` `R` start/stop/restart · `U` uninstall · `I` install · `e` `d` enable/disable |
| **Files** | Full File Station browser. Starts at the share root; `⏎` descends; `⌫` ascends; breadcrumb shows the current path. `o` downloads to a per-session temp dir and hands off to the OS opener (`open` / `xdg-open` / `start`) — the TUI equivalent of double-clicking in a file manager. | `⏎` open · `⌫`/`h` up · `o` open with system app · `W` download · `D` delete · `N` rename |
| **Usage Analyzer** | ncdu-style space breakdown. Root lists shares by used size (via `SYNO.FileStation.DirSize`); drill in to see immediate children sized in the background. `e` toggles a by-extension breakdown of the current level. Results are cached per-path for the session. | `⏎` drill in · `⌫` up · `e` ext breakdown · `R` re-size |
| **API Explorer** | Browse every API the device advertises via `SYNO.API.Info`. Pick an API, pick a method, fill the `+`/`-` params editor, fire — see the JSON response in a syntax-highlighted viewer. The safety net for anything the TUI doesn't first-class wire. | `⏎` select · `+`/`-` add/remove param · `^r` call |
| **Users** | Local accounts with full CRUD: `c` create, `E` edit, `P` reset password (all via a tab-cycling form). `D` deletes (confirmed). | `c` create · `E` edit · `P` password · `D` delete |
| **System** | Identity (model, serial, DSM, CPU, RAM) + utilisation (load 1/5/15m, mem, swap) + temperature + power. | `B` reboot · `S` shutdown — both confirm |
| **Volumes / Disks / Shares** | Per-entity views. Inspector shows the cursor'd row's full details; `⏎` opens a charted drill-down (capacity / inode / temperature / SMART / pool membership / share flags). | `⏎` details · `/` filter |
| **Logs** | Paginated system / connection log. `t` toggles the source; `n` / `p` flip pages. | `n`/`p` next/prev page · `t` toggle source · `⏎` open entry |

The 10 newer package areas (Containers, Cameras, Hyper Backup, Active
Backup, Drive, Certificates, Security Advisor, Firewall, DDNS,
Scheduled Tasks) follow the same listBase pattern: row list + filter +
detail drill-down + inspector where the cursor'd row has rich info
worth previewing. Each handles "package not installed" gracefully —
when the underlying API isn't advertised by the device, the view shows
a tasteful empty-state instead of erroring.

## Global keys

| key | action |
|---|---|
| `tab` / `]` | next view in the sidebar |
| `shift+tab` / `[` | previous view |
| `}` / `{` | jump to next / previous section |
| `:` | command palette (fuzzy match on view names) |
| `/` | filter the current list |
| `r` | refresh now |
| `a` | open the contextual action menu |
| `i` | toggle the inspector pane |
| `^b` | toggle the sidebar |
| `?` | help overlay |
| `q` / `^c` | quit |

The bottom hint strip reads from each view's `Hint()` so it only
shows keys that actually do something in the current context.

## CLI subcommands

| command | what it does |
|---|---|
| `synoctl` | Launch the TUI (auto-onboards on first run). |
| `synoctl discover` | mDNS + subnet sweep + Tailscale peer enumeration. |
| `synoctl login` | Re-run onboarding to add/update a profile. |
| `synoctl logout` | Remove the active profile's password from Keychain. |
| `synoctl apis [-f X]` | Dump `SYNO.API.Info` — what's actually advertised by your DSM build. |
| `synoctl raw <api> <method> [-v N] [-p k=v]` | Issue any DSM call and print the JSON envelope. Indispensable for diagnosing API mismatches across DSM versions. |
| `synoctl version` | Build info. |

The TUI's **API Explorer** view is the same idea as `synoctl raw` with
a form-based UI on top, so most exploratory work happens inside the
TUI now.

## How it stays version-tolerant

DSM API naming + payload shapes drift between firmware versions. The
reference device for this project is a DS220j on DSM 7.0.1-42218
introspected with `synoctl apis` and `synoctl raw`. Drift-tolerance is
baked in across the dsm client:

* **`flexBool`** — accepts both `true`/`false` and `0`/`1` for fields
  like `recycle_bin_admin_only`, `enable`, `is_default`, `is_broken`,
  `encrypted`, `enabled_ipv6` that flip shape across firmwares.
* **Modern-first with fall-back** — calls that exist at multiple
  versions (`SYNO.Core.Package.Server` v2/v1, `SYNO.SurveillanceStation.Camera`
  v9/v8, etc.) try the newer shape first and fall back on error code 104.
* **Envelope-key drift** — `packages` vs `list`, `tasks` vs `task_list`,
  `items` vs `checklist`, `files` vs `items` — we accept both.
* **Field-name drift** — `SYNO.Core.Package.Server` returns the
  identifier under `id` on modern DSM and `package` on DSM 7.0.1, and
  omits the human-readable name entirely on the older firmware (the
  web UI joins against a separate i18n bundle). `ServerPackage`
  exposes `Identifier()` and `DisplayName()` to coalesce both.
* **Long timeouts on the HTTP client** — the slowest DSM calls
  (catalogue walk, package list with `additional`) can take 30–120 s
  on a low-end NAS. The default HTTP client timeout is set to 3 min;
  per-view fetches apply their own (shorter) context deadlines.

## Project layout

```
cmd/synoctl/                 # binary entry
internal/cli/                # Cobra commands + onboarding flow
internal/config/             # YAML config + Keychain wrapper
internal/discover/           # mDNS / subnet sweep / Tailscale peers
internal/dsm/                # Typed DSM Web API client (one file per area)
internal/tui/                # Bubbletea root model, theme, keymap,
                             # nav/actions/inspector contracts
internal/tui/views/          # Per-screen views + shared listBase,
                             # Confirm, Prompt, OTP modal, UserForm
```

## Install

```bash
brew tap janekbaraniewski/tap
brew install synoctl
```

Or grab a release tarball from the [releases page](https://github.com/janekbaraniewski/synology-ctl/releases) and drop the binary anywhere on `$PATH`.
