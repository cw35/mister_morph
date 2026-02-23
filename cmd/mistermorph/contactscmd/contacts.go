package contactscmd

import (
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/contacts"
	"github.com/quailyquaily/mistermorph/internal/pathutil"
	"github.com/quailyquaily/mistermorph/internal/statepaths"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contacts",
		Short: "Manage business-layer contacts",
	}
	cmd.PersistentFlags().String("dir", "", "Contacts state directory (defaults to file_state_dir/contacts)")
	return cmd
}

func serviceFromCmd(cmd *cobra.Command) *contacts.Service {
	dir, _ := cmd.Flags().GetString("dir")
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = statepaths.ContactsDir()
	} else {
		dir = pathutil.ExpandHomePath(dir)
	}
	return contacts.NewServiceWithOptions(
		contacts.NewFileStore(dir),
		contacts.ServiceOptions{
			FailureCooldown: configuredContactsFailureCooldown(),
		},
	)
}

func configuredContactsFailureCooldown() time.Duration {
	v := viper.GetDuration("contacts.proactive.failure_cooldown")
	if v <= 0 {
		return 72 * time.Hour
	}
	return v
}
