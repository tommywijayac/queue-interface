package main

import (
	"log"
	"os"
	"testing"
	"time"
)

func TestConstructRoomListBasedOnTime(t *testing.T) {
	process := "pol"
	// RoomMap must be populated as reference. That needs logger too..
	file, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal("Fail to initialize logger!")
	}
	InfoLogger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	AppConfig.readConfig()

	ctime := time.Now()

	type Test struct {
		name string
		args []PatientLog
		want []RoomDisplay
	}

	var tests = []Test{
		{
			name: "all",
			args: []PatientLog{
				{Group: "REG", Time: ctime},
				{Group: "RM", Time: ctime.Add(time.Second * 1)},
				{Group: "PA", Time: ctime.Add(time.Second * 2)},
				{Group: "REF", Time: ctime.Add(time.Second * 3)},
				{Group: "POLI", Time: ctime.Add(time.Second * 4)},
			},
			want: []RoomDisplay{
				{Name: "Registrasi", IsActive: false},
				{Name: "Rekam Medik", IsActive: false},
				{Name: "Pemeriksaan Awal", IsActive: false},
				{Name: "Refraksi", IsActive: false},
				{Name: "Ruang Konsul", IsActive: true},
			},
		}, {
			name: "mix",
			args: []PatientLog{
				{Group: "RM", Time: ctime},
				{Group: "POLI", Time: ctime.Add(time.Second * 1)},
				{Group: "REF", Time: ctime.Add(time.Second * 2)},
			},
			want: []RoomDisplay{
				{Name: "Rekam Medik", IsActive: false},
				{Name: "Ruang Konsul", IsActive: false},
				{Name: "Refraksi", IsActive: true},
			},
		}, {
			name: "dupe-sequential",
			args: []PatientLog{
				{Group: "RM", Time: ctime},
				{Group: "RM", Time: ctime},
				{Group: "RM", Time: ctime.Add(time.Second * 1)},
				{Group: "REF", Time: ctime.Add(time.Second * 2)},
				{Group: "REF", Time: ctime.Add(time.Second * 3)},
				{Group: "REF", Time: ctime.Add(time.Second * 3)},
				{Group: "POLI", Time: ctime.Add(time.Second * 4)},
				{Group: "POLI", Time: ctime.Add(time.Second * 5)},
			},
			want: []RoomDisplay{
				{Name: "Rekam Medik", Time: ctime.Format("15:04:05"), IsActive: false},
				{Name: "Refraksi", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Konsul", Time: ctime.Add(time.Second * 4).Format("15:04:05"), IsActive: true},
			},
		}, {
			name: "dupe-mixed",
			args: []PatientLog{
				{Group: "RM", Time: ctime},
				{Group: "REF", Time: ctime.Add(time.Second * 1)},
				{Group: "POLI", Time: ctime.Add(time.Second * 2)},
				{Group: "REF", Time: ctime.Add(time.Second * 3)},
				{Group: "POLI", Time: ctime.Add(time.Second * 4)},
				{Group: "RM", Time: ctime.Add(time.Second * 5)},
			},
			want: []RoomDisplay{
				{Name: "Rekam Medik", Time: ctime.Format("15:04:05"), IsActive: false},
				{Name: "Refraksi", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Konsul", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: true},
			},
		},
	}

	for _, tt := range tests {
		get := ConstructRoomListBasedOnTime(tt.args, process)

		if len(get) != len(tt.want) {
			t.Fatalf("case %v: different length: get %v, want %v", tt.name, len(get), len(tt.want))
		}

		for i := 0; i < len(get); i++ {
			if get[i].Name != tt.want[i].Name {
				t.Errorf("case %v: wrong name: get %v want %v", tt.name, get[i].Name, tt.want[i].Name)
				continue
			}
			if get[i].IsActive != tt.want[i].IsActive {
				t.Errorf("case %v: wrong active: room %v is active %v, should be %v",
					tt.name, get[i].Name, get[i].IsActive, tt.want[i].IsActive)
			}
		}
	}
}

func TestConstructRoomListBasedOnOrder(t *testing.T) {
	process := "opr"
	ctime := time.Now()

	// RoomMap must be populated as reference. That needs logger too..
	file, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal("Fail to initialize logger!")
	}
	InfoLogger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	AppConfig.readConfig()

	type Test struct {
		name string
		args []PatientLog
		want []RoomDisplay
	}

	tests := []Test{
		{
			name: "ideal",
			args: []PatientLog{
				{Room: "PREOP", Time: ctime.Add(time.Second * 1)},
				{Room: "OT1", Time: ctime.Add(time.Second * 2)},
				{Room: "LAS", Time: ctime.Add(time.Second * 3)},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: ctime.Add(time.Second * 1).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Tindakan", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Pemulihan", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: true},
			},
		}, {
			name: "ideal-diff-branch",
			args: []PatientLog{
				{Room: "PREO", Time: ctime.Add(time.Second * 1)},
				{Room: "OT3", Time: ctime.Add(time.Second * 2)},
				{Room: "POSO", Time: ctime.Add(time.Second * 3)},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: ctime.Add(time.Second * 1).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Tindakan", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Pemulihan", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: true},
			},
		}, {
			name: "mix",
			args: []PatientLog{
				{Room: "OT1", Time: ctime.Add(time.Second * 1)},
				{Room: "LAS", Time: ctime.Add(time.Second * 2)},
				{Room: "PREOP", Time: ctime.Add(time.Second * 3)},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Tindakan", Time: ctime.Add(time.Second * 1).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Pemulihan", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: true},
			},
		}, {
			name: "dupe",
			args: []PatientLog{
				{Room: "PREOP", Time: ctime.Add(time.Second * 1)},
				{Room: "PREOP", Time: ctime.Add(time.Second * 2)},
				{Room: "PREOP", Time: ctime.Add(time.Second * 3)},
				{Room: "OT3", Time: ctime.Add(time.Second * 4)},
				{Room: "OT3", Time: ctime.Add(time.Second * 5)},
				{Room: "LAS", Time: ctime.Add(time.Second * 6)},
				{Room: "LAS", Time: ctime.Add(time.Second * 7)},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: ctime.Add(time.Second * 1).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Tindakan", Time: ctime.Add(time.Second * 4).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Pemulihan", Time: ctime.Add(time.Second * 6).Format("15:04:05"), IsActive: true},
			},
		}, {
			name: "jump-begin",
			args: []PatientLog{
				{Room: "OT1", Time: ctime.Add(time.Second * 2)},
				{Room: "LAS", Time: ctime.Add(time.Second * 3)},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: "-", IsActive: false},
				{Name: "Ruang Tindakan", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Pemulihan", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: true},
			},
		}, {
			name: "jump-middle",
			args: []PatientLog{
				{Room: "PREOP", Time: ctime.Add(time.Second * 1)},
				{Room: "LAS", Time: ctime.Add(time.Second * 3)},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: ctime.Add(time.Second * 1).Format("15:04:05"), IsActive: false},
				{Name: "Ruang Tindakan", Time: "-", IsActive: false},
				{Name: "Ruang Pemulihan", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: true},
			},
		}, {
			name: "incomplete",
			args: []PatientLog{
				{Room: "OT1", Time: ctime.Add(time.Second * 2)},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: "-", IsActive: false},
				{Name: "Ruang Tindakan", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: true},
				{Name: "Ruang Pemulihan", Time: "-", IsActive: false},
			},
		},
	}

	for _, tt := range tests {
		get := ConstructRoomListBasedOnOrder(tt.args, process)

		if len(get) != len(tt.want) {
			t.Fatalf("case %v: different length: get %v, want %v", tt.name, len(get), len(tt.want))
		}

		for i := 0; i < len(get); i++ {
			if get[i].Name != tt.want[i].Name {
				t.Errorf("case %v: wrong name: get %v want %v", tt.name, get[i].Name, tt.want[i].Name)
				continue
			}
			if get[i].IsActive != tt.want[i].IsActive {
				t.Errorf("case %v: wrong active: room %v is active %v, should be %v",
					tt.name, get[i].Name, get[i].IsActive, tt.want[i].IsActive)
				continue
			}
			if get[i].Time != tt.want[i].Time {
				t.Errorf("case %v: wrong time: get %v want %v", tt.name, get[i].Time, tt.want[i].Time)
				continue
			}
		}
	}
}
