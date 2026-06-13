package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) latestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "latest",
		Short: "Show the most recent articles from the High Scalability feed",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching latest %d articles...", n)
			articles, err := a.client.Latest(cmd.Context(), n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(articles, len(articles))
		},
	}
	return cmd
}
