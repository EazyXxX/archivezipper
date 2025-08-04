package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"archivezipper/task"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var taskManager *task.TaskManager

func Init(m *task.TaskManager) {
    taskManager = m
}

type Response struct {
    Data  interface{} `json:"data,omitempty"`
    Error string      `json:"error,omitempty"`
}

type TaskResponse struct {
    TaskID string `json:"task_id"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(Response{Data: data})
}

func writeError(w http.ResponseWriter, status int, err error) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(Response{Error: err.Error()})
}

//POST /tasks
func CreateTaskHandler(w http.ResponseWriter, r *http.Request) {
    id := uuid.New().String()
    if err := taskManager.CreateTask(id); err != nil {
        writeError(w, http.StatusTooManyRequests, err)
        return
    }
    writeJSON(w, http.StatusCreated, TaskResponse{TaskID: id})
}

//POST /tasks/{id}/files
func AddFileHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    var req struct{ URL string `json:"url"` }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request"))
        return
    }
    if err := taskManager.AddFileToTask(id, req.URL); err != nil {
        writeError(w, http.StatusBadRequest, err)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"message": "file added"})
}

//GET /tasks/{id}
func GetTaskHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    t, err := taskManager.GetTask(id)
    if err != nil {
        writeError(w, http.StatusNotFound, err)
        return
    }
    snap := t.GetSnapshot()
    if snap.Status == task.StatusDone && snap.ArchiveURL != "" {
        scheme := "http"
        if r.TLS != nil {
            scheme = "https"
        }
        url := fmt.Sprintf("%s://%s/tasks/%s/archive", scheme, r.Host, id)
        type out struct {
            ID         string             `json:"id"`
            Status     task.TaskStatus    `json:"status"`
            Files      []task.FileResult  `json:"files"`
            ArchiveURL string             `json:"archive_url"`
        }
        writeJSON(w, http.StatusOK, out{snap.ID, snap.Status, snap.Files, url})
        return
    }
    writeJSON(w, http.StatusOK, snap)
}

//GET /tasks/{id}/archive
func DownloadArchiveHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    t, err := taskManager.GetTask(id)
    if err != nil {
        writeError(w, http.StatusNotFound, err)
        return
    }

	// Using GetSnapshot for safe access
		snap := t.GetSnapshot()
    if snap.Status != task.StatusDone || snap.ArchiveURL == "" {
        writeError(w, http.StatusBadRequest, fmt.Errorf("archive not ready"))
        return
    }
    w.Header().Set("Content-Type", "application/zip")
    w.Header().Set("Content-Disposition", "attachment; filename=archive.zip")
    http.ServeFile(w, r, snap.ArchiveURL)
}