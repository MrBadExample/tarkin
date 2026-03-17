package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yourusername/tarkin/internal/models"
)

var DB *sql.DB

// DBPath returns ~/.tarkin/tarkin.db
func DBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tarkin", "tarkin.db")
}

// Init opens (or creates) the SQLite database and runs migrations.
func Init() error {
	path := DBPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("could not create ~/.tarkin dir: %w", err)
	}

	var err error
	DB, err = sql.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("could not open db: %w", err)
	}

	return migrate()
}

func migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		title       TEXT NOT NULL,
		status      TEXT NOT NULL DEFAULT 'backlog',
		priority    TEXT NOT NULL DEFAULT 'medium',
		agent       TEXT DEFAULT '',
		notes       TEXT DEFAULT '',
		description TEXT DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ideas (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		title       TEXT NOT NULL,
		notes       TEXT DEFAULT '',
		promoted    INTEGER DEFAULT 0,
		task_id     INTEGER DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS agents (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		codename     TEXT NOT NULL UNIQUE,
		tool         TEXT NOT NULL DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'offline',
		current_task TEXT DEFAULT '',
		last_seen    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS activity (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		message    TEXT NOT NULL,
		agent      TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS task_comments (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id    INTEGER NOT NULL,
		content    TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := DB.Exec(schema); err != nil {
		return err
	}
	// add description column to existing DBs (no-op if already present)
	DB.Exec(`ALTER TABLE tasks ADD COLUMN description TEXT DEFAULT ''`)
	// add deleted_at for soft-delete / trash (no-op if already present)
	DB.Exec(`ALTER TABLE tasks ADD COLUMN deleted_at DATETIME DEFAULT NULL`)
	return nil
}

// ── Tasks ────────────────────────────────────────────────────────────────────

func CreateTask(title, priority, agent string) (models.Task, error) {
	res, err := DB.Exec(
		`INSERT INTO tasks (title, priority, agent) VALUES (?, ?, ?)`,
		title, priority, agent,
	)
	if err != nil {
		return models.Task{}, err
	}
	id, _ := res.LastInsertId()
	Log(fmt.Sprintf("task #%d created: %s", id, title), "")
	return GetTask(int(id))
}

func GetTask(id int) (models.Task, error) {
	row := DB.QueryRow(`SELECT id,title,status,priority,agent,notes,description,created_at,updated_at FROM tasks WHERE id=?`, id)
	return scanTask(row)
}

func ListTasks(status string) ([]models.Task, error) {
	query := `SELECT id,title,status,priority,agent,notes,description,created_at,updated_at FROM tasks WHERE deleted_at IS NULL`
	args := []interface{}{}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	query += ` ORDER BY
		CASE status WHEN 'in_progress' THEN 0 WHEN 'backlog' THEN 1 WHEN 'blocked' THEN 2 WHEN 'done' THEN 3 END,
		CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 END,
		created_at ASC`

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func UpdateTaskStatus(id int, status models.Status) error {
	_, err := DB.Exec(
		`UPDATE tasks SET status=?, updated_at=? WHERE id=?`,
		status, time.Now(), id,
	)
	if err == nil {
		Log(fmt.Sprintf("task #%d → %s", id, status), "")
	}
	return err
}

func AssignTask(id int, agent string) error {
	_, err := DB.Exec(
		`UPDATE tasks SET agent=?, updated_at=? WHERE id=?`,
		agent, time.Now(), id,
	)
	if err == nil {
		Log(fmt.Sprintf("task #%d assigned to %s", id, agent), agent)
	}
	return err
}

func DeleteTask(id int) error {
	_, err := DB.Exec(`UPDATE tasks SET deleted_at=? WHERE id=?`, time.Now(), id)
	if err == nil {
		Log(fmt.Sprintf("task #%d moved to trash", id), "")
	}
	return err
}

func ListTrashedTasks() ([]models.Task, error) {
	rows, err := DB.Query(`SELECT id,title,status,priority,agent,notes,description,created_at,updated_at FROM tasks WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func RestoreTask(id int) error {
	_, err := DB.Exec(`UPDATE tasks SET deleted_at=NULL, updated_at=? WHERE id=?`, time.Now(), id)
	if err == nil {
		Log(fmt.Sprintf("task #%d restored from trash", id), "")
	}
	return err
}

func PermanentDeleteTask(id int) error {
	_, err := DB.Exec(`DELETE FROM tasks WHERE id=?`, id)
	if err == nil {
		Log(fmt.Sprintf("task #%d permanently deleted", id), "")
	}
	return err
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanTask(s scanner) (models.Task, error) {
	var t models.Task
	err := s.Scan(&t.ID, &t.Title, &t.Status, &t.Priority, &t.Agent, &t.Notes, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func UpdateTaskDescription(id int, description string) error {
	_, err := DB.Exec(`UPDATE tasks SET description=?, updated_at=? WHERE id=?`, description, time.Now(), id)
	if err == nil {
		Log(fmt.Sprintf("task #%d description updated", id), "")
	}
	return err
}

func UpdateTaskTitle(id int, title string) error {
	_, err := DB.Exec(`UPDATE tasks SET title=?, updated_at=? WHERE id=?`, title, time.Now(), id)
	if err == nil {
		Log(fmt.Sprintf("task #%d title updated: %s", id, title), "")
	}
	return err
}

func UpdateTaskPriority(id int, priority models.Priority) error {
	_, err := DB.Exec(`UPDATE tasks SET priority=?, updated_at=? WHERE id=?`, priority, time.Now(), id)
	if err == nil {
		Log(fmt.Sprintf("task #%d priority → %s", id, priority), "")
	}
	return err
}

// ── Ideas ────────────────────────────────────────────────────────────────────

func CreateIdea(title, notes string) (models.Idea, error) {
	res, err := DB.Exec(`INSERT INTO ideas (title, notes) VALUES (?, ?)`, title, notes)
	if err != nil {
		return models.Idea{}, err
	}
	id, _ := res.LastInsertId()
	Log(fmt.Sprintf("idea #%d captured: %s", id, title), "")
	return GetIdea(int(id))
}

func GetIdea(id int) (models.Idea, error) {
	row := DB.QueryRow(`SELECT id,title,notes,promoted,task_id,created_at FROM ideas WHERE id=?`, id)
	var i models.Idea
	err := row.Scan(&i.ID, &i.Title, &i.Notes, &i.Promoted, &i.TaskID, &i.CreatedAt)
	return i, err
}

func ListIdeas() ([]models.Idea, error) {
	rows, err := DB.Query(`SELECT id,title,notes,promoted,task_id,created_at FROM ideas ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ideas []models.Idea
	for rows.Next() {
		var i models.Idea
		if err := rows.Scan(&i.ID, &i.Title, &i.Notes, &i.Promoted, &i.TaskID, &i.CreatedAt); err != nil {
			return nil, err
		}
		ideas = append(ideas, i)
	}
	return ideas, nil
}

func PromoteIdea(id int, priority, agent string) (models.Task, error) {
	idea, err := GetIdea(id)
	if err != nil {
		return models.Task{}, fmt.Errorf("idea #%d not found", id)
	}

	task, err := CreateTask(idea.Title, priority, agent)
	if err != nil {
		return models.Task{}, err
	}

	_, err = DB.Exec(`UPDATE ideas SET promoted=1, task_id=? WHERE id=?`, task.ID, id)
	if err != nil {
		return models.Task{}, err
	}

	Log(fmt.Sprintf("idea #%d promoted to task #%d", id, task.ID), "")
	return task, nil
}

func UpdateIdea(id int, title, notes string) error {
	_, err := DB.Exec(`UPDATE ideas SET title=?, notes=? WHERE id=?`, title, notes, id)
	if err == nil {
		Log(fmt.Sprintf("idea #%d updated: %s", id, title), "")
	}
	return err
}

func DeleteIdea(id int) error {
	_, err := DB.Exec(`DELETE FROM ideas WHERE id=?`, id)
	if err == nil {
		Log(fmt.Sprintf("idea #%d deleted", id), "")
	}
	return err
}

// ── Comments ─────────────────────────────────────────────────────────────────

func AddComment(taskID int, content string) (models.Comment, error) {
	res, err := DB.Exec(`INSERT INTO task_comments (task_id, content) VALUES (?, ?)`, taskID, content)
	if err != nil {
		return models.Comment{}, err
	}
	id, _ := res.LastInsertId()
	var c models.Comment
	row := DB.QueryRow(`SELECT id, task_id, content, created_at FROM task_comments WHERE id=?`, id)
	err = row.Scan(&c.ID, &c.TaskID, &c.Content, &c.CreatedAt)
	if err == nil {
		Log(fmt.Sprintf("task #%d comment added", taskID), "")
	}
	return c, err
}

func ListComments(taskID int) ([]models.Comment, error) {
	rows, err := DB.Query(`SELECT id, task_id, content, created_at FROM task_comments WHERE task_id=? ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []models.Comment
	for rows.Next() {
		var c models.Comment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func DeleteComment(id int) error {
	_, err := DB.Exec(`DELETE FROM task_comments WHERE id=?`, id)
	if err == nil {
		Log(fmt.Sprintf("comment #%d deleted", id), "")
	}
	return err
}

// ── Agents ───────────────────────────────────────────────────────────────────

func UpsertAgent(codename, tool string) error {
	_, err := DB.Exec(`
		INSERT INTO agents (codename, tool, status, last_seen)
		VALUES (?, ?, 'offline', CURRENT_TIMESTAMP)
		ON CONFLICT(codename) DO UPDATE SET tool=excluded.tool
	`, codename, tool)
	return err
}

func ListAgents() ([]models.Agent, error) {
	rows, err := DB.Query(`SELECT id,codename,tool,status,current_task,last_seen FROM agents ORDER BY codename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Codename, &a.Tool, &a.Status, &a.CurrentTask, &a.LastSeen); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func UpdateAgentStatus(codename, status, currentTask string) error {
	_, err := DB.Exec(
		`UPDATE agents SET status=?, current_task=?, last_seen=? WHERE codename=?`,
		status, currentTask, time.Now(), codename,
	)
	return err
}

// ── Activity log ─────────────────────────────────────────────────────────────

func Log(message, agent string) {
	DB.Exec(`INSERT INTO activity (message, agent) VALUES (?, ?)`, message, agent)
}

func ListActivity(limit int) ([]struct {
	Message   string
	Agent     string
	CreatedAt time.Time
}, error) {
	rows, err := DB.Query(
		`SELECT message, agent, created_at FROM activity ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []struct {
		Message   string
		Agent     string
		CreatedAt time.Time
	}
	for rows.Next() {
		var e struct {
			Message   string
			Agent     string
			CreatedAt time.Time
		}
		if err := rows.Scan(&e.Message, &e.Agent, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
