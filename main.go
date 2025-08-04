package main

import (
	"archivezipper/config"
	"archivezipper/handlers"
	"archivezipper/task"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/gorilla/mux"
)

func loadConfig(path string) (*config.Config, error) {
	var cfg config.Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func main() {
	//Config init
	cfg, err := loadConfig("config.toml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	var taskManager = task.NewTaskManager(cfg.Task.MaxConcurrent)
	//Dependency injection
	handlers.Init(taskManager)

	//Router initialization
	r := mux.NewRouter()
	r.HandleFunc("/tasks", handlers.CreateTaskHandler).Methods("POST")
	r.HandleFunc("/tasks/{id}", handlers.GetTaskHandler).Methods("GET")
	r.HandleFunc("/tasks/{id}/files", handlers.AddFileHandler).Methods("POST")

	//Private endpoint for archive loading (used only for task status URL)
  r.HandleFunc("/tasks/{id}/archive", handlers.DownloadArchiveHandler).Methods("GET")

//HTTP server initialization
srv := &http.Server{
	Addr: ":" + cfg.Server.Port,
	Handler: r,
}

//Graceful shutdown chanell
done := make(chan os.Signal, 1)
signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

go func() {
	log.Printf("Server started on port %s 🚀 (max tasks: %d)", cfg.Server.Port, cfg.Task.MaxConcurrent)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error %v", err)
	}
}()

//Waiting for the ending signal
<-done
log.Println("Server is shutting down")

//Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Delay)
defer cancel()

if err := srv.Shutdown(ctx); err != nil {
	log.Printf("Server shutdown error: %v", err)
}

taskManager.Shutdown(ctx)

log.Println("Server stopped")
}
