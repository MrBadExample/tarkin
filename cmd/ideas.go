package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/tarkin/internal/db"
	"github.com/yourusername/tarkin/internal/ui"
)

func init() {
	ideaCmd.Flags().StringP("notes", "n", "", "optional notes")
	rootCmd.AddCommand(ideaCmd)
	rootCmd.AddCommand(ideasCmd)

	promoteCmd.Flags().StringVarP(&flagPriority, "priority", "p", "medium", "priority: low | medium | high")
	promoteCmd.Flags().StringVarP(&flagAgent, "agent", "a", "", "assign to agent immediately")
	rootCmd.AddCommand(promoteCmd)
}

var ideaCmd = &cobra.Command{
	Use:   "idea <title>",
	Short: "capture a new idea",
	Example: `  tarkin idea "quick capture hotkey"
  tarkin idea "inter-agent messaging bus" --notes "vader passes context to ackbar on handoff"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := strings.Join(args, " ")
		notes, _ := cmd.Flags().GetString("notes")
		i, err := db.CreateIdea(title, notes)
		if err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("idea #%d captured: %s", i.ID, i.Title))
		return nil
	},
}

var ideasCmd = &cobra.Command{
	Use:     "ideas",
	Short:   "list all ideas",
	Aliases: []string{"backlog"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ideas, err := db.ListIdeas()
		if err != nil {
			return err
		}
		ui.PrintIdeas(ideas)
		return nil
	},
}

var promoteCmd = &cobra.Command{
	Use:   "promote <idea-id>",
	Short: "promote an idea to a task",
	Example: `  tarkin promote 3
  tarkin promote 3 --priority high --agent vader`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid idea id: %s", args[0])
		}
		t, err := db.PromoteIdea(id, flagPriority, flagAgent)
		if err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("idea #%d promoted → task #%d: %s", id, t.ID, t.Title))
		ui.PrintTask(t)
		return nil
	},
}
