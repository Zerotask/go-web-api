package main

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/zerotask/go-web-api/internal/taskstore"
)

type taskServer struct {
	store *taskstore.TaskStore
}

func NewTaskServer() *taskServer {
	store := taskstore.New()
	return &taskServer{store: store}
}

func (ts *taskServer) createTaskHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling task create at %s\n", req.URL.Path)

	// Types used internally in this handler to (de-)serialize the request and
	// response from/to JSON.
	type RequestTask struct {
		Text string    `json:"text"`
		Tags []string  `json:"tags"`
		Due  time.Time `json:"due"`
	}

	type ResponseId struct {
		Id int `json:"id"`
	}

	// Enforce a JSON Content-Type.
	contentType := req.Header.Get("Content-Type")
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if mediatype != "application/json" {
		http.Error(w, "expect application/json Content-Type", http.StatusUnsupportedMediaType)
		return
	}

	// Decode request body to RequestTask struct
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	var rt RequestTask
	if err := dec.Decode(&rt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Store new task
	id := ts.store.CreateTask(rt.Text, rt.Tags, rt.Due)

	// Encode reponse
	renderJSON(w, ResponseId{Id: id})
}

func (ts *taskServer) getAllTasksHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling get all tasks at %s\n", req.URL.Path)

	tasks := ts.store.GetAllTasks()
	renderJSON(w, tasks)
}

func (ts *taskServer) getTaskHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling get task at %s\n", req.URL.Path)

	// Read ID from the request
	id, err := strconv.Atoi(mux.Vars(req)["id"])
	if err != nil {
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}

	// Get task by ID
	task, err := ts.store.GetTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	renderJSON(w, task)
}

func (ts *taskServer) deleteTaskHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling delete task at %s\n", req.URL.Path)

	// Get ID by request
	id, _ := strconv.Atoi(mux.Vars(req)["id"])

	// Delte by ID
	err := ts.store.DeleteTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}
}

func (ts *taskServer) deleteAllTasksHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling delete all tasks at %s\n", req.URL.Path)
	ts.store.DeleteAllTasks()
}

func (ts *taskServer) tagHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling tasks by tag at %s\n", req.URL.Path)

	if req.Method != http.MethodGet {
		http.Error(w, fmt.Sprintf("expect method GET /tag/<tag>, got %v", req.Method), http.StatusMethodNotAllowed)
		return
	}

	path := strings.Trim(req.URL.Path, "/")
	pathParts := strings.Split(path, "/")
	if len(pathParts) < 2 {
		http.Error(w, "expect /tag/<tag> path", http.StatusBadRequest)
		return
	}
	tag := pathParts[1]

	tasks := ts.store.GetTasksByTag(tag)
	renderJSON(w, tasks)
}

func (ts *taskServer) dueHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling tasks by due at %s\n", req.URL.Path)

	if req.Method != http.MethodGet {
		http.Error(w, fmt.Sprintf("expect method GET /due/<date>, got %v", req.Method), http.StatusMethodNotAllowed)
		return
	}

	path := strings.Trim(req.URL.Path, "/")
	pathParts := strings.Split(path, "/")

	badRequestError := func() {
		http.Error(w, fmt.Sprintf("expect /due/<year>/<month>/<day>, got %v", req.URL.Path), http.StatusBadRequest)
	}
	if len(pathParts) != 4 {
		badRequestError()
		return
	}

	year, err := strconv.Atoi(pathParts[1])
	if err != nil {
		badRequestError()
		return
	}
	month, err := strconv.Atoi(pathParts[2])
	if err != nil || month < int(time.January) || month > int(time.December) {
		badRequestError()
		return
	}
	day, err := strconv.Atoi(pathParts[3])
	if err != nil {
		badRequestError()
		return
	}

	tasks := ts.store.GetTasksByDueDate(year, time.Month(month), day)
	renderJSON(w, tasks)
}

func renderJSON(w http.ResponseWriter, data interface{}) {
	json, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func main() {
	router := mux.NewRouter()
	router.StrictSlash(true)
	server := NewTaskServer()
	router.HandleFunc("/tasks/", server.createTaskHandler).Methods("POST")
	router.HandleFunc("/tasks/", server.getAllTasksHandler).Methods("GET")
	router.HandleFunc("/tasks/", server.deleteAllTasksHandler).Methods("DELETE")
	router.HandleFunc("/tasks/{id:[0-9]+}/", server.getTaskHandler).Methods("GET")
	router.HandleFunc("/tasks/{id:[0-9]+}/", server.deleteTaskHandler).Methods("DELETE")
	router.HandleFunc("/tag/{tag}/", server.tagHandler).Methods("GET")
	router.HandleFunc("/due/{year:[0-9]+}/{month:[0-9]+}/{day:[0-9]+}/", server.dueHandler).Methods("GET")

	godotenv.Load()
	port := os.Getenv("SERVER_PORT")
	log.Println("Port: ", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
