package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/yourusername/tarkin/internal/models"
)

// ── Colors ───────────────────────────────────────────────────────────────────

var (
	Bold      = color.New(color.Bold)
	Muted     = color.New(color.FgHiBlack)
	Success   = color.New(color.FgGreen)
	Warning   = color.New(color.FgYellow)
	Danger    = color.New(color.FgRed)
	Info      = color.New(color.FgCyan)
	Purple    = color.New(color.FgMagenta)
)

// ── Header ───────────────────────────────────────────────────────────────────

func PrintHeader() {
	fmt.Println()
	Bold.Println("  ☠  tarkin")
	Muted.Println("  Grand Moff — mission control for your agents")
	fmt.Println()
}

func PrintSuccess(msg string) {
	Success.Printf("  ✓  %s\n\n", msg)
}

func PrintError(msg string) {
	Danger.Printf("  ✗  %s\n\n", msg)
}

func PrintInfo(msg string) {
	Info.Printf("  →  %s\n\n", msg)
}

// ── Tasks ────────────────────────────────────────────────────────────────────

func PrintTasks(tasks []models.Task) {
	if len(tasks) == 0 {
		Muted.Println("  no tasks yet. run: tarkin add \"your task\"")
		fmt.Println()
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"#", "task", "status", "priority", "agent"})
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	for _, t := range tasks {
		table.Append([]string{
			strconv.Itoa(t.ID),
			truncate(t.Title, 48),
			statusLabel(t.Status),
			string(t.Priority),
			agentLabel(t.Agent),
		})
	}

	fmt.Println()
	table.Render()
	fmt.Println()
}

func PrintTask(t models.Task) {
	fmt.Println()
	Bold.Printf("  #%d  %s\n", t.ID, t.Title)
	fmt.Printf("  status    %s\n", statusLabel(t.Status))
	fmt.Printf("  priority  %s\n", t.Priority)
	fmt.Printf("  agent     %s\n", agentLabel(t.Agent))
	if t.Notes != "" {
		fmt.Printf("  notes     %s\n", t.Notes)
	}
	Muted.Printf("  created   %s\n", humanTime(t.CreatedAt))
	fmt.Println()
}

// ── Ideas ────────────────────────────────────────────────────────────────────

func PrintIdeas(ideas []models.Idea) {
	if len(ideas) == 0 {
		Muted.Println("  no ideas yet. run: tarkin idea \"your idea\"")
		fmt.Println()
		return
	}

	fmt.Println()
	for _, i := range ideas {
		prefix := "  ·"
		if i.Promoted {
			prefix = "  ✓"
			Muted.Printf("%s  #%d  %s", prefix, i.ID, i.Title)
			Muted.Printf("  → task #%d\n", i.TaskID)
		} else {
			fmt.Printf("%s  ", prefix)
			Bold.Printf("#%d", i.ID)
			fmt.Printf("  %s\n", i.Title)
			if i.Notes != "" {
				Muted.Printf("       %s\n", i.Notes)
			}
		}
	}
	fmt.Println()
}

// ── Agents ───────────────────────────────────────────────────────────────────

func PrintAgents(agents []models.Agent) {
	if len(agents) == 0 {
		Muted.Println("  no agents registered. run: tarkin agent add <codename> <tool>")
		fmt.Println()
		return
	}

	fmt.Println()
	for _, a := range agents {
		dot := onlineDot(a.Status)
		fmt.Printf("  %s  ", dot)
		Bold.Printf("%-12s", a.Codename)
		Muted.Printf("  %-14s", a.Tool)
		if a.CurrentTask != "" {
			fmt.Printf("  %s", truncate(a.CurrentTask, 40))
		} else {
			Muted.Printf("  idle")
		}
		fmt.Println()
	}
	fmt.Println()
}

// ── Activity ─────────────────────────────────────────────────────────────────

func PrintActivity(entries []struct {
	Message   string
	Agent     string
	CreatedAt time.Time
}) {
	if len(entries) == 0 {
		Muted.Println("  no activity yet.")
		fmt.Println()
		return
	}

	fmt.Println()
	for _, e := range entries {
		Muted.Printf("  %-14s", humanTime(e.CreatedAt))
		if e.Agent != "" {
			Purple.Printf("  %-10s", e.Agent)
		} else {
			fmt.Printf("  %-10s", "")
		}
		fmt.Printf("  %s\n", e.Message)
	}
	fmt.Println()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func statusLabel(s models.Status) string {
	switch s {
	case models.StatusInProgress:
		return Info.Sprint("in progress")
	case models.StatusDone:
		return Success.Sprint("done")
	case models.StatusBlocked:
		return Danger.Sprint("blocked")
	default:
		return Muted.Sprint("backlog")
	}
}

func agentLabel(agent string) string {
	if agent == "" {
		return Muted.Sprint("unassigned")
	}
	return Purple.Sprint(agent)
}

func onlineDot(status string) string {
	switch status {
	case "online":
		return Success.Sprint("●")
	case "busy":
		return Warning.Sprint("●")
	default:
		return Muted.Sprint("○")
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func humanTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("Jan 2")
	}
}

func SectionTitle(title string) {
	fmt.Println()
	fmt.Printf("  %s\n", strings.ToUpper(title))
	Muted.Printf("  %s\n", strings.Repeat("─", 40))
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func PrintDashboard(tasks []models.Task, ideas []models.Idea, agents []models.Agent, activity []struct {
	Message   string
	Agent     string
	CreatedAt time.Time
}) {
	printDashboardTasks(tasks)
	printDashboardIdeas(ideas)
	if len(agents) > 0 {
		printDashboardAgents(agents)
	}
	printDashboardActivity(activity)
}

func printDashboardTasks(tasks []models.Task) {
	done := 0
	for _, t := range tasks {
		if t.Status == models.StatusDone {
			done++
		}
	}

	fmt.Println()
	Bold.Printf("  ACTIVE TASKS")
	if len(tasks) > 0 {
		Muted.Printf("  %d of %d done\n", done, len(tasks))
	} else {
		fmt.Println()
	}
	Muted.Printf("  %s\n", strings.Repeat("─", 40))

	if len(tasks) == 0 {
		Muted.Println("  no tasks yet. run: tarkin add \"your task\"")
		fmt.Println()
		return
	}

	for _, t := range tasks {
		var check string
		if t.Status == models.StatusDone {
			check = Success.Sprint("✓")
			Muted.Printf("  %s  %s", check, truncate(t.Title, 44))
		} else if t.Status == models.StatusInProgress {
			check = Info.Sprint("→")
			fmt.Printf("  %s  ", check)
			Bold.Printf("%s", truncate(t.Title, 44))
		} else if t.Status == models.StatusBlocked {
			check = Danger.Sprint("!")
			fmt.Printf("  %s  %s", check, truncate(t.Title, 44))
		} else {
			check = Muted.Sprint("·")
			fmt.Printf("  %s  %s", check, truncate(t.Title, 44))
		}

		meta := []string{}
		if t.Agent != "" {
			meta = append(meta, Purple.Sprint(t.Agent))
		}
		if t.Status != models.StatusDone {
			meta = append(meta, Muted.Sprint(string(t.Priority)))
		}
		if len(meta) > 0 {
			fmt.Printf("  %s", strings.Join(meta, Muted.Sprint(" · ")))
		}
		fmt.Println()
	}
	fmt.Println()
}

func printDashboardIdeas(ideas []models.Idea) {
	var unpromoted []models.Idea
	for _, i := range ideas {
		if !i.Promoted {
			unpromoted = append(unpromoted, i)
		}
	}

	fmt.Println()
	Bold.Printf("  IDEAS")
	Muted.Printf("  %d captured\n", len(unpromoted))
	Muted.Printf("  %s\n", strings.Repeat("─", 40))

	if len(unpromoted) == 0 {
		Muted.Println("  no ideas yet. run: tarkin idea \"your idea\"")
		fmt.Println()
		return
	}

	limit := unpromoted
	if len(limit) > 5 {
		limit = limit[:5]
	}
	for _, i := range limit {
		fmt.Printf("  ")
		Purple.Printf("#%d", i.ID)
		fmt.Printf("  %s\n", i.Title)
		if i.Notes != "" {
			Muted.Printf("       %s\n", truncate(i.Notes, 60))
		}
	}
	if len(unpromoted) > 5 {
		Muted.Printf("  … and %d more\n", len(unpromoted)-5)
	}
	fmt.Println()
}

func printDashboardAgents(agents []models.Agent) {
	online := 0
	for _, a := range agents {
		if a.Status == "online" || a.Status == "busy" {
			online++
		}
	}

	fmt.Println()
	Bold.Printf("  AGENTS")
	if online > 0 {
		Success.Printf("  %d online\n", online)
	} else {
		Muted.Printf("  all offline\n")
	}
	Muted.Printf("  %s\n", strings.Repeat("─", 40))

	for _, a := range agents {
		dot := onlineDot(a.Status)
		fmt.Printf("  %s  ", dot)
		Bold.Printf("%-12s", a.Codename)
		if a.CurrentTask != "" {
			fmt.Printf("  %s", truncate(a.CurrentTask, 40))
		} else {
			Muted.Printf("  idle")
		}
		fmt.Println()
	}
	fmt.Println()
}

func printDashboardActivity(activity []struct {
	Message   string
	Agent     string
	CreatedAt time.Time
}) {
	fmt.Println()
	Bold.Println("  ACTIVITY")
	Muted.Printf("  %s\n", strings.Repeat("─", 40))

	if len(activity) == 0 {
		Muted.Println("  no activity yet.")
		fmt.Println()
		return
	}

	for _, e := range activity {
		Muted.Printf("  %-14s", humanTime(e.CreatedAt))
		if e.Agent != "" {
			Purple.Printf("  %-10s", e.Agent)
		} else {
			fmt.Printf("  %-10s", "")
		}
		fmt.Printf("  %s\n", e.Message)
	}
	fmt.Println()
}
