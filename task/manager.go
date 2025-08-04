package task

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

type TaskManager struct {
	tasks     map[string]*Task
	active    int
	maxActive int
	mu        sync.Mutex
}

func NewTaskManager(max int) *TaskManager {
	return &TaskManager{
		tasks:     make(map[string]*Task),
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
		id:     id,
		status: StatusInProgress,
		files:  []FileResult{},
	}
	m.active++
	return nil
}

func (m *TaskManager) GetTask(id string) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	task, ok := m.tasks[id]
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

	task.mu.Lock()
	defer task.mu.Unlock()

	//Max file count check
	if len(task.files) >= 3 {
		return errors.New("max files per task reached")
	}

	//File extension check
	if !isAllowedExtension(url) {
		return errors.New("unsupported file type")
	}

	//Adding a file
	task.files = append(task.files, FileResult{
		URL:     url,
		Success: false,
	})

	//Archivation start upon reaching 3 files
	if len(task.files) == 3 {
		go m.processTaskArchive(task)
	}

	return nil
}

func (m *TaskManager) Shutdown(ctx context.Context) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    //Making a copy for checking tasks endings
    done := make(chan struct{})
    go func() {
        for m.active > 0 {
            time.Sleep(100 * time.Millisecond)
        }
        close(done)
    }()
    
    //Waiting for timeout
    select {
    case <-done:
        log.Println("All tasks completed successfully")
    case <-ctx.Done():
        log.Println("Shutdown timeout, terminating with active tasks")
    }
    
    //State cleanse
    m.tasks = make(map[string]*Task)
    m.active = 0
}
