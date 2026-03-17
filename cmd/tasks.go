package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/tarkin/internal/db"
	"github.com/yourusername/tarkin/internal/models"
	"github.com/yourusername/tarkin/internal/ui"
)

var (
	flagPriority string
	flagAgent    string
	flagStatus   string
)

func init() {
	// tarkin add
	addCmd.Flags().StringVarP(&flagPriority, "priority", "p", "medium", "priority: low | medium | high")
	addCmd.Flags().StringVarP(&flagAgent, "agent", "a", "", "assign to agent (e.g. vader)")
	rootCmd.AddCommand(addCmd)

	// tarkin ls
	lsCmd.Flags().StringVarP(&flagStatus, "status", "s", "", "filter by status: backlog | in_progress | done | blocked")
	rootCmd.AddCommand(lsCmd)

	// tarkin done / block / start
	rootCmd.AddCommand(doneCmd)
	rootCmd.AddCommand(blockCmd)
	rootCmd.AddCommand(startCmd)

	// tarkin assign
	rootCmd.AddCommand(assignCmd)

	// tarkin show
	rootCmd.AddCommand(showCmd)

	// tarkin rm
	rootCmd.AddCommand(rmCmd)
}

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "add a new task",
	Example: `  tarkin add "build auth flow"
  tarkin add "refactor db layer" --priority high --agent vader`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := strings.Join(args, " ")
		t, err := db.CreateTask(title, flagPriority, flagAgent)
		if err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("task #%d added: %s", t.ID, t.Title))
		ui.PrintTask(t)
		return nil
	},
}

var lsCmd = &cobra.Command{
	Use:     "ls",
	Short:   "list tasks",
	Aliases: []string{"list", "tasks"},
	Example: `  tarkin ls
  tarkin ls --status backlog
  tarkin ls -s done`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tasks, err := db.ListTasks(flagStatus)
		if err != nil {
			return err
		}
		ui.PrintTasks(tasks)
		return nil
	},
}

var doneCmd = &cobra.Command{
	Use:     "done <id>",
	Short:   "mark a task as done",
	Example: `  tarkin done 4`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %s", args[0])
		}
		if err := db.UpdateTaskStatus(id, models.StatusDone); err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("task #%d marked as done", id))
		return nil
	},
}

var blockCmd = &cobra.Command{
	Use:     "block <id>",
	Short:   "mark a task as blocked",
	Example: `  tarkin block 4`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %s", args[0])
		}
		if err := db.UpdateTaskStatus(id, models.StatusBlocked); err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("task #%d marked as blocked", id))
		return nil
	},
}

var startCmd = &cobra.Command{
	Use:     "start <id>",
	Short:   "mark a task as in progress",
	Example: `  tarkin start 4`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %s", args[0])
		}
		if err := db.UpdateTaskStatus(id, models.StatusInProgress); err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("task #%d started", id))
		return nil
	},
}

var assignCmd = &cobra.Command{
	Use:     "assign <id> <agent>",
	Short:   "assign a task to an agent",
	Example: `  tarkin assign 4 vader
  tarkin assign 7 ackbar`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %s", args[0])
		}
		agent := args[1]
		if err := db.AssignTask(id, agent); err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("task #%d assigned to %s", id, agent))
		return nil
	},
}

var showCmd = &cobra.Command{
	Use:     "show <id>",
	Short:   "show task details",
	Example: `  tarkin show 4`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %s", args[0])
		}
		t, err := db.GetTask(id)
		if err != nil {
			return fmt.Errorf("task #%d not found", id)
		}
		ui.PrintTask(t)
		return nil
	},
}

var rmCmd = &cobra.Command{
	Use:     "rm <id>",
	Short:   "delete a task",
	Example: `  tarkin rm 4`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid task id: %s", args[0])
		}
		if err := db.DeleteTask(id); err != nil {
			return err
		}
		ui.PrintSuccess(fmt.Sprintf("task #%d deleted", id))
		return nil
	},
}
