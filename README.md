# synoctl

A TUI-first management tool for Synology DSM. Auto-discovers your NAS on the
local network, stores credentials in the macOS Keychain, and renders a live
dashboard with drill-down views.

## Build & run

```bash
make build         # → ./bin/synoctl
make run           # build and launch the TUI
make discover      # mDNS scan only
```

## First run

Just run `./bin/synoctl`. With no profile configured, it drops straight into
onboarding:

1. Spinner while it scans the local mDNS bus for Synology devices.
2. A picker shows every discovered NAS (plus "Enter host manually") — even
   when there's only one device, so you can review what you're about to
   connect to.
3. A credentials form (username, password, optional 2FA OTP).
4. Probes the chosen scheme; if you picked http but the box only speaks
   https (or vice versa) it automatically retries with the other one.
5. Saves the profile to `~/Library/Application Support/synoctl/config.yaml`
   (non-secrets only) and the password to the macOS Keychain. Stores a 2FA
   device token so you won't be asked for OTP again on this machine.

Subsequent runs jump straight to the dashboard.

## CLI

| command            | what it does |
| ------------------ | ------------ |
| `synoctl`          | Launch the TUI (auto-onboards on first run). |
| `synoctl discover` | Print discovered NAS devices and exit. |
| `synoctl login`    | Re-run the onboarding flow to add or update a profile. |
| `synoctl logout`   | Remove the active profile's password from Keychain. |
| `synoctl version`  | Print build info. |

## TUI key bindings

| key            | action |
| -------------- | ------ |
| `tab` / `[`,`]` | next / previous view |
| `:`             | command palette (type a view name) |
| `/`             | filter (in views that support it) |
| `j` / `k`       | move cursor down / up |
| `g` / `G`       | jump to top / bottom |
| `r`             | refresh now |
| `s` / `x` / `R` | start / stop / restart (Services & Packages) |
| `n` / `p`       | next / prev page (Logs) |
| `t`             | toggle log source (Logs) |
| `?`             | help overlay |
| `q`             | quit |

## Views

Dashboard · Volumes · Disks · Shares · Users · Packages · Services ·
Network · Logs.

Each view is one file under `internal/tui/views/` — adding more is a matter
of writing a `View` implementation and registering it in
`internal/cli/tui.go`.

## Project layout

```
cmd/synoctl/                 # binary entry
internal/cli/                # Cobra commands + onboarding flow
internal/config/             # YAML config + Keychain wrapper
internal/discover/           # mDNS scanner (multi-service browse)
internal/dsm/                # Typed DSM Web API client
internal/tui/                # Bubbletea root model, theme, keymap
internal/tui/views/          # Per-screen views (dashboard, volumes, …)
```

## Dependencies (all upstream — no reinvention)

`charmbracelet/{bubbletea,bubbles,lipgloss,huh,huh/spinner,glamour,log}`
· `NimbleMarkets/ntcharts` · `76creates/stickers` · `dustin/go-humanize` ·
`atotto/clipboard` · `pkg/browser` · `grandcat/zeroconf` ·
`zalando/go-keyring` · `spf13/cobra` · `gopkg.in/yaml.v3`.
