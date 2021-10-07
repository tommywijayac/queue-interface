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
				{Group: "REG", Time: ctime, Status: "I"},
				{Group: "RM", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "PA", Time: ctime.Add(time.Second * 2), Status: "I"},
				{Group: "REF", Time: ctime.Add(time.Second * 3), Status: "I"},
				{Group: "POLI", Time: ctime.Add(time.Second * 4), Status: "I"},
				{Group: "LAB", Time: ctime.Add(time.Second * 5), Status: "I"},
				{Group: "PP", Time: ctime.Add(time.Second * 6), Status: "I"},
			},
			want: []RoomDisplay{
				{Name: "Registrasi", Time: ctime.Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Rekam Medik", Time: ctime.Add(time.Second * 1).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Pemeriksaan Awal", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Refraksi", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Ruang Konsul", Time: ctime.Add(time.Second * 4).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Laboratorium", Time: ctime.Add(time.Second * 5).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Pemeriksaan Penunjang", Time: ctime.Add(time.Second * 6).Format("15:04:05"), IsActive: true, TimeOut: "-"},
			},
		}, {
			name: "mix",
			args: []PatientLog{
				{Group: "RM", Time: ctime, Status: "I"},
				{Group: "POLI", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "REF", Time: ctime.Add(time.Second * 2), Status: "I"},
			},
			want: []RoomDisplay{
				{Name: "Rekam Medik", Time: ctime.Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Ruang Konsul", Time: ctime.Add(time.Second * 1).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Refraksi", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: true, TimeOut: "-"},
			},
		}, {
			name: "inactive",
			args: []PatientLog{
				{Group: "RM", Time: ctime, Status: "I"},
				{Group: "POLI", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "POLI", Time: ctime.Add(time.Second * 2), Status: "O"},
			},
			want: []RoomDisplay{
				{
					Name: "Rekam Medik", IsActive: false,
					Time:    ctime.Format("15:04:05"),
					TimeOut: "-",
				}, {
					Name: "Ruang Konsul", IsActive: false,
					Time:    ctime.Add(time.Second * 1).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 2).Format("15:04:05"),
				},
			},
		}, {
			name: "inactive-2",
			args: []PatientLog{
				{Group: "RM", Time: ctime, Status: "I"},
				{Group: "POLI", Time: ctime.Add(time.Second * 2), Status: "O"},
			},
			want: []RoomDisplay{
				{
					Name: "Rekam Medik", IsActive: false,
					Time:    ctime.Format("15:04:05"),
					TimeOut: "-",
				}, {
					Name: "Ruang Konsul", IsActive: false,
					Time:    "-",
					TimeOut: ctime.Add(time.Second * 2).Format("15:04:05"),
				},
			},
		}, {
			name: "dupe-sequential",
			args: []PatientLog{
				{Group: "RM", Time: ctime, Status: "I"},
				{Group: "RM", Time: ctime, Status: "O"},
				{Group: "RM", Time: ctime.Add(time.Second * 1), Status: "O"},
				{Group: "REF", Time: ctime.Add(time.Second * 2), Status: "I"},
				{Group: "REF", Time: ctime.Add(time.Second * 3), Status: "I"},
				{Group: "REF", Time: ctime.Add(time.Second * 3), Status: "O"},
				{Group: "POLI", Time: ctime.Add(time.Second * 4), Status: "I"},
				{Group: "POLI", Time: ctime.Add(time.Second * 5), Status: "I"},
			},
			want: []RoomDisplay{
				{
					Name: "Rekam Medik", IsActive: false,
					Time:    ctime.Format("15:04:05"),
					TimeOut: ctime.Format("15:04:05"),
				}, {
					Name: "Refraksi", IsActive: false,
					Time:    ctime.Add(time.Second * 2).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 3).Format("15:04:05"),
				}, {
					Name: "Ruang Konsul", IsActive: true,
					Time:    ctime.Add(time.Second * 4).Format("15:04:05"),
					TimeOut: "-",
				},
			},
		}, {
			name: "dupe-mixed",
			args: []PatientLog{
				{Group: "RM", Time: ctime, Status: "I"},
				{Group: "REF", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "POLI", Time: ctime.Add(time.Second * 2), Status: "I"},
				{Group: "REF", Time: ctime.Add(time.Second * 3), Status: "I"},
				{Group: "POLI", Time: ctime.Add(time.Second * 4), Status: "I"},
				{Group: "RM", Time: ctime.Add(time.Second * 5), Status: "I"},
			},
			want: []RoomDisplay{
				{Name: "Rekam Medik", Time: ctime.Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Refraksi", Time: ctime.Add(time.Second * 1).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Ruang Konsul", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Refraksi", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Ruang Konsul", Time: ctime.Add(time.Second * 4).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Rekam Medik", Time: ctime.Add(time.Second * 5).Format("15:04:05"), IsActive: true, TimeOut: "-"},
			},
		}, {
			name: "seq-pps",
			args: []PatientLog{
				{Group: "PP", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "PP", Time: ctime.Add(time.Second * 2), Status: "O"},
				{Group: "PP", Time: ctime.Add(time.Second * 3), Status: "I"},
				{Group: "PP", Time: ctime.Add(time.Second * 4), Status: "O"},
			},
			want: []RoomDisplay{
				{
					Name: "Pemeriksaan Penunjang", IsActive: false,
					Time:    ctime.Add(time.Second * 1).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 4).Format("15:04:05"),
				},
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
			if get[i].Time != tt.want[i].Time {
				t.Errorf("case %v: room %v wrong time in: get %v want %v", tt.name, get[i].Name, get[i].Time, tt.want[i].Time)
				continue
			}
			if get[i].TimeOut != tt.want[i].TimeOut {
				t.Errorf("case %v: room %v wrong time out: get %v want %v", tt.name, get[i].Name, get[i].TimeOut, tt.want[i].TimeOut)
				continue
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
				{Group: "PREOP", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "PREOP", Time: ctime.Add(time.Second * 1), Status: "O"},
				{Group: "OT", Time: ctime.Add(time.Second * 2), Status: "I"},
				{Group: "OT", Time: ctime.Add(time.Second * 2), Status: "O"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 3), Status: "I"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 3), Status: "O"},
			},
			want: []RoomDisplay{
				{
					Name: "Ruang Persiapan Tindakan", IsActive: false,
					Time:    ctime.Add(time.Second * 1).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 1).Format("15:04:05"),
				}, {
					Name: "Ruang Tindakan", IsActive: false,
					Time:    ctime.Add(time.Second * 2).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 2).Format("15:04:05"),
				}, {
					Name: "Ruang Pemulihan", IsActive: false,
					Time:    ctime.Add(time.Second * 3).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 3).Format("15:04:05"),
				},
			},
		}, {
			name: "mix-order",
			args: []PatientLog{
				{Group: "OT", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 2), Status: "I"},
				{Group: "PREOP", Time: ctime.Add(time.Second * 3), Status: "I"},
				{Group: "OT", Time: ctime.Add(time.Second * 4), Status: "O"},
				{Group: "PREOP", Time: ctime.Add(time.Second * 5), Status: "O"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 6), Status: "O"},
			},
			want: []RoomDisplay{
				{
					Name: "Ruang Persiapan Tindakan", IsActive: false,
					Time:    ctime.Add(time.Second * 3).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 5).Format("15:04:05"),
				}, {
					Name: "Ruang Tindakan", IsActive: false,
					Time:    ctime.Add(time.Second * 1).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 4).Format("15:04:05"),
				}, {
					Name: "Ruang Pemulihan", IsActive: false,
					Time:    ctime.Add(time.Second * 2).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 6).Format("15:04:05"),
				},
			},
		}, {
			name: "dupe",
			args: []PatientLog{
				{Group: "PREOP", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "PREOP", Time: ctime.Add(time.Second * 2), Status: "I"},
				{Group: "PREOP", Time: ctime.Add(time.Second * 3), Status: "I"},
				{Group: "PREOP", Time: ctime.Add(time.Second * 4), Status: "O"},
				{Group: "PREOP", Time: ctime.Add(time.Second * 5), Status: "O"},
				{Group: "OT", Time: ctime.Add(time.Second * 4), Status: "I"},
				{Group: "OT", Time: ctime.Add(time.Second * 5), Status: "I"},
				{Group: "OT", Time: ctime.Add(time.Second * 6), Status: "O"},
				{Group: "OT", Time: ctime.Add(time.Second * 7), Status: "O"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 8), Status: "I"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 9), Status: "I"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 10), Status: "O"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 11), Status: "O"},
			},
			want: []RoomDisplay{
				{
					Name: "Ruang Persiapan Tindakan", IsActive: false,
					Time:    ctime.Add(time.Second * 1).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 4).Format("15:04:05"),
				}, {
					Name: "Ruang Tindakan", IsActive: false,
					Time:    ctime.Add(time.Second * 4).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 6).Format("15:04:05"),
				}, {
					Name: "Ruang Pemulihan", IsActive: false,
					Time:    ctime.Add(time.Second * 8).Format("15:04:05"),
					TimeOut: ctime.Add(time.Second * 10).Format("15:04:05"),
				},
			},
		}, {
			name: "jump-begin",
			args: []PatientLog{
				{Group: "OT", Time: ctime.Add(time.Second * 2), Status: "I"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 3), Status: "I"},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: "-", IsActive: false, TimeOut: "-"},
				{Name: "Ruang Tindakan", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Ruang Pemulihan", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: true, TimeOut: "-"},
			},
		}, {
			name: "jump-middle",
			args: []PatientLog{
				{Group: "PREOP", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "PREPOST", Time: ctime.Add(time.Second * 3), Status: "I"},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: ctime.Add(time.Second * 1).Format("15:04:05"), IsActive: false, TimeOut: "-"},
				{Name: "Ruang Tindakan", Time: "-", IsActive: false, TimeOut: "-"},
				{Name: "Ruang Pemulihan", Time: ctime.Add(time.Second * 3).Format("15:04:05"), IsActive: true, TimeOut: "-"},
			},
		}, {
			name: "incomplete",
			args: []PatientLog{
				{Group: "OT", Time: ctime.Add(time.Second * 2), Status: "I"},
			},
			want: []RoomDisplay{
				{Name: "Ruang Persiapan Tindakan", Time: "-", IsActive: false, TimeOut: "-"},
				{Name: "Ruang Tindakan", Time: ctime.Add(time.Second * 2).Format("15:04:05"), IsActive: true, TimeOut: "-"},
				{Name: "Ruang Pemulihan", Time: "-", IsActive: false, TimeOut: "-"},
			},
		}, {
			name: "no OPR data",
			args: []PatientLog{
				{Group: "REG", Time: ctime, Status: "I"},
				{Group: "RM", Time: ctime.Add(time.Second * 1), Status: "I"},
				{Group: "PA", Time: ctime.Add(time.Second * 2), Status: "I"},
				{Group: "REF", Time: ctime.Add(time.Second * 3), Status: "I"},
				{Group: "POLI", Time: ctime.Add(time.Second * 4), Status: "I"},
				{Group: "LAB", Time: ctime.Add(time.Second * 5), Status: "I"},
				{Group: "PP", Time: ctime.Add(time.Second * 6), Status: "I"},
			},
			want: nil,
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
				t.Errorf("case %v: room %v wrong time in: get %v want %v", tt.name, get[i].Name, get[i].Time, tt.want[i].Time)
				continue
			}
			if get[i].TimeOut != tt.want[i].TimeOut {
				t.Errorf("case %v: room %v wrong time out: get %v want %v", tt.name, get[i].Name, get[i].TimeOut, tt.want[i].TimeOut)
				continue
			}
		}
	}
}
