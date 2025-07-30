package task

import (
	"errors"
	"strings"
	"sync"
)

type TaskManager struct {
	tasks map[string]*Task
	active int
	maxActive int
	mu sync.Mutex
}

//NOTE OOP incapsulation pattern
// Exported TaskManager constructor for the other packages
func NewTaskManager(max int) *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*Task),
		maxActive: max,
	}
}

func (m *TaskManager) CreateTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active >= m.maxActive {
		return errors.New("server busy: max active tasks reached")
	}

	m.tasks[id] = &Task{
		ID: id,
		Status: StatusInProgress,
		Files: []FileResult{},
	}
	m.active++
	return nil
}

func (m *TaskManager) GetTask(id string) (*Task, error) {
 m.mu.Lock()
	task, ok := m.tasks[id]
	m.mu.Unlock()

	if !ok {
		return nil, errors.New("task not found")
	}
	return task, nil
}

func (m *TaskManager) AddFileToTask(taskID, url string) error {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	m.mu.Unlock()

	if !ok {
		return errors.New("task not found")
	}

	task.Mu.Lock()
	defer task.Mu.Unlock()

	//Less then 3 files check
	if len(task.Files) >= 3 {
		return errors.New("max files per task reached")
	}

	//Extension check
	if !(strings.HasSuffix(url, ".pdf") || strings.HasSuffix(url, ".jpeg")) {
		return errors.New("unsupported file type")
	}

	task.Files = append(task.Files, FileResult{
		URL: 				url,
		Success: false,
	})

	//Archivation by a separate goroutine
	if len(task.Files) == 3 {
		go m.processTaskArchive(task)
	}

	return nil
}
