package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/teamcutter/chatr/internal/config"
	"github.com/teamcutter/chatr/internal/registry"
	"github.com/teamcutter/chatr/internal/state"
	"golang.org/x/sync/errgroup"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Fetch the latest package index",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			formulaReg := registry.New(cfg.FormulaeDir)
			caskReg := registry.NewCask(cfg.FormulaeDir)

			stop := withSpinner(cmd.Context(), "Updating package index...")

			var formulaeCount, caskCount int
			g, ctx := errgroup.WithContext(cmd.Context())

			g.Go(func() error {
				n, err := formulaReg.Update(ctx)
				if err != nil {
					return fmt.Errorf("formulae: %w", err)
				}
				formulaeCount = n
				return nil
			})

			g.Go(func() error {
				n, err := caskReg.Update(ctx)
				if err != nil {
					return fmt.Errorf("casks: %w", err)
				}
				caskCount = n
				return nil
			})

			if err := g.Wait(); err != nil {
				stop()
				return err
			}

			stop()
			fmt.Printf("%s Updated successfully: %s formulae, %s casks\n",
				green("✓"), green(formulaeCount), green(caskCount))

			st, err := state.NewSQLite(cfg.StateDB, cfg.ManifestFile)
			if err != nil {
				return nil
			}

			installed, err := st.ListInstalled()
			if err != nil {
				return nil
			}

			type outdated struct {
				name       string
				oldVersion string
				newVersion string
			}
			var outdatedPkgs []outdated

			for _, pkg := range installed {
				if pkg.IsDep {
					continue
				}

				var newVersion string
				if pkg.IsCask {
					f, err := caskReg.Get(cmd.Context(), pkg.Name)
					if err != nil {
						continue
					}
					newVersion = f.FullVersion()
				} else {
					f, err := formulaReg.Get(cmd.Context(), pkg.Name)
					if err != nil {
						continue
					}
					newVersion = f.FullVersion()
				}

				if pkg.FullVersion() != newVersion {
					outdatedPkgs = append(outdatedPkgs, outdated{
						name:       pkg.Name,
						oldVersion: pkg.FullVersion(),
						newVersion: newVersion,
					})
				}
			}

			if len(outdatedPkgs) == 0 {
				return nil
			}

			fmt.Printf("\nOutdated packages:\n")
			for _, o := range outdatedPkgs {
				fmt.Printf("  %s %s → %s\n", bold(o.name), dim(o.oldVersion), green(o.newVersion))
			}
			fmt.Printf("\n%s package(s) can be upgraded. Run %s to update them.\n",
				yellow(len(outdatedPkgs)), bold("chatr upgrade --all"))

			return nil
		},
	}
}
