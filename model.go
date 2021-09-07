package main

import (
	"database/sql"
	"time"
)

// [TODO] actual database field type may be different. modify here
// Store raw database data
type PatientLog struct {
	Group  string
	Room   string
	Time   time.Time
	Status string
}

// Parse DateTime column according to our need (time only)
type RawTime []byte

func (t RawTime) Time() (time.Time, error) {
	return time.Parse("15:04:05", string(t))
}

func GetQueueLogs(db *sql.DB, branchID, patientID string) ([]PatientLog, error) {
	var date string
	if AppConfig.IsDev {
		date = "2021-08-24"
	} else {
		date = time.Now().Format("2006-01-02") //YYYY-MM-DD
	}

	// Read data from database
	rows, err := db.Query("SELECT DISTINCT kelompok, ruang, jam, status FROM antri WHERE (lokasi=? AND nomor=? AND tanggal=? AND status IN ('I','O')) ORDER BY jam", branchID, patientID, date)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var logs []PatientLog
	var log PatientLog

	for rows.Next() {
		var time RawTime
		err := rows.Scan(&log.Group, &log.Room, &time, &log.Status)
		if err != nil {
			return nil, err
		}

		// room nil is ok (no details), time can't be nil
		if log.Group == "" {
			continue
		}

		log.Time, err = time.Time()
		if err != nil {
			return nil, err
		}

		logs = append(logs, log)
	}

	if len(logs) == 0 {
		return nil, sql.ErrNoRows
	} else {
		return logs, nil
	}
}
