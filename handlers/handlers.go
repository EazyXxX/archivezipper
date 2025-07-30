package handlers

import (
	"encoding/json"
	"net/http"

	"archivezipper/task"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var taskManager *task.TaskManager

func Init(manager *task.TaskManager) {
	taskManager = manager
}

// POST /tasks
func CreateTaskHandler(w http.ResponseWriter, r *http.Request) {
	id := uuid.New().String()

	err := taskManager.CreateTask(id)
	if err != nil {
		http.Error(w, "Server busy: max active tasks reached", http.StatusTooManyRequests)
		return 
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"task_id": id})
}

type AddRequest struct {
	URL string `json:"url"`
}

func AddFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req AddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := taskManager.AddFileToTask(taskID, req.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

//GET /tasks/{id}
func GetTaskHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	t, err := taskManager.GetTask(taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	t.Mu.Lock()
	defer t.Mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}
