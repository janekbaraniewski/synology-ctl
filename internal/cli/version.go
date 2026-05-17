package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build-time variables set via -ldflags in the Makefile.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("synoctl %s\n  commit: %s\n  built:  %s\n", version, commit, date)
		},
	}
}
