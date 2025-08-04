package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"archivezipper/task"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
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

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
}

// POST /tasks
func CreateTaskHandler(w http.ResponseWriter, r *http.Request) {
	id := uuid.New().String()
	logger := logrus.WithField("task_id", id)

	if err := taskManager.CreateTask(id); err != nil {
		logger.WithError(err).Warn("failed to create task")
		writeError(w, http.StatusTooManyRequests, err)
		return
	}

	logger.Info("task created successfully")
	writeJSON(w, http.StatusCreated, TaskResponse{TaskID: id})
}

// POST /tasks/{id}/files
func AddFileHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	logger := logrus.WithField("task_id", id)

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.WithError(err).Warn("invalid request body")
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request"))
		return
	}

	logger = logger.WithField("file_url", req.URL)

	if err := taskManager.AddFileToTask(id, req.URL); err != nil {
		logger.WithError(err).Warn("failed to add file to task")
		writeError(w, http.StatusBadRequest, err)
		return
	}

	logger.Info("file added to task successfully")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "file added"}); err != nil {
		logger.WithError(err).Error("failed to write file-added response")
	}
}

// GET /tasks/{id}
func GetTaskHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	logger := logrus.WithField("task_id", id)

	t, err := taskManager.GetTask(id)
	if err != nil {
		logger.WithError(err).Warn("task not found")
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
			ID         string            `json:"id"`
			Status     task.TaskStatus   `json:"status"`
			Files      []task.FileResult `json:"files"`
			ArchiveURL string            `json:"archive_url"`
		}

		logger.Info("returning task with archive URL")
		writeJSON(w, http.StatusOK, out{snap.ID, snap.Status, snap.Files, url})
		return
	}

	logger.Info("returning task snapshot without archive")
	writeJSON(w, http.StatusOK, snap)
}

// GET /tasks/{id}/archive
func DownloadArchiveHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	logger := logrus.WithField("task_id", id)

	t, err := taskManager.GetTask(id)
	if err != nil {
		logger.WithError(err).Warn("task not found")
		writeError(w, http.StatusNotFound, err)
		return
	}

	snap := t.GetSnapshot()

	if snap.Status != task.StatusDone || snap.ArchiveURL == "" {
		logger.Warn("archive not ready")
		writeError(w, http.StatusBadRequest, fmt.Errorf("archive not ready"))
		return
	}

	logger.WithField("archive_path", snap.ArchiveURL).Info("serving archive to client")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=archive.zip")
	http.ServeFile(w, r, snap.ArchiveURL)
}
