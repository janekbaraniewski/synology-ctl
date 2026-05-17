package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/janbaraniewski/synology-ctl/internal/config"
	"github.com/janbaraniewski/synology-ctl/internal/dsm"
)

// newAPIsCmd dumps the SYNO.API.Info table from the active profile's NAS.
// This is the ground truth for which APIs/versions are available on a
// given DSM build — without it, picking the right call for storage,
// services, etc. is guesswork.
func newAPIsCmd() *cobra.Command {
	var filter string
	cmd := &cobra.Command{
		Use:   "apis",
		Short: "List the DSM APIs the active profile's NAS advertises",
		Long: `Calls SYNO.API.Info on the current profile and prints the API
table — useful for figuring out which exact API name/version/method to
use for a given feature, since DSM versions disagree on the names.

Pass --filter to grep, e.g. --filter Storage or --filter Service.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			profile, ok := cfg.Active()
			if !ok {
				return errors.New("no profile configured; run `synoctl login`")
			}
			client, err := dsm.New(dsm.Options{
				Scheme: profile.Scheme, Host: profile.Host, Port: profile.Port,
				Insecure: profile.Insecure, Timeout: 20 * time.Second,
			})
			if err != nil {
				return err
			}
			password, err := config.LoadPassword(profile.Host, profile.Username)
			if err != nil || password == "" {
				return errors.New("no password in keychain; run `synoctl login`")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if _, err := client.Login(ctx, dsm.LoginRequest{
				Account: profile.Username, Password: password,
				DeviceID: profile.DeviceID, DeviceName: "synoctl-apis",
			}); err != nil {
				return fmt.Errorf("login: %w", err)
			}
			defer func() { _ = client.Logout(ctx) }()

			if err := client.Info(ctx); err != nil {
				return fmt.Errorf("SYNO.API.Info: %w", err)
			}
			apis := client.SupportedAPIs()
			sort.Strings(apis)

			accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")).Bold(true)
			muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))

			matched := 0
			for _, name := range apis {
				if filter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
					continue
				}
				matched++
				info, _ := client.APIInfo(name)
				fmt.Printf("%s  %s  v%d→v%d\n",
					accent.Render(name),
					muted.Render("path="+info.Path),
					info.MinVersion, info.MaxVersion,
				)
			}
			if filter != "" {
				fmt.Printf("\n%s\n", muted.Render(fmt.Sprintf("%d / %d APIs matched %q", matched, len(apis), filter)))
			} else {
				fmt.Printf("\n%s\n", muted.Render(fmt.Sprintf("%d APIs total", len(apis))))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "case-insensitive substring filter")
	return cmd
}
