package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

type App struct {
	Router *mux.Router
	DB     *sql.DB

	BranchName string
}

func (theApp *App) Initialize(user, password, dbname, branchname string) {
	// Connect to database
	var err error
	connectionString := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s", user, password, dbname)
	theApp.DB, err = sql.Open("mysql", connectionString)
	if err != nil {
		panic(err.Error())
	}

	router := mux.NewRouter()
	// Initialize routes
	// TODO: input form
	router.HandleFunc("/", theApp.HomeHandler).Methods("GET")
	router.HandleFunc("/{id:[0-9]{4}}", theApp.DisplayQueueHandler).Methods("GET") // Sanitize input: valid ID is a 4 digit number
	router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	// Constant details
	theApp.BranchName = branchname
}

func (a *App) Run(addr string) {
	// Start API
	server := &http.Server{
		Handler: a.Router,
		Addr:    addr,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	fmt.Println("Launched at localhost", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		panic(err.Error())
	}
}

func (theApp *App) HomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello"))
}

func (theApp *App) DisplayQueueHandler(w http.ResponseWriter, r *http.Request) {
	// Get sanitized ID from URL
	vars := mux.Vars(r)
	id := vars["id"]

	room_id, logs, err := GetQueueLogs(theApp.DB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var q = Queue{
		Branch:    theApp.BranchName,
		Date:      time.Now().Format("02-01-2006"),
		Id:        room_id,
		Highlight: logs[0],
		Logs:      logs[1 : len(logs)-1],
	}

	// Render output
	var tmpl = template.Must(template.ParseFiles("template/index.html"))
	if err := tmpl.Execute(w, q); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}
