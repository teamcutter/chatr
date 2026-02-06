package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of chatr",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s%s%s%s%s%s%s\n", bold("chatr"), bold("-"), bold(version.Version),
				bold("-"), bold(runtime.GOOS), bold("/"), bold(runtime.GOARCH))
		},
	}
}
