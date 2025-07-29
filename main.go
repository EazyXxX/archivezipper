package gotesttask

import "archivezipper/task"

var taskManager = &task.TaskManager{
	Tasks: make(map[string]*task.Task),
	MaxActive: 3,
}
