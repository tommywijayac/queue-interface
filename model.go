package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// [TODO] actual database field type may be different. modify here
// Store raw database data
type PatientLog struct {
	Room string    `json:"room"`
	Time time.Time `json:"time"`
}
type Patient struct {
	Records []PatientLog `json:"records"`
}

// Parse DateTime column according to our need (time only)
type RawTime []byte

func (t RawTime) Time() (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", string(t))
}

// API call
// [TODO] latency problem?
// [TODO] diff branch diff API endpoint
// dbAPI := fmt.Sprintf("http://localhost:8080/?id=%s&action=I", fullId)
// response, err := http.Get(dbAPI)
// if err != nil {
// 	http.Error(w, err.Error(), http.StatusInternalServerError)
// }
// defer response.Body.Close()
// // [TODO] response header evaluation?
// responseData, err := ioutil.ReadAll(response.Body)
// if err != nil {
// 	http.Error(w, err.Error(), http.StatusInternalServerError)
// }
// var patient Patient
// err = json.Unmarshal(responseData, &patient)
// if err != nil {
// 	http.Error(w, err.Error(), http.StatusInternalServerError)
// }

func GetQueueLogs(db *sql.DB, id string) ([]PatientLog, error) {
	// Testing purpose: use static date so we dont need to modify sql everyday
	// [TODO] production must use dynamic time
	//var date = time.Now().Format("2006-01-02")
	date := "2021-04-18"

	// Read data from database
	rows, err := db.Query("SELECT room_id, time FROM `queue` WHERE (cust_id=? AND date=? AND action=?)", id, date, "I")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	defer rows.Close()

	var logs []PatientLog
	var log PatientLog

	for rows.Next() {
		// [TODO] actual database field type may be different. modify here
		var time RawTime
		err := rows.Scan(&log.Room, &time)
		if err != nil {
			// [TODO] scanning error. should just skip? or how to handle this
			return nil, err
		}

		log.Time, err = time.Time()
		if err != nil {
			// [TODO] time conversion error
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
}
