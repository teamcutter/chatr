package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:  "list",
        RunE: func(cmd *cobra.Command, args []string) error {
            mgr, _, _ := newManager()

            packages, err := mgr.List()
			if err != nil {
				return err
			}

			for _, v := range packages {
				fmt.Printf("%s\n", v)
			}

            return nil
        },
    }

    return cmd
}