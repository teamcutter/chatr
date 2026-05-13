package cli

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newTldrCmd() *cobra.Command {
	var cask bool

	cmd := &cobra.Command{
		Use:   "tldr <package>",
		Short: "Show package summary and manual",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, reg, _, err := newManagerWithOptions(cask)
			if err != nil {
				return err
			}

			name := args[0]
			formula, err := reg.Get(cmd.Context(), name)
			if err != nil {
				return err
			}

			displayName := formula.Name
			if i := strings.Index(displayName, "@"); i != -1 {
				displayName = displayName[:i]
			}

			fmt.Printf("%s %s\n", green("●"), bold(displayName))
			fmt.Printf("  %s %s\n", cyan("version:"), formula.FullVersion())
			if formula.Description != "" {
				fmt.Printf("  %s %s\n", cyan("desc:"), formula.Description)
			}
			if formula.Homepage != "" {
				fmt.Printf("  %s %s\n", cyan("url:"), dim(formula.Homepage))
			}
			if formula.KegOnly {
				fmt.Printf("  %s %s\n", cyan("keg-only:"), yellow("yes"))
			}
			if len(formula.Dependencies) > 0 {
				fmt.Printf("  %s %s\n", cyan("deps:"), strings.Join(formula.Dependencies, ", "))
			}

			installed, pkg, _ := mgr.IsInstalled(name)
			if installed {
				fmt.Printf("  %s %s\n", cyan("installed:"), green(pkg.FullVersion()))
			} else {
				fmt.Printf("  %s %s\n", cyan("installed:"), dim("no"))
			}

			if page := fetchTldrPage(displayName); page != "" {
				fmt.Println()
				renderTldrPage(page)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cask, "cask", false, "Show cask info")
	return cmd
}

func fetchTldrPage(name string) string {
	platform := "common"
	if runtime.GOOS == "darwin" {
		platform = "osx"
	} else if runtime.GOOS == "linux" {
		platform = "linux"
	}

	bases := []string{
		"https://raw.githubusercontent.com/tldr-pages/tldr/main/pages/" + platform + "/",
		"https://raw.githubusercontent.com/tldr-pages/tldr/main/pages/common/",
	}

	for _, base := range bases {
		resp, err := http.Get(base + name + ".md")
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			return string(body)
		}
	}

	return ""
}

func renderTldrPage(page string) {
	for _, line := range strings.Split(page, "\n") {
		line = strings.TrimRight(line, "\r")

		switch {
		case strings.HasPrefix(line, "# "):
			// skip title, already shown above
		case strings.HasPrefix(line, "> "):
			text := strings.TrimPrefix(line, "> ")
			text = strings.TrimPrefix(text, "More information: ")
			fmt.Printf("  %s\n", dim(text))
		case strings.HasPrefix(line, "- "):
			text := strings.TrimPrefix(line, "- ")
			fmt.Printf("\n  %s\n", text)
		case strings.HasPrefix(line, "`") && strings.HasSuffix(line, "`"):
			code := line[1 : len(line)-1]
			code = renderPlaceholders(code)
			fmt.Printf("    %s\n", cyan(code))
		}
	}
}

func renderPlaceholders(code string) string {
	var result strings.Builder
	i := 0
	for i < len(code) {
		if i+1 < len(code) && code[i] == '{' && code[i+1] == '{' {
			end := strings.Index(code[i:], "}}")
			if end != -1 {
				placeholder := code[i+2 : i+end]
				result.WriteString("<" + placeholder + ">")
				i += end + 2
				continue
			}
		}
		result.WriteByte(code[i])
		i++
	}
	return result.String()
}
