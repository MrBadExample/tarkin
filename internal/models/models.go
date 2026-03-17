package models

import "time"

type Status string
type Priority string

const (
	StatusBacklog    Status = "backlog"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusBlocked    Status = "blocked"

	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

type Task struct {
	ID          int
	Title       string
	Status      Status
	Priority    Priority
	Agent       string
	Notes       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Idea struct {
	ID          int
	Title       string
	Notes       string // description
	Promoted    bool
	TaskID      int
	CreatedAt   time.Time
}

type Comment struct {
	ID        int
	TaskID    int
	Content   string
	CreatedAt time.Time
}

type Agent struct {
	ID        int
	Codename  string // "vader", "ackbar", "r2-d2"
	Tool      string // "claude-code", "openclaw"
	Status    string // "online", "offline", "busy"
	CurrentTask string
	LastSeen  time.Time
}
