package gotesttask

import (
	"archivezipper/handlers"
	"archivezipper/task"
)

func main() {	
	var taskManager = task.NewTaskManager(3)
	
	//NOTE Dependency injection
	handlers.Init(taskManager)

}
