package main

import (
	"archivezipper/config"
	"archivezipper/handlers"
	"archivezipper/task"
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func loadConfig(path string) (*config.Config, error) {
	var cfg config.Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func main() {
	//Logger setup
	logger := logrus.New()
	logger.Out = os.Stdout
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logger.SetLevel(logrus.InfoLevel)

	//Config loading
	cfg, err := loadConfig("config.toml")
	if err != nil {
		logger.Fatalf("failed to load config: %v", err)
	}

	//Task manager and Dependency injection
	manager := task.NewTaskManager(cfg.Task.MaxConcurrent)
	handlers.Init(manager)

	//Router setup
	r := mux.NewRouter()
	r.HandleFunc("/tasks", handlers.CreateTaskHandler).Methods("POST")
	r.HandleFunc("/tasks/{id}", handlers.GetTaskHandler).Methods("GET")
	r.HandleFunc("/tasks/{id}/files", handlers.AddFileHandler).Methods("POST")
	r.HandleFunc("/tasks/{id}/archive", handlers.DownloadArchiveHandler).Methods("GET")

	//HTTP server setup
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	//Server startup in goroutine
	go func() {
		logger.WithFields(logrus.Fields{
			"port":     cfg.Server.Port,
			"maxTasks": cfg.Task.MaxConcurrent,
		}).Info("starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	//Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Delay)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("server shutdown error")
	}

	manager.Shutdown(ctx)
	logger.Info("server stopped gracefully")
}
