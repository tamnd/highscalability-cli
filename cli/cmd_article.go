package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) articleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "article <url>",
		Short: "Fetch and display a single High Scalability article by URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.progressf("fetching article %s...", args[0])
			article, err := a.client.Article(cmd.Context(), args[0])
			if err != nil {
				return mapFetchErr(err)
			}
			return a.render(article)
		},
	}
	return cmd
}
