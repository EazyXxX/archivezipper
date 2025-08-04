package task

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
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
		logrus.WithField("task_id", id).Warn("max active tasks reached")
		return errors.New("server busy: max active tasks reached")
	}

	m.tasks[id] = &Task{
		id:     id,
		status: StatusInProgress,
		files:  []FileResult{},
	}
	m.active++

	logrus.WithFields(logrus.Fields{
		"task_id":    id,
		"active_now": m.active,
	}).Info("task created")

	return nil
}

func (m *TaskManager) GetTask(id string) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		logrus.WithField("task_id", id).Warn("task not found")
		return nil, errors.New("task not found")
	}
	return task, nil
}

func (m *TaskManager) AddFileToTask(taskID, url string) error {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	m.mu.Unlock()

	if !ok {
		logrus.WithField("task_id", taskID).Warn("task not found")
		return errors.New("task not found")
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	//Max file count check
	if len(task.files) >= 3 {
		logrus.WithField("task_id", taskID).Warn("max files per task reached")
		return errors.New("max files per task reached")
	}

	//File extension check
	if !isAllowedExtension(url) {
		logrus.WithFields(logrus.Fields{
			"task_id": taskID,
			"url":     url,
		}).Warn("unsupported file type")
		return errors.New("unsupported file type")
	}

	//Adding a file
	task.files = append(task.files, FileResult{
		URL:     url,
		Success: false,
	})

	logrus.WithFields(logrus.Fields{
		"task_id": taskID,
		"url":     url,
		"count":   len(task.files),
	}).Info("file added to task")

	//Archivation start upon reaching 3 files
	if len(task.files) == 3 {
		logrus.WithField("task_id", taskID).Info("starting archiving task")
		go m.processTaskArchive(task)
	}

	return nil
}

func (m *TaskManager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	logrus.Info("waiting for active tasks to finish")

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
		logrus.Info("all tasks completed successfully")
	case <-ctx.Done():
		logrus.Warn("shutdown timeout, terminating with active tasks")
	}

	//State cleanse
	m.tasks = make(map[string]*Task)
	m.active = 0

	logrus.Info("task manager shutdown complete")
}
