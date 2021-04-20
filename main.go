package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/gorilla/mux"
)

type QLog struct {
	Room   string
	Action string
	Time   string
}

type Queue struct {
	Id        string
	Logs      []QLog
	Highlight QLog
}

// Can be modified to JSON..
type DisplayData struct {
	Branch string
	Date   string
	Data   Queue
}

func main() {
	http.HandleFunc("/", routeIndex)
	http.Handle("/assets/",
		http.StripPrefix("/assets/",
			http.FileServer(http.Dir("assets"))))

	server := new(http.Server)
	server.Addr = ":8081"
	fmt.Println("Launched at localhost", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		panic(err.Error())
	}
}

func routeIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// TODO: Receive id from Request

		// TODO: Receive all data in table related to given id (filter by date as well)
		var id = "0001"
		var date = time.Now().Format("2006-01-02")
		//var date = "2021-04-16"

		// Select from table value 'room' 'action' 'time' where id=[id] and date=date
		db, err := connectToDatabase()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		rows, err := db.Query("SELECT room_id, room, action, time FROM `queue` WHERE (cust_id=? AND date=?)", id, date)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			rows.Close()
			return
		}

		var room_id string
		var logs []QLog
		var log QLog

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

			fmt.Println(logs)
		}

		// Check for empty rows
		if len(logs) == 0 {
			// Assign default value
			logs = append(logs, QLog{
				Room:   "-",
				Action: "-",
				Time:   "-",
			})

			room_id = id
		}

		// Set last element as highlight
		var data Queue
		data.Id = room_id
		data.Highlight = logs[len(logs)-1]
		data.Logs = logs[:len(logs)-1]

		// Reverse the order of histories
		if len(data.Logs) > 1 {
			for i, j := 0, len(data.Logs)-1; i < j; i, j = i+1, j-1 {
				data.Logs[i], data.Logs[j] = data.Logs[j], data.Logs[i]
			}
		}

		// Render output
		var disp_data = DisplayData{
			Branch: "Kebon Jeruk",
			Date:   time.Now().Format("01-02-2006"),
			Data:   data,
		}

		var tmpl = template.Must(template.ParseFiles("template/index.html"))
		if err := tmpl.Execute(w, disp_data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Error(w, "", http.StatusBadRequest)
}

func connectToDatabase() (*sql.DB, error) {
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/kmn_queue")
	if err != nil {
		return nil, err
	}

	return db, nil
}
