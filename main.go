package main

import (
	"archivezipper/handlers"
	"archivezipper/task"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {	
	var taskManager = task.NewTaskManager(3)
	
	//NOTE Dependency injection
	handlers.Init(taskManager)

	r := mux.NewRouter()
	r.HandleFunc("/tasks", handlers.CreateTaskHandler).Methods("POST")
	r.HandleFunc("/tasks/{id}", handlers.GetTaskHandler).Methods("GET")
	r.HandleFunc("/tasks/{id}/files", handlers.AddFileHandler).Methods("POST")
	r.HandleFunc("/tasks/{id}/archive", handlers.DownloadArchiveHandler).Methods("GET")


	log.Println("Server started on port 8080 🚀")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
