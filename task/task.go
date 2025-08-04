package task

import (
	"archive/zip"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

//GetSnapshot returns an immutable snapshot of the task status
func (t *Task) GetSnapshot() TaskSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Creating files deep copy
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
	//Creating a temporary folder
	tempDir, err := os.MkdirTemp("", "task-"+t.id)
	if err != nil {
		m.updateTaskStatus(t, StatusError)
		return
	}

	//Parallel file downloading
	var wg sync.WaitGroup
	errorCh := make(chan error, len(t.files))
	fileResults := make([]FileResult, len(t.files))

	for i, fileRes := range t.files {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()
			
			filename := filepath.Base(url)
			localPath := filepath.Join(tempDir, filename)
			
			if err := downloadFileWithRetry(localPath, url, 3); err != nil {
				errorCh <- err
				fileResults[index] = FileResult{
					URL:     url,
					Success: false,
					Error:   err.Error(),
				}
				return
			}
			
			fileResults[index] = FileResult{
				URL:     url,
				Success: true,
			}
		}(i, fileRes.URL)
	}
	
	wg.Wait()
	close(errorCh)
	
	//Updating file results
	t.mu.Lock()
	t.files = fileResults
	t.mu.Unlock()
	
	//Checking for errors
	if len(errorCh) > 0 {
		m.updateTaskStatus(t, StatusError)
		os.RemoveAll(tempDir)
		return
	}
	
	//Creating an archive
	archivesDir := "./archives"
	_ = os.MkdirAll(archivesDir, 0755)
	
	archivePath := filepath.Join(archivesDir, t.id+".zip")
	if err := createZipArchive(archivePath, tempDir); err != nil {
		m.updateTaskStatus(t, StatusError)
		os.RemoveAll(tempDir)
		return
	}
	
	//Updating task status
	t.mu.Lock()
	t.archiveURL = archivePath
	t.status = StatusDone
	t.mu.Unlock()
	
	//Temp dir removal
	os.RemoveAll(tempDir)
	
	//Reducing the active task counter
	m.mu.Lock()
	m.active--
	m.mu.Unlock()
}

func (m *TaskManager) updateTaskStatus(t *Task, status TaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = status
}

//Extension check
func isAllowedExtension(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasSuffix(lower, ".pdf") || 
	       strings.HasSuffix(lower, ".jpeg") || 
	       strings.HasSuffix(lower, ".jpg")
}

//URL file download
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

