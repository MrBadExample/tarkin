package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yourusername/tarkin/internal/db"
	"github.com/yourusername/tarkin/internal/ui"
)

func init() {
	logCmd.Flags().IntP("limit", "n", 20, "number of entries to show")
	rootCmd.AddCommand(logCmd)
}

var logCmd = &cobra.Command{
	Use:     "log",
	Short:   "show activity log",
	Aliases: []string{"history", "feed"},
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		entries, err := db.ListActivity(limit)
		if err != nil {
			return err
		}
		ui.PrintActivity(entries)
		return nil
	},
}
