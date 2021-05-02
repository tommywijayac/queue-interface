package main

import (
	"database/sql"
	"fmt"
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

func GetQueueLogs(db *sql.DB, id string) (string, []QueueLog, error) {
	// Testing purpose: use static date so we dont need to modify sql everyday
	//var date = time.Now().Format("2006-01-02")
	date := "2021-04-18"

	// Read data from database
	rows, err := db.Query("SELECT room_id, room, action, time FROM `queue` WHERE (cust_id=? AND date=?)", id, date)
	if err != nil {
		return id, nil, err
	}

	defer rows.Close()

	var room_id string
	var logs []QueueLog
	var log QueueLog

	for rows.Next() {
		var action_id string
		err := rows.Scan(&room_id, &log.Room, &action_id, &log.Time)
		if err != nil {
			return id, nil, err
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
	} else
	// Reverse the order of histories
	if len(logs) > 1 {
		for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
			logs[i], logs[j] = logs[j], logs[i]
		}
	}

	return room_id, logs, nil
}
