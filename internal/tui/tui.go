package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourusername/tarkin/internal/db"
	"github.com/yourusername/tarkin/internal/models"
)

// ── Views & modes ─────────────────────────────────────────────────────────────

type view int

const (
	viewBoard view = iota
	viewIdeas
	viewLog
	viewTrash
	viewTaskDetail
	viewIdeaDetail
)

type inputMode int

const (
	modeNone inputMode = iota
	modeAddTask
	modeAddIdea
	modeAddComment
	modeEditTaskTitle
	modeEditTaskDesc
	modeEditIdeaTitle
	modeEditIdeaDesc
)

// task detail field indices
const (
	fieldTitle        = 0
	fieldStatus       = 1
	fieldPriority     = 2
	fieldDescription  = 3
	fieldAddComment   = 4
	fieldCommentsBase = 5
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	sPurple = lipgloss.NewStyle().Foreground(lipgloss.Color("#7F77DD"))
	sGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("#1D9E75"))
	sBlue   = lipgloss.NewStyle().Foreground(lipgloss.Color("#378ADD"))
	sRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0392B"))
	sMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5f5e5a"))
	sDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9a9890"))
	sBold   = lipgloss.NewStyle().Foreground(lipgloss.Color("#e8e6df")).Bold(true)
	sNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("#e8e6df"))
	sDone   = lipgloss.NewStyle().Foreground(lipgloss.Color("#5f5e5a")).Strikethrough(true)

	sidebarStyle = lipgloss.NewStyle().Background(lipgloss.Color("#141412"))
	contentStyle = lipgloss.NewStyle().PaddingLeft(1)
	overlayStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1c1c1a")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7F77DD")).
			Padding(1, 3)
)

// ── Column definitions ────────────────────────────────────────────────────────

type column struct {
	status models.Status
	label  string
	color  lipgloss.Style
}

var columns = []column{
	{models.StatusBacklog, "TO DO", sMuted},
	{models.StatusInProgress, "IN PROGRESS", sBlue},
	{models.StatusBlocked, "BLOCKED", sRed},
	{models.StatusDone, "DONE", sGreen},
}

// ── Data ─────────────────────────────────────────────────────────────────────

type activityEntry struct {
	Message   string
	CreatedAt time.Time
}

type appData struct {
	tasks        []models.Task
	ideas        []models.Idea
	activity     []activityEntry
	trashedTasks []models.Task
}

type dataLoadedMsg appData
type commentsLoadedMsg []models.Comment
type errMsg error

func loadData() tea.Msg {
	tasks, err := db.ListTasks("")
	if err != nil {
		return errMsg(err)
	}
	ideas, err := db.ListIdeas()
	if err != nil {
		return errMsg(err)
	}
	raw, err := db.ListActivity(30)
	if err != nil {
		return errMsg(err)
	}
	var activity []activityEntry
	for _, e := range raw {
		activity = append(activity, activityEntry{e.Message, e.CreatedAt})
	}
	trashed, err := db.ListTrashedTasks()
	if err != nil {
		return errMsg(err)
	}
	return dataLoadedMsg{tasks, ideas, activity, trashed}
}

func loadComments(taskID int) tea.Cmd {
	return func() tea.Msg {
		comments, err := db.ListComments(taskID)
		if err != nil {
			return errMsg(err)
		}
		return commentsLoadedMsg(comments)
	}
}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	width, height int
	currentView   view
	prevView      view
	data          appData
	loaded        bool
	err           error

	boardCol int
	boardRow int

	ideaCursor   int
	logCursor    int
	trashCursor  int
	detailCursor int // 0=title, 1=priority, 2=desc, 3+=comments

	detailTask *models.Task
	comments   []models.Comment
	detailIdea *models.Idea

	inputMode inputMode
	input     textinput.Model
	statusMsg string

	confirmQuit bool
	showHelp    bool
}

func New() Model {
	ti := textinput.New()
	ti.CharLimit = 500
	return Model{input: ti}
}

func (m Model) Init() tea.Cmd { return loadData }

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case dataLoadedMsg:
		m.data = appData(msg)
		m.loaded = true
		m.clampCursors()
		m.refreshDetailPointers()
	case commentsLoadedMsg:
		m.comments = []models.Comment(msg)
		m.clampDetailCursor()
	case errMsg:
		m.err = error(msg)
	case tea.KeyMsg:
		if m.inputMode != modeNone {
			return m.handleInput(msg)
		}
		if m.confirmQuit {
			return m.handleConfirmQuit(msg)
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		return m.handleNav(msg)
	}
	return m, nil
}

func (m *Model) clampCursors() {
	if col := m.colTasks(m.boardCol); m.boardRow >= len(col) {
		m.boardRow = maxInt(0, len(col)-1)
	}
	if up := unpromotedIdeas(m.data.ideas); m.ideaCursor >= len(up) {
		m.ideaCursor = maxInt(0, len(up)-1)
	}
	if m.logCursor >= len(m.data.activity) {
		m.logCursor = maxInt(0, len(m.data.activity)-1)
	}
	if m.trashCursor >= len(m.data.trashedTasks) {
		m.trashCursor = maxInt(0, len(m.data.trashedTasks)-1)
	}
	m.clampDetailCursor()
}

func (m *Model) clampDetailCursor() {
	top := fieldAddComment
	if len(m.comments) > 0 {
		top = fieldCommentsBase + len(m.comments) - 1
	}
	if m.detailCursor > top {
		m.detailCursor = maxInt(0, top)
	}
}

func (m *Model) refreshDetailPointers() {
	if m.detailTask != nil {
		for i := range m.data.tasks {
			if m.data.tasks[i].ID == m.detailTask.ID {
				m.detailTask = &m.data.tasks[i]
				return
			}
		}
	}
	if m.detailIdea != nil {
		for i := range m.data.ideas {
			if m.data.ideas[i].ID == m.detailIdea.ID {
				m.detailIdea = &m.data.ideas[i]
				return
			}
		}
	}
}

// ── Input mode ────────────────────────────────────────────────────────────────

func (m Model) handleInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = modeNone
		m.input.Blur()
		m.statusMsg = ""
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.input.Value())
		mode := m.inputMode
		m.inputMode = modeNone
		m.input.Blur()
		if val == "" {
			return m, nil
		}
		switch mode {
		case modeAddTask:
			t, err := db.CreateTask(val, "medium", "")
			if err == nil {
				m.statusMsg = fmt.Sprintf("added #%d", t.ID)
			}
			return m, loadData
		case modeAddIdea:
			i, err := db.CreateIdea(val, "")
			if err == nil {
				m.statusMsg = fmt.Sprintf("captured idea #%d", i.ID)
			}
			return m, loadData
		case modeAddComment:
			if m.detailTask != nil {
				db.AddComment(m.detailTask.ID, val)
				m.statusMsg = "comment added"
				return m, tea.Batch(loadData, loadComments(m.detailTask.ID))
			}
		case modeEditTaskTitle:
			if m.detailTask != nil {
				db.UpdateTaskTitle(m.detailTask.ID, val)
				m.statusMsg = "saved"
				return m, tea.Batch(loadData, loadComments(m.detailTask.ID))
			}
		case modeEditTaskDesc:
			if m.detailTask != nil {
				db.UpdateTaskDescription(m.detailTask.ID, val)
				m.statusMsg = "saved"
				return m, tea.Batch(loadData, loadComments(m.detailTask.ID))
			}
		case modeEditIdeaTitle:
			if m.detailIdea != nil {
				db.UpdateIdea(m.detailIdea.ID, val, m.detailIdea.Notes)
				m.statusMsg = "saved"
				return m, loadData
			}
		case modeEditIdeaDesc:
			if m.detailIdea != nil {
				db.UpdateIdea(m.detailIdea.ID, m.detailIdea.Title, val)
				m.statusMsg = "saved"
				return m, loadData
			}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// ── Quit confirmation ─────────────────────────────────────────────────────────

func (m Model) handleConfirmQuit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y":
		return m, tea.Quit
	default:
		m.confirmQuit = false
	}
	return m, nil
}

// ── Navigation ────────────────────────────────────────────────────────────────

func (m Model) handleNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "q":
		m.confirmQuit = true
		return m, nil

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "1":
		m.currentView = viewBoard
	case "2":
		m.currentView = viewIdeas
	case "3":
		m.currentView = viewLog
	case "4":
		m.currentView = viewTrash

	case "esc":
		switch m.currentView {
		case viewTaskDetail, viewIdeaDetail:
			m.currentView = m.prevView
			m.detailTask = nil
			m.detailIdea = nil
			m.comments = nil
			m.detailCursor = 0
		}

	case "j", "down":
		m.cursorDown()
	case "k", "up":
		m.cursorUp()

	case "h", "left":
		switch m.currentView {
		case viewBoard:
			if m.boardCol > 0 {
				m.boardCol--
				m.boardRow = 0
			}
		case viewTaskDetail:
			if m.detailTask != nil {
				switch m.detailCursor {
				case fieldStatus:
					s := prevStatus(m.detailTask.Status)
					db.UpdateTaskStatus(m.detailTask.ID, s)
					m.detailTask.Status = s
					return m, loadData
				case fieldPriority:
					p := prevPriority(m.detailTask.Priority)
					db.UpdateTaskPriority(m.detailTask.ID, p)
					m.detailTask.Priority = p
					return m, loadData
				}
			}
		}

	case "l", "right":
		switch m.currentView {
		case viewBoard:
			if m.boardCol < len(columns)-1 {
				m.boardCol++
				m.boardRow = 0
			}
		case viewTaskDetail:
			if m.detailTask != nil {
				switch m.detailCursor {
				case fieldStatus:
					s := nextStatus(m.detailTask.Status)
					db.UpdateTaskStatus(m.detailTask.ID, s)
					m.detailTask.Status = s
					return m, loadData
				case fieldPriority:
					p := nextPriority(m.detailTask.Priority)
					db.UpdateTaskPriority(m.detailTask.ID, p)
					m.detailTask.Priority = p
					return m, loadData
				}
			}
		}

	case "enter":
		switch m.currentView {
		case viewBoard:
			if t := m.selectedTask(); t != nil {
				m.prevView = viewBoard
				m.currentView = viewTaskDetail
				m.detailTask = t
				m.detailCursor = 0
				return m, loadComments(t.ID)
			}
		case viewIdeas:
			if up := unpromotedIdeas(m.data.ideas); m.ideaCursor < len(up) {
				m.prevView = viewIdeas
				m.currentView = viewIdeaDetail
				idea := up[m.ideaCursor]
				m.detailIdea = &idea
				m.detailCursor = 0
			}
		case viewTaskDetail:
			return m.handleTaskDetailEnter()
		case viewIdeaDetail:
			return m.handleIdeaDetailEnter()
		}

	case "a":
		switch m.currentView {
		case viewBoard, viewTaskDetail:
			return m.startInput(modeAddTask, "new task: ")
		case viewIdeas, viewIdeaDetail:
			return m.startInput(modeAddIdea, "new idea: ")
		}

	case "c":
		if m.currentView == viewTaskDetail {
			return m.startInput(modeAddComment, "comment: ")
		}

	case "p":
		if m.currentView == viewIdeas {
			if up := unpromotedIdeas(m.data.ideas); m.ideaCursor < len(up) {
				idea := up[m.ideaCursor]
				task, err := db.PromoteIdea(idea.ID, "medium", "")
				if err == nil {
					m.statusMsg = fmt.Sprintf("#%d → task #%d", idea.ID, task.ID)
					return m, loadData
				}
			}
		}

	case "X":
		if m.currentView == viewTrash {
			if m.trashCursor < len(m.data.trashedTasks) {
				t := m.data.trashedTasks[m.trashCursor]
				db.PermanentDeleteTask(t.ID)
				m.statusMsg = fmt.Sprintf("permanently deleted #%d", t.ID)
				return m, loadData
			}
		}

	case "x":
		switch m.currentView {
		case viewBoard:
			if t := m.selectedTask(); t != nil {
				db.DeleteTask(t.ID)
				m.statusMsg = fmt.Sprintf("moved #%d to trash", t.ID)
				return m, loadData
			}
		case viewIdeas:
			if up := unpromotedIdeas(m.data.ideas); m.ideaCursor < len(up) {
				idea := up[m.ideaCursor]
				db.DeleteIdea(idea.ID)
				m.statusMsg = fmt.Sprintf("deleted idea #%d", idea.ID)
				return m, loadData
			}
		case viewTaskDetail:
			if m.detailCursor >= fieldCommentsBase {
				ci := m.detailCursor - fieldCommentsBase
				if ci < len(m.comments) {
					db.DeleteComment(m.comments[ci].ID)
					m.statusMsg = "comment deleted"
					return m, tea.Batch(loadData, loadComments(m.detailTask.ID))
				}
			}
		case viewIdeaDetail:
			if m.detailIdea != nil {
				db.DeleteIdea(m.detailIdea.ID)
				m.currentView = viewIdeas
				m.detailIdea = nil
				m.statusMsg = "idea deleted"
				return m, loadData
			}
		}

	// board quick-status shortcuts (power user, shown in help)
	case "s":
		if t := m.selectedTask(); t != nil {
			db.UpdateTaskStatus(t.ID, models.StatusInProgress)
			m.boardCol, m.boardRow = 1, 0
			m.statusMsg = fmt.Sprintf("started #%d", t.ID)
			return m, loadData
		}
	case "d":
		if m.currentView == viewBoard {
			if t := m.selectedTask(); t != nil {
				db.UpdateTaskStatus(t.ID, models.StatusDone)
				m.boardCol, m.boardRow = 3, 0
				m.statusMsg = fmt.Sprintf("done #%d", t.ID)
				return m, loadData
			}
		}
	case "b":
		if t := m.selectedTask(); t != nil {
			db.UpdateTaskStatus(t.ID, models.StatusBlocked)
			m.boardCol, m.boardRow = 2, 0
			m.statusMsg = fmt.Sprintf("blocked #%d", t.ID)
			return m, loadData
		}
	case "u":
		if t := m.selectedTask(); t != nil {
			db.UpdateTaskStatus(t.ID, models.StatusBacklog)
			m.boardCol, m.boardRow = 0, 0
			return m, loadData
		}

	case "r":
		if m.currentView == viewTrash {
			if m.trashCursor < len(m.data.trashedTasks) {
				t := m.data.trashedTasks[m.trashCursor]
				db.RestoreTask(t.ID)
				m.statusMsg = fmt.Sprintf("restored #%d", t.ID)
				return m, loadData
			}
		} else {
			m.loaded = false
			m.statusMsg = ""
			cmds := []tea.Cmd{loadData}
			if m.currentView == viewTaskDetail && m.detailTask != nil {
				cmds = append(cmds, loadComments(m.detailTask.ID))
			}
			return m, tea.Batch(cmds...)
		}
	}

	return m, nil
}

func (m Model) handleTaskDetailEnter() (Model, tea.Cmd) {
	if m.detailTask == nil {
		return m, nil
	}
	switch m.detailCursor {
	case fieldTitle:
		return m.startInputWithValue(modeEditTaskTitle, "title: ", m.detailTask.Title)
	case fieldStatus:
		next := nextStatus(m.detailTask.Status)
		db.UpdateTaskStatus(m.detailTask.ID, next)
		m.detailTask.Status = next
		return m, loadData
	case fieldPriority:
		next := nextPriority(m.detailTask.Priority)
		db.UpdateTaskPriority(m.detailTask.ID, next)
		m.detailTask.Priority = next
		return m, loadData
	case fieldDescription:
		return m.startInputWithValue(modeEditTaskDesc, "description: ", m.detailTask.Description)
	case fieldAddComment:
		return m.startInput(modeAddComment, "comment: ")
	}
	return m, nil
}

func (m Model) handleIdeaDetailEnter() (Model, tea.Cmd) {
	if m.detailIdea == nil {
		return m, nil
	}
	switch m.detailCursor {
	case fieldTitle:
		return m.startInputWithValue(modeEditIdeaTitle, "title: ", m.detailIdea.Title)
	case fieldDescription:
		return m.startInputWithValue(modeEditIdeaDesc, "description: ", m.detailIdea.Notes)
	}
	return m, nil
}

func (m *Model) cursorDown() {
	switch m.currentView {
	case viewBoard:
		if col := m.colTasks(m.boardCol); m.boardRow < len(col)-1 {
			m.boardRow++
		}
	case viewIdeas:
		if up := unpromotedIdeas(m.data.ideas); m.ideaCursor < len(up)-1 {
			m.ideaCursor++
		}
	case viewLog:
		if m.logCursor < len(m.data.activity)-1 {
			m.logCursor++
		}
	case viewTrash:
		if m.trashCursor < len(m.data.trashedTasks)-1 {
			m.trashCursor++
		}
	case viewTaskDetail:
		top := fieldAddComment
		if len(m.comments) > 0 {
			top = fieldCommentsBase + len(m.comments) - 1
		}
		if m.detailCursor < top {
			m.detailCursor++
		}
	case viewIdeaDetail:
		if m.detailCursor < fieldDescription {
			m.detailCursor++
		}
	}
}

func (m *Model) cursorUp() {
	switch m.currentView {
	case viewBoard:
		if m.boardRow > 0 {
			m.boardRow--
		}
	case viewIdeas:
		if m.ideaCursor > 0 {
			m.ideaCursor--
		}
	case viewLog:
		if m.logCursor > 0 {
			m.logCursor--
		}
	case viewTrash:
		if m.trashCursor > 0 {
			m.trashCursor--
		}
	case viewTaskDetail, viewIdeaDetail:
		if m.detailCursor > 0 {
			m.detailCursor--
		}
	}
}

func (m Model) selectedTask() *models.Task {
	if m.currentView != viewBoard {
		return nil
	}
	col := m.colTasks(m.boardCol)
	if len(col) == 0 || m.boardRow >= len(col) {
		return nil
	}
	t := col[m.boardRow]
	return &t
}

func (m Model) colTasks(colIdx int) []models.Task {
	status := columns[colIdx].status
	var out []models.Task
	for _, t := range m.data.tasks {
		if t.Status == status {
			out = append(out, t)
		}
	}
	return out
}

func (m Model) startInput(mode inputMode, prompt string) (Model, tea.Cmd) {
	return m.startInputWithValue(mode, prompt, "")
}

func (m Model) startInputWithValue(mode inputMode, prompt, value string) (Model, tea.Cmd) {
	m.inputMode = mode
	m.input.Reset()
	m.input.SetValue(value)
	m.input.Placeholder = ""
	m.input.Prompt = sPurple.Render("❯ ") + sDim.Render(prompt)
	m.input.Focus()
	return m, textinput.Blink
}

// ── View ─────────────────────────────────────────────────────────────────────

const sidebarW = 18
const logFooterH = 4

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nerror: %v\n\nq to quit\n", m.err)
	}
	if !m.loaded {
		return sMuted.Render("\n  loading…")
	}

	showFooter := m.currentView != viewLog
	mainH := m.height
	if showFooter {
		mainH = m.height - logFooterH
	}

	contentW := m.width - sidebarW - 1
	sidebar := sidebarStyle.Width(sidebarW).Height(mainH).Render(m.renderSidebar())
	content := contentStyle.Width(contentW).Height(mainH).Render(m.renderContentWithHints(contentW, mainH))
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	if m.showHelp {
		main = m.renderHelpOverlay(main)
	}

	if !showFooter {
		return main
	}
	return lipgloss.JoinVertical(lipgloss.Left, main, m.renderLogFooter())
}

func (m Model) renderLogFooter() string {
	var b strings.Builder
	b.WriteString(sMuted.Render(strings.Repeat("─", m.width)) + "\n")
	entries := m.data.activity
	max := logFooterH - 1
	if len(entries) < max {
		max = len(entries)
	}
	for i := 0; i < max; i++ {
		e := entries[i]
		time := sDim.Render(fmt.Sprintf("%-10s", humanTime(e.CreatedAt)))
		msg := sMuted.Render(truncate(e.Message, m.width-14))
		b.WriteString("  " + time + "  " + msg + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderContentWithHints(w, h int) string {
	inner := m.renderContent()
	lines := strings.Split(strings.TrimRight(inner, "\n"), "\n")

	// position hint at 75% down or just below content, whichever is lower
	naturalRow := len(lines) + 1
	preferredRow := (h * 3) / 4
	hintRow := naturalRow
	if preferredRow > hintRow {
		hintRow = preferredRow
	}
	if hintRow >= h {
		hintRow = h - 1
	}

	var hintLine string
	switch {
	case m.confirmQuit:
		hintLine = centerIn(hint("↵", "confirm quit")+sep()+hint("esc", "cancel"), w)
	case m.inputMode != modeNone:
		hintLine = m.input.View()
		hintRow = h - 1
	default:
		hintLine = centerIn(m.contextHints(), w)
	}

	result := make([]string, h)
	for i := 0; i < h; i++ {
		if i < len(lines) {
			result[i] = lines[i]
		}
	}
	if hintRow >= 0 && hintRow < h {
		result[hintRow] = hintLine
	}

	return strings.Join(result, "\n")
}

func centerIn(s string, w int) string {
	if s == "" {
		return ""
	}
	visible := lipgloss.Width(s)
	pad := (w - visible) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + s
}

func (m Model) renderSidebar() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(sBold.Render("  tarkin") + "\n")
	b.WriteString(sMuted.Render("  "+strings.Repeat("─", sidebarW-2)) + "\n\n")

	type navItem struct {
		v     view
		label string
	}
	nav := []navItem{
		{viewBoard, "board"},
		{viewIdeas, "ideas"},
		{viewLog, "log"},
		{viewTrash, "trash"},
	}
	for i, item := range nav {
		active := item.v == m.currentView ||
			(m.currentView == viewTaskDetail && item.v == viewBoard) ||
			(m.currentView == viewIdeaDetail && item.v == viewIdeas)
		num := sDim.Render(fmt.Sprintf("%d", i+1))
		label := item.label
		if item.v == viewTrash && len(m.data.trashedTasks) > 0 {
			label = fmt.Sprintf("trash (%d)", len(m.data.trashedTasks))
		}
		if active {
			b.WriteString("  " + sPurple.Render("▶") + " " + num + " " + sBold.Render(label) + "\n")
		} else {
			b.WriteString("  " + sMuted.Render("·") + " " + num + " " + sMuted.Render(label) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(sMuted.Render("  "+strings.Repeat("─", sidebarW-2)) + "\n")
	b.WriteString(sDim.Render("  ?  help") + "\n")
	b.WriteString(sDim.Render("  q  quit") + "\n")

	return b.String()
}

func (m Model) renderContent() string {
	switch m.currentView {
	case viewBoard:
		return m.renderBoard()
	case viewIdeas:
		return m.renderIdeas()
	case viewLog:
		return m.renderLog()
	case viewTrash:
		return m.renderTrash()
	case viewTaskDetail:
		return m.renderTaskDetail()
	case viewIdeaDetail:
		return m.renderIdeaDetail()
	}
	return ""
}

// ── Help overlay ──────────────────────────────────────────────────────────────

func (m Model) helpLines() []string {
	global := []string{
		"",
		sDim.Render("global"),
		"  " + sNormal.Render("1 2 3 4") + "     " + sMuted.Render("switch views"),
		"  " + sNormal.Render("r") + "           " + sMuted.Render("refresh"),
		"  " + sNormal.Render("?") + "           " + sMuted.Render("toggle help"),
		"  " + sNormal.Render("q") + "           " + sMuted.Render("quit (confirm with ↵)"),
	}

	nav := []string{
		sDim.Render("navigate"),
		"  " + sNormal.Render("j / k") + "       " + sMuted.Render("move down / up"),
		"  " + sNormal.Render("↵") + "           " + sMuted.Render("open / confirm"),
		"  " + sNormal.Render("esc") + "         " + sMuted.Render("back / cancel"),
	}

	switch m.currentView {
	case viewBoard:
		return append([]string{
			sBold.Render("board"),
			"",
		}, append(nav, append([]string{
			"",
			sDim.Render("actions"),
			"  " + sNormal.Render("h / l") + "       " + sMuted.Render("move between columns"),
			"  " + sNormal.Render("a") + "           " + sMuted.Render("add task"),
			"  " + sNormal.Render("s / d / b / u") + " " + sMuted.Render("start · done · block · backlog"),
			"  " + sNormal.Render("x") + "           " + sMuted.Render("move to trash"),
		}, append(global, sDim.Render("any key to close"))...)...)...)

	case viewTaskDetail:
		return append([]string{
			sBold.Render("task detail"),
			"",
		}, append(nav, append([]string{
			"",
			sDim.Render("fields"),
			"  " + sNormal.Render("↵ on title") + "  " + sMuted.Render("edit title"),
			"  " + sNormal.Render("↵ on status") + " " + sMuted.Render("cycle status (h/l also works)"),
			"  " + sNormal.Render("↵ on priority") + " " + sMuted.Render("cycle priority (h/l also works)"),
			"  " + sNormal.Render("↵ on desc") + "   " + sMuted.Render("edit description"),
			"  " + sNormal.Render("↵ on + comment") + " " + sMuted.Render("add comment"),
			"  " + sNormal.Render("x on comment") + " " + sMuted.Render("delete comment"),
		}, append(global, sDim.Render("any key to close"))...)...)...)

	case viewIdeas, viewIdeaDetail:
		return append([]string{
			sBold.Render("ideas"),
			"",
		}, append(nav, append([]string{
			"",
			sDim.Render("actions"),
			"  " + sNormal.Render("a") + "           " + sMuted.Render("add idea"),
			"  " + sNormal.Render("p") + "           " + sMuted.Render("promote to task"),
			"  " + sNormal.Render("x") + "           " + sMuted.Render("delete idea"),
			"  " + sNormal.Render("↵ on title") + "  " + sMuted.Render("edit title"),
			"  " + sNormal.Render("↵ on desc") + "   " + sMuted.Render("edit description"),
		}, append(global, sDim.Render("any key to close"))...)...)...)

	case viewTrash:
		return []string{
			sBold.Render("trash"),
			"",
			sDim.Render("navigate"),
			"  " + sNormal.Render("j / k") + "       " + sMuted.Render("move down / up"),
			"",
			sDim.Render("actions"),
			"  " + sNormal.Render("r") + "           " + sMuted.Render("restore task to board"),
			"  " + sNormal.Render("X") + "           " + sMuted.Render("delete forever (no undo)"),
			"",
			sDim.Render("global"),
			"  " + sNormal.Render("1 2 3 4") + "     " + sMuted.Render("switch views"),
			"  " + sNormal.Render("q") + "           " + sMuted.Render("quit"),
			"",
			sDim.Render("any key to close"),
		}

	default: // viewLog
		return []string{
			sBold.Render("activity log"),
			"",
			sDim.Render("navigate"),
			"  " + sNormal.Render("j / k") + "       " + sMuted.Render("move down / up"),
			"",
			sDim.Render("global"),
			"  " + sNormal.Render("r") + "           " + sMuted.Render("refresh"),
			"  " + sNormal.Render("1 2 3 4") + "     " + sMuted.Render("switch views"),
			"  " + sNormal.Render("q") + "           " + sMuted.Render("quit"),
			"",
			sDim.Render("any key to close"),
		}
	}
}

func (m Model) renderHelpOverlay(behind string) string {
	help := overlayStyle.Render(strings.Join(m.helpLines(), "\n"))

	// center it over the main view
	helpW := lipgloss.Width(help)
	helpH := lipgloss.Height(help)
	left := (m.width - helpW) / 2
	top := (m.height - helpH) / 2
	if left < 0 {
		left = 0
	}
	if top < 0 {
		top = 0
	}

	lines := strings.Split(behind, "\n")
	helpLines := strings.Split(help, "\n")
	for i, hl := range helpLines {
		row := top + i
		if row >= len(lines) {
			break
		}
		line := lines[row]
		runes := []rune(stripAnsi(line))
		prefix := ""
		if left <= len(runes) {
			prefix = string(runes[:left])
		} else {
			prefix = line + strings.Repeat(" ", left-len(runes))
		}
		lines[row] = prefix + hl
	}
	return strings.Join(lines, "\n")
}

// stripAnsi removes ANSI escape codes for width calculation
func stripAnsi(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
		} else if inEsc && r == 'm' {
			inEsc = false
		} else if !inEsc {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// ── Board ─────────────────────────────────────────────────────────────────────

func (m Model) renderBoard() string {
	contentW := m.width - sidebarW - 3
	colW := contentW / 4

	// available height for cards: total - logFooter - leading \n - 3 header lines
	const cardH = 7 // 6 lines per card + 1 \n separator
	const colHeaderH = 3
	availH := m.height - logFooterH - 1 - colHeaderH
	visible := availH / cardH
	if visible < 1 {
		visible = 1
	}

	// scroll offset: keep selected card in view
	scroll := 0
	if m.boardRow >= visible {
		scroll = m.boardRow - visible + 1
	}

	cols := make([]string, len(columns))
	for i, col := range columns {
		cols[i] = m.renderColumn(i, col, m.colTasks(i), colW, visible, scroll)
	}
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m Model) renderColumn(idx int, col column, tasks []models.Task, width, visible, scroll int) string {
	active := idx == m.boardCol
	hdr := col.color
	if active {
		hdr = sBold
	}
	div := sMuted
	if active {
		div = sPurple
	}

	var rows []string
	rows = append(rows, hdr.Render(col.label)+" "+sDim.Render(fmt.Sprintf("(%d)", len(tasks))))
	rows = append(rows, div.Render(strings.Repeat("─", width-2)))
	rows = append(rows, "")

	if len(tasks) == 0 {
		rows = append(rows, sMuted.Render("  empty"))
	}

	// only apply scroll offset on the active column
	colScroll := 0
	if active {
		colScroll = scroll
	}

	cardW := width - 3
	end := colScroll + visible
	if end > len(tasks) {
		end = len(tasks)
	}
	for i := colScroll; i < end; i++ {
		rows = append(rows, m.renderTaskCard(tasks[i], active && i == m.boardRow, cardW))
	}

	// scroll indicators
	if colScroll > 0 {
		rows = append([]string{sMuted.Render(fmt.Sprintf("  ↑ %d above", colScroll))}, rows...)
	}
	below := len(tasks) - end
	if below > 0 {
		rows = append(rows, sMuted.Render(fmt.Sprintf("  ↓ %d below", below)))
	}

	return lipgloss.NewStyle().Width(width).PaddingLeft(1).Render(strings.Join(rows, "\n"))
}

func (m Model) renderTaskCard(t models.Task, selected bool, width int) string {
	innerW := width - 4 // border (2) + padding (2)
	if innerW < 4 {
		innerW = 4
	}

	// line 1: id + title
	idStr := fmt.Sprintf("#%d", t.ID)
	titleAvail := innerW - len(idStr) - 1
	titleText := truncate(t.Title, titleAvail)
	var line1 string
	if t.Status == models.StatusDone {
		line1 = sDim.Render(idStr) + " " + sDone.Render(titleText)
	} else {
		line1 = sDim.Render(idStr) + " " + sNormal.Render(titleText)
	}

	// line 2: priority
	line2 := priorityLabel(t.Priority)

	// lines 3-4: description word-wrapped to 2 lines, each styled individually
	// Do NOT use lipgloss Width() on the card — it re-wraps styled strings and strips ANSI.
	// Instead, pad each line to innerW manually so the border sizes correctly.
	var line3, line4 string
	if t.Description == "" {
		line3 = sMuted.Render("—")
	} else {
		wrapped := wrapLines(t.Description, innerW, 2)
		line3 = sMuted.Render(wrapped[0])
		if len(wrapped) > 1 {
			line4 = sMuted.Render(wrapped[1])
		}
	}

	borderColor := lipgloss.Color("#2a2a28")
	if selected {
		borderColor = lipgloss.Color("#7F77DD")
	}

	// pad every line to innerW so the border is uniform width
	pad := func(s string) string { return padTo(s, innerW) }
	content := pad(line1) + "\n" + pad(line2) + "\n" + pad(line3) + "\n" + pad(line4)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(content)
}

// ── Task detail ───────────────────────────────────────────────────────────────

func (m Model) renderTaskDetail() string {
	t := m.detailTask
	if t == nil {
		return ""
	}
	for i := range m.data.tasks {
		if m.data.tasks[i].ID == t.ID {
			t = &m.data.tasks[i]
			break
		}
	}

	cur := func(field int) string {
		if m.detailCursor == field {
			return sPurple.Render("▶ ")
		}
		return "  "
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(cur(fieldTitle) + sBold.Render(t.Title) + "\n")
	b.WriteString(sMuted.Render(strings.Repeat("─", 40)) + "\n\n")

	b.WriteString(cur(fieldStatus) + sDim.Render("status    ") + statusLabel(t.Status) + sMuted.Render("  ← →") + "\n")
	b.WriteString(cur(fieldPriority) + sDim.Render("priority  ") + priorityLabel(t.Priority) + sMuted.Render("  ← →") + "\n")
	b.WriteString("  " + sDim.Render("id        ") + sMuted.Render(fmt.Sprintf("#%d", t.ID)) + "\n\n")

	b.WriteString("  " + sBold.Render("description") + "\n")
	b.WriteString(sMuted.Render("  "+strings.Repeat("─", 20)) + "\n")
	if t.Description == "" {
		b.WriteString(cur(fieldDescription) + sMuted.Render("(empty)") + "\n")
	} else {
		b.WriteString(cur(fieldDescription) + sNormal.Render(t.Description) + "\n")
	}

	b.WriteString("\n")
	b.WriteString("  " + sBold.Render(fmt.Sprintf("comments (%d)", len(m.comments))) + "\n")
	b.WriteString(sMuted.Render("  " + strings.Repeat("─", 20)) + "\n")
	for i, c := range m.comments {
		cc := "  "
		if m.detailCursor == fieldCommentsBase+i {
			cc = sPurple.Render(" ▶")
		}
		b.WriteString(fmt.Sprintf("%s %s  %s\n", cc, sDim.Render(humanTime(c.CreatedAt)), sNormal.Render(c.Content)))
	}
	addLabel := sMuted.Render("+ add comment")
	if m.detailCursor == fieldAddComment {
		addLabel = sPurple.Render("▶ ") + sNormal.Render("+ add comment")
	}
	b.WriteString("  " + addLabel + "\n")

	return b.String()
}

// ── Idea detail ───────────────────────────────────────────────────────────────

func (m Model) renderIdeaDetail() string {
	idea := m.detailIdea
	if idea == nil {
		return ""
	}
	for i := range m.data.ideas {
		if m.data.ideas[i].ID == idea.ID {
			idea = &m.data.ideas[i]
			break
		}
	}

	cur := func(field int) string {
		if m.detailCursor == field {
			return sPurple.Render("▶ ")
		}
		return "  "
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(cur(fieldTitle) + sBold.Render(idea.Title) + "\n")
	b.WriteString(sMuted.Render(strings.Repeat("─", 40)) + "\n\n")
	b.WriteString("  " + sDim.Render(fmt.Sprintf("id  #%d", idea.ID)) + "\n\n")

	b.WriteString("  " + sBold.Render("description") + "\n")
	b.WriteString(sMuted.Render("  "+strings.Repeat("─", 20)) + "\n")
	if idea.Notes == "" {
		b.WriteString(cur(fieldDescription) + sMuted.Render("(empty)") + "\n")
	} else {
		b.WriteString(cur(fieldDescription) + sNormal.Render(idea.Notes) + "\n")
	}

	return b.String()
}

// ── Ideas list ────────────────────────────────────────────────────────────────

func (m Model) renderIdeas() string {
	var b strings.Builder
	b.WriteString("\n")
	up := unpromotedIdeas(m.data.ideas)
	b.WriteString(section("ideas", fmt.Sprintf("%d captured", len(up))))
	if len(up) == 0 {
		b.WriteString(sMuted.Render("  no ideas — press a to capture one") + "\n")
	}
	for i, idea := range up {
		cursor := "  "
		if i == m.ideaCursor {
			cursor = sPurple.Render(" ▶")
		}
		b.WriteString(fmt.Sprintf("%s %s  %s\n", cursor, sPurple.Render(fmt.Sprintf("#%d", idea.ID)), sNormal.Render(idea.Title)))
		if idea.Notes != "" {
			b.WriteString(fmt.Sprintf("     %s\n", sMuted.Render(idea.Notes)))
		}
	}
	return b.String()
}

// ── Log ───────────────────────────────────────────────────────────────────────

func (m Model) renderLog() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(section("activity log", fmt.Sprintf("%d entries", len(m.data.activity))))
	if len(m.data.activity) == 0 {
		b.WriteString(sMuted.Render("  no activity yet") + "\n")
	}
	for i, e := range m.data.activity {
		cursor := "  "
		if i == m.logCursor {
			cursor = sPurple.Render(" ▶")
		}
		b.WriteString(fmt.Sprintf("%s %s  %s\n", cursor, sDim.Render(fmt.Sprintf("%-12s", humanTime(e.CreatedAt))), sMuted.Render(e.Message)))
	}
	return b.String()
}

// ── Trash ─────────────────────────────────────────────────────────────────────

func (m Model) renderTrash() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(section("trash", fmt.Sprintf("%d items", len(m.data.trashedTasks))))
	if len(m.data.trashedTasks) == 0 {
		b.WriteString(sMuted.Render("  trash is empty") + "\n")
		return b.String()
	}
	for i, t := range m.data.trashedTasks {
		cursor := "  "
		if i == m.trashCursor {
			cursor = sPurple.Render(" ▶")
		}
		b.WriteString(fmt.Sprintf("%s %s  %s\n",
			cursor,
			sDim.Render(fmt.Sprintf("#%-4d", t.ID)),
			sDone.Render(t.Title),
		))
		b.WriteString(fmt.Sprintf("       %s  %s\n",
			statusLabel(t.Status),
			priorityLabel(t.Priority),
		))
	}
	return b.String()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// ── Bottom bar ────────────────────────────────────────────────────────────────

func hint(key, desc string) string {
	k := lipgloss.NewStyle().Foreground(lipgloss.Color("#e8e6df")).Render(key)
	d := lipgloss.NewStyle().Foreground(lipgloss.Color("#5f5e5a")).Render(desc)
	return k + " " + d
}

func sep() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#2a2a28")).Render("  ·  ")
}

func (m Model) contextHints() string {
	h := func(key, desc string) string { return hint(key, desc) }
	s := sep()

	switch m.currentView {
	case viewBoard:
		if m.selectedTask() != nil {
			return strings.Join([]string{
				h("↵", "open"), h("s", "start"), h("d", "done"), h("b", "block"), h("x", "delete"), h("a", "new"),
			}, s)
		}
		return h("a", "new task")

	case viewTaskDetail:
		switch m.detailCursor {
		case fieldTitle:
			return strings.Join([]string{h("↵", "edit title"), h("esc", "back")}, s)
		case fieldStatus:
			return strings.Join([]string{h("↵", "cycle"), h("←/→", "cycle"), h("esc", "back")}, s)
		case fieldPriority:
			return strings.Join([]string{h("↵", "cycle"), h("←/→", "cycle"), h("esc", "back")}, s)
		case fieldDescription:
			return strings.Join([]string{h("↵", "edit description"), h("esc", "back")}, s)
		case fieldAddComment:
			return strings.Join([]string{h("↵", "add comment"), h("esc", "back")}, s)
		default:
			if m.detailCursor >= fieldCommentsBase {
				return strings.Join([]string{h("x", "delete comment"), h("esc", "back")}, s)
			}
		}
		return h("esc", "back")

	case viewIdeas:
		if len(unpromotedIdeas(m.data.ideas)) > 0 {
			return strings.Join([]string{h("↵", "open"), h("p", "→ task"), h("x", "delete"), h("a", "new")}, s)
		}
		return h("a", "new idea")

	case viewIdeaDetail:
		switch m.detailCursor {
		case fieldTitle:
			return strings.Join([]string{h("↵", "edit title"), h("x", "delete"), h("esc", "back")}, s)
		case fieldDescription:
			return strings.Join([]string{h("↵", "edit description"), h("x", "delete"), h("esc", "back")}, s)
		}
		return strings.Join([]string{h("x", "delete"), h("esc", "back")}, s)

	case viewLog:
		return h("r", "refresh")

	case viewTrash:
		if len(m.data.trashedTasks) > 0 {
			return strings.Join([]string{h("r", "restore"), h("X", "delete forever")}, s)
		}
	}

	return ""
}

func section(title, sub string) string {
	t := sBold.Render(strings.ToUpper(title))
	s := ""
	if sub != "" {
		s = "  " + sDim.Render(sub)
	}
	return t + s + "\n" + sMuted.Render(strings.Repeat("─", 40)) + "\n"
}

func statusLabel(s models.Status) string {
	switch s {
	case models.StatusInProgress:
		return sBlue.Render("in progress")
	case models.StatusDone:
		return sGreen.Render("done")
	case models.StatusBlocked:
		return sRed.Render("blocked")
	default:
		return sMuted.Render("backlog")
	}
}

func priorityLabel(p models.Priority) string {
	switch p {
	case models.PriorityHigh:
		return sRed.Render("high")
	case models.PriorityMedium:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#C8A400")).Render("medium")
	default:
		return sBlue.Render("low")
	}
}

func nextStatus(s models.Status) models.Status {
	switch s {
	case models.StatusBacklog:
		return models.StatusInProgress
	case models.StatusInProgress:
		return models.StatusBlocked
	case models.StatusBlocked:
		return models.StatusDone
	default:
		return models.StatusBacklog
	}
}

func prevStatus(s models.Status) models.Status {
	switch s {
	case models.StatusInProgress:
		return models.StatusBacklog
	case models.StatusBlocked:
		return models.StatusInProgress
	case models.StatusDone:
		return models.StatusBlocked
	default:
		return models.StatusDone
	}
}

func nextPriority(p models.Priority) models.Priority {
	switch p {
	case models.PriorityLow:
		return models.PriorityMedium
	case models.PriorityMedium:
		return models.PriorityHigh
	default:
		return models.PriorityLow
	}
}

func prevPriority(p models.Priority) models.Priority {
	switch p {
	case models.PriorityHigh:
		return models.PriorityMedium
	case models.PriorityMedium:
		return models.PriorityLow
	default:
		return models.PriorityHigh
	}
}

func unpromotedIdeas(ideas []models.Idea) []models.Idea {
	var out []models.Idea
	for _, i := range ideas {
		if !i.Promoted {
			out = append(out, i)
		}
	}
	return out
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

// wrapLines word-wraps s into at most maxLines lines of width chars each.
// Each returned line is safe to style independently (no lipgloss re-wrapping issues).
func wrapLines(s string, width, maxLines int) []string {
	var lines []string
	words := strings.Fields(s)
	line := ""
	for _, w := range words {
		if len(lines) == maxLines {
			break
		}
		candidate := w
		if line != "" {
			candidate = line + " " + w
		}
		if len([]rune(candidate)) <= width {
			line = candidate
		} else {
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			if len(lines) < maxLines {
				// word itself wider than width — hard truncate
				line = truncate(w, width)
			}
		}
	}
	if line != "" && len(lines) < maxLines {
		lines = append(lines, line)
	}
	// truncate last line if it somehow exceeds width
	if len(lines) > 0 {
		last := len(lines) - 1
		lines[last] = truncate(lines[last], width)
	}
	return lines
}

func padTo(s string, w int) string {
	visible := lipgloss.Width(s)
	if visible >= w {
		return s
	}
	return s + strings.Repeat(" ", w-visible)
}

func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return string(runes[:maxW])
	}
	return string(runes[:maxW-1]) + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
