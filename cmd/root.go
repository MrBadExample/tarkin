package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/tarkin/internal/db"
	"github.com/yourusername/tarkin/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "tarkin",
	Short: "☠  mission control for your agents",
	Long:  `Tarkin — track tasks, capture ideas, and monitor your AI agents from the terminal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

func Execute() {
	if err := db.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "error: could not init db: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
