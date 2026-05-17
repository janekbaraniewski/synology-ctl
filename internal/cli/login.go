package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janbaraniewski/synology-ctl/internal/config"
)

// newLoginCmd creates or updates a NAS profile. It always runs the full
// onboarding flow (scan → pick → credentials) so the user can review what
// they're connecting to even when a single device is discovered.
func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Configure a NAS profile (scan → pick → credentials → keychain)",
		Long: `Run the interactive onboarding flow.

This always shows the list of devices mDNS discovered (even when there's
only one) so you can review what you're about to connect to. Credentials
are stored in the macOS Keychain; non-secret settings live in
~/.config/synoctl/config.yaml.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			_, err = Onboard(cmd.Context(), cfg)
			return err
		},
	}
}

// newLogoutCmd removes the active profile's password from the keychain.
func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the active profile's password from Keychain",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			p, ok := cfg.Active()
			if !ok {
				return errors.New("no active profile configured")
			}
			if err := config.DeletePassword(p.Host, p.Username); err != nil {
				return err
			}
			fmt.Printf("✓ Cleared password for %s@%s\n", p.Username, p.Host)
			return nil
		},
	}
}
