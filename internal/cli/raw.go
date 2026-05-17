package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/janbaraniewski/synology-ctl/internal/config"
	"github.com/janbaraniewski/synology-ctl/internal/dsm"
)

// newRawCmd issues an arbitrary DSM API call and prints the unwrapped
// data envelope as pretty JSON. Indispensable when DSM's documented field
// names don't match what the box actually returns.
//
// Usage:
//
//	synoctl raw SYNO.Core.Storage.Volume list -v 1 -p location=internal
//	synoctl raw SYNO.Core.Service get
func newRawCmd() *cobra.Command {
	var (
		version int
		params  []string
	)
	cmd := &cobra.Command{
		Use:   "raw <api> <method>",
		Short: "Issue a DSM API call and print the JSON response",
		Args:  cobra.ExactArgs(2),
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
				DeviceID: profile.DeviceID, DeviceName: "synoctl-raw",
			}); err != nil {
				return fmt.Errorf("login: %w", err)
			}
			defer func() { _ = client.Logout(ctx) }()

			vals := url.Values{}
			for _, kv := range params {
				k, v, ok := strings.Cut(kv, "=")
				if !ok {
					return fmt.Errorf("--param must be key=value, got %q", kv)
				}
				vals.Set(k, v)
			}
			var raw json.RawMessage
			if err := client.Call(ctx, args[0], version, args[1], vals, &raw); err != nil {
				return err
			}
			var out bytes.Buffer
			if err := json.Indent(&out, raw, "", "  "); err != nil {
				return err
			}
			fmt.Println(out.String())
			return nil
		},
	}
	cmd.Flags().IntVarP(&version, "version", "v", 1, "API version")
	cmd.Flags().StringArrayVarP(&params, "param", "p", nil, "additional key=value parameters (repeatable)")
	return cmd
}
