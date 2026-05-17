// Command synoctl is a TUI-first CLI for managing Synology NAS devices.
//
// Running `synoctl` with no arguments loads the active profile from
// ~/.config/synoctl/config.yaml, fetches the stored credentials from the
// OS keychain, and launches the dashboard. The subcommands cover one-shot
// flows that don't need the full-screen interface.
package main

import "github.com/janbaraniewski/synology-ctl/internal/cli"

func main() {
	cli.Execute()
}
