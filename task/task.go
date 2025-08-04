package task

import (
	"archive/zip"
	"errors"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type TaskStatus string

const (
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusError      TaskStatus = "error"
)

type FileResult struct {
	URL     string `json:"url"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type Task struct {
	id         string
	status     TaskStatus
	files      []FileResult
	archiveURL string
	mu         sync.Mutex
}

type TaskSnapshot struct {
	ID         string       `json:"id"`
	Status     TaskStatus   `json:"status"`
	Files      []FileResult `json:"files"`
	ArchiveURL string       `json:"archive_url,omitempty"`
}

// GetSnapshot returns an immutable snapshot of the task status
func (t *Task) GetSnapshot() TaskSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()

	filesCopy := make([]FileResult, len(t.files))
	copy(filesCopy, t.files)

	return TaskSnapshot{
		ID:         t.id,
		Status:     t.status,
		Files:      filesCopy,
		ArchiveURL: t.archiveURL,
	}
}

func (m *TaskManager) processTaskArchive(t *Task) {
	logger := logrus.WithField("task_id", t.id)

	tempDir, err := os.MkdirTemp("", "task-"+t.id)
	if err != nil {
		logger.WithError(err).Error("failed to create temp dir")
		m.updateTaskStatus(t, StatusError)
		return
	}
	logger.WithField("temp_dir", tempDir).Info("temp directory created")

	var wg sync.WaitGroup
	errorCh := make(chan error, len(t.files))
	fileResults := make([]FileResult, len(t.files))

	for i, fileRes := range t.files {
		wg.Add(1)
		go func(index int, rawURL string) {
			defer wg.Done()

			//Parsing URL
			u, err := urlpkg.Parse(rawURL)
			if err != nil {
				logger.WithField("file_url", rawURL).WithError(err).Warn("invalid URL format")
				errorCh <- err
				fileResults[index] = FileResult{URL: rawURL, Success: false, Error: "invalid URL"}
				return
			}

			//Extracting and decoding filename
			name := filepath.Base(u.Path)
			decoded, err := urlpkg.QueryUnescape(name)
			if err != nil {
				logger.WithField("file_url", rawURL).WithError(err).Warn("filename decode failed")
				errorCh <- err
				fileResults[index] = FileResult{URL: rawURL, Success: false, Error: "decode error"}
				return
			}

			localPath := filepath.Join(tempDir, decoded)

			//Use rawURL instead of undefined "url"
			if err := downloadFileWithRetry(localPath, rawURL, 3); err != nil {
				logger.WithError(err).Warn("file download failed")
				errorCh <- err
				fileResults[index] = FileResult{
					URL:     rawURL,
					Success: false,
					Error:   err.Error(),
				}
				return
			}

			logger.Info("file downloaded successfully")
			fileResults[index] = FileResult{
				URL:     rawURL,
				Success: true,
			}
		}(i, fileRes.URL)
	}

	wg.Wait()
	close(errorCh)

	t.mu.Lock()
	t.files = fileResults
	t.mu.Unlock()

	if len(errorCh) > 0 {
		logger.Warn("some files failed to download")
		m.updateTaskStatus(t, StatusError)
		os.RemoveAll(tempDir)
		return
	}

	archivesDir := "./archives"
	if err := os.MkdirAll(archivesDir, 0755); err != nil {
		logger.WithError(err).Error("failed to create archive directory")
		m.updateTaskStatus(t, StatusError)
		os.RemoveAll(tempDir)
		return
	}

	archivePath := filepath.Join(archivesDir, t.id+".zip")
	if err := createZipArchive(archivePath, tempDir); err != nil {
		logger.WithError(err).Error("failed to create zip archive")
		m.updateTaskStatus(t, StatusError)
		os.RemoveAll(tempDir)
		return
	}

	logger.WithField("archive_path", archivePath).Info("archive created successfully")

	t.mu.Lock()
	t.archiveURL = archivePath
	t.status = StatusDone
	t.mu.Unlock()

	os.RemoveAll(tempDir)
	logger.Info("temp directory cleaned up")

	m.mu.Lock()
	m.active--
	m.mu.Unlock()
	logger.Info("task completed and resources released")
}

func (m *TaskManager) updateTaskStatus(t *Task, status TaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = status
}

// isAllowedExtension returns true if the file extension is allowed
func isAllowedExtension(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasSuffix(lower, ".pdf") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".jpg")
}

func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to download file: " + resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func downloadFileWithRetry(filepath string, url string, maxRetries int) error {
	var lastError error

	for i := 0; i < maxRetries; i++ {
		if err := downloadFile(filepath, url); err == nil {
			return nil
		} else {
			lastError = err
			time.Sleep(time.Duration(i*i) * time.Second)
		}
	}

	return lastError
}

func createZipArchive(zipPath string, dir string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		wr, err := archive.Create(relPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(wr, file)
		return err
	})
}
