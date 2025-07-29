package task

import "sync"

type TaskStatus string

const (
	StatusInProgress TaskStatus = "in_progress"
	StatusDone 						TaskStatus = "done"
	StatusError						TaskStatus = "error"
)

type FileResult struct {
	URL string `json:"url"`
	Success bool `json:"success"`
	Error string `json:"error,omitempty"`
}

type Task struct {
	ID string
	Status TaskStatus
	Files []FileResult
	ArchiveURL string
	mu sync.Mutex
}
