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
)

type TaskStatus string

const (
	StatusInProgress TaskStatus = "in_progress"
	StatusDone 						TaskStatus = "done"
	StatusError						TaskStatus = "error"
)

type FileResult struct {
	URL string `json:"url"`
	Success bool `json:"success"`
	Error string `json:"error,omitempty"`
}

type Task struct {
	ID string
	Status TaskStatus
	Files []FileResult
	ArchiveURL string
	Mu sync.Mutex
}

func (m *TaskManager) processTaskArchive(t *Task) {
	t.Mu.Lock()
	defer t.Mu.Unlock()

	//Creating a temporary folder
	tempDir, err := os.MkdirTemp("", "task-"+t.ID)
	if err != nil {
					t.Status = StatusError
					return
	}

	//Downloading files to tempDir
	for i, fileRes := range t.Files {
					if !isAllowedExtension(fileRes.URL) {
									t.Files[i].Success = false
									t.Files[i].Error = "unsupported file extension"
									continue
					}

					filename := filepath.Base(fileRes.URL)
					localPath := filepath.Join(tempDir, filename)

					if err := downloadFile(localPath, fileRes.URL); err != nil {
									t.Files[i].Success = false
									t.Files[i].Error = "failed to download: " + err.Error()
									continue
					}
					t.Files[i].Success = true
	}

	//Preparing a folder for the final archives.
	archivesDir := "./archives"
	_ = os.MkdirAll(archivesDir, 0755)

	//Collecting zip in archivesDir
	archivePath := filepath.Join(archivesDir, t.ID+".zip")
	if err := createZipArchive(archivePath, tempDir); err != nil {
					t.Status = StatusError
					return
	}

	t.ArchiveURL = archivePath
	t.Status = StatusDone

	//Cleaning tempDir, because the archive is already in archivesDir
	os.RemoveAll(tempDir)
}


//Extension check
func isAllowedExtension(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasSuffix(lower, ".pdf") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".jpg")
}

//URL file download
func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
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

func createZipArchive(zipPath string, dir string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() || file.Name() == filepath.Base(zipPath) {
			continue
		}

		filePath := filepath.Join(dir, file.Name())
		
		f, err := os.Open(filePath)
		if err != nil{
			return err
		}

		wr, err := archive.Create(file.Name())
		if err != nil {
			f.Close()
			return err
		}

		_, err = io.Copy(wr, f)
		f.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
