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

type QueueLog struct {
	Room   string
	Action string
	Time   string
}

type Queue struct {
	Branch    string
	Date      string
	Id        string `json:"id"`
	Logs      []QueueLog
	Highlight QueueLog
}

func main() {
	router := mux.NewRouter()
	// TODO: input form
	router.HandleFunc("/", HomeHandler).Methods("GET")
	// Sanitize input: valid ID is a 4 digit number
	router.HandleFunc("/{id:[0-9]{4}}", DisplayQueueHandler).Methods("GET")
	router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	// Start API
	server := &http.Server{
		Handler: router,
		Addr:    ":8081",
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

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello"))
}

func DisplayQueueHandler(w http.ResponseWriter, r *http.Request) {
	// Get sanitized ID from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Testing purpose: use static date so we dont need to modify sql everyday
	//var date = time.Now().Format("2006-01-02")
	var date = "2021-04-18"

	// Connect to database
	db, err := connectToDatabase()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Read data from database
	rows, err := db.Query("SELECT room_id, room, action, time FROM `queue` WHERE (cust_id=? AND date=?)", id, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		rows.Close()
		return
	}

	var room_id string
	var logs []QueueLog
	var log QueueLog

	for rows.Next() {
		var action_id string
		err := rows.Scan(&room_id, &log.Room, &action_id, &log.Time)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			break
		}

		// Pre-process data received from database
		room_id = fmt.Sprintf("%s%s", room_id, id)

		if action_id == "m" {
			log.Action = "masuk"
		} else if action_id == "k" {
			log.Action = "keluar"
		} else {
			log.Action = "undefined"
		}

		// Append
		logs = append(logs, log)
	}

	if len(logs) == 0 {
		// Assign default value
		logs = append(logs, QueueLog{
			Room:   "data tidak ditemukan",
			Action: "-",
			Time:   "-",
		})
		room_id = id
	}

	// Set last element as highlight
	var q Queue
	q.Id = room_id
	q.Highlight = logs[len(logs)-1]
	q.Logs = logs[:len(logs)-1]

	// Reverse the order of histories
	if len(q.Logs) > 1 {
		for i, j := 0, len(q.Logs)-1; i < j; i, j = i+1, j-1 {
			q.Logs[i], q.Logs[j] = q.Logs[j], q.Logs[i]
		}
	}

	// Render output
	// displayData, err := json.Marshal(data)
	//w.Header().Set("content-type", "application/json")
	// w.Write([]byte(displayData))
	var tmpl = template.Must(template.ParseFiles("template/index.html"))
	if err := tmpl.Execute(w, q); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func connectToDatabase() (*sql.DB, error) {
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/kmn_queue")
	if err != nil {
		return nil, err
	}

	return db, nil
}
