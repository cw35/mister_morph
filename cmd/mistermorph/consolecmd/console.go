package consolecmd

import "github.com/spf13/cobra"

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console",
		Short: "Run console HTTP APIs and SPA",
	}
	cmd.AddCommand(newServeCmd())
	return cmd
}
