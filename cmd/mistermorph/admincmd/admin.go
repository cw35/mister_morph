package admincmd

import "github.com/spf13/cobra"

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Run admin HTTP APIs and SPA",
	}
	cmd.AddCommand(newServeCmd())
	return cmd
}
