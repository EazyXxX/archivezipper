package task

import (
	"errors"
	"sync"
)

type TaskManager struct {
	Tasks map[string]*Task
	Active int
	MaxActive int
	mu sync.Mutex
}

func (m *TaskManager) CreateTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Active >= m.MaxActive {
		return errors.New("server busy: max active tasks reached")
	}

	m.Tasks[id] = &Task{
		ID: id,
		Status: StatusInProgress,
		Files: []FileResult{},
	}
	m.Active++
	return nil
}

func (m *TaskManager) GetTask(id string) (*Task, error) {
 m.mu.Lock()
	task, ok := m.Tasks[id]
	m.mu.Unlock()

	if !ok {
		return nil, errors.New("task not found")
	}
	return task, nil
}
