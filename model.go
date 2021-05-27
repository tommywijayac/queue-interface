package main

import (
	"database/sql"
	"errors"
	"fmt"
)

// [TODO] actual database field type may be different. modify here
// Store raw database data
type QueueLog struct {
	Room string
	Time string
}

func GetQueueLogs(db *sql.DB, id string) ([]QueueLog, error) {
	// Testing purpose: use static date so we dont need to modify sql everyday
	// [TODO] production must use dynamic time
	//var date = time.Now().Format("2006-01-02")
	date := "2021-04-16"

	// Read data from database
	rows, err := db.Query("SELECT room_id, time FROM `queue` WHERE (cust_id=? AND date=? AND action=?)", id, date, "I")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	defer rows.Close()

	var logs []QueueLog
	var log QueueLog

	for rows.Next() {
		// [TODO] actual database field type may be different. modify here
		err := rows.Scan(&log.Room, &log.Time)
		if err != nil {
			// [TODO] scanning error. should just skip? or how to handle this
			return nil, err
		}

		// Append
		logs = append(logs, log)
	}

	if len(logs) == 0 {
		return nil, errors.New("Data tidak ditemukan")
	} else {
		return logs, nil
	}

	// else
	// // Reverse the order of histories
	// // Should be sorted by time inherently because logging is always moving forward
	// // However we just want to be sure
	// if len(logs) > 1 {
	// 	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
	// 		logs[i], logs[j] = logs[j], logs[i]
	// 	}
	// }
}
