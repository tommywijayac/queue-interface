package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

type ProcessData struct {
	Name string
	Code string
}

// always return constant slice of ProcessData
// here, we do hard-code limitation on allowable process
// if new process were to be added, must modify below function and also config file
func getProcess() []ProcessData {
	var processes []ProcessData
	processes = append(processes, ProcessData{
		Code: "opr",
		Name: "Operasi",
	})
	processes = append(processes, ProcessData{
		Code: "pol",
		Name: "Poli / Rawat Jalan",
	})
	return processes
}

func validateProcess(processCode string) bool {
	exp := regexp.MustCompile(`^[a-z]{3}$`)
	if valid := exp.MatchString(processCode); !valid {
		return false
	}

	processesRef := getProcess()
	for _, processRef := range processesRef {
		if processCode == processRef.Code {
			return true
		}
	}
	return false
}

const MAX_ROOM int = 10

type App struct {
	Router     *mux.Router
	serverCert string
	serverKey  string

	TemplateHome    *template.Template
	TemplateSearch  *template.Template
	TemplateDisplay *template.Template
	TemplateError   *template.Template
	TemplateNoData  *template.Template

	DB       []*sql.DB
	Branches []BranchData

	RoomMap     map[string]map[string]RoomData //process code -> room code -> room data
	OrderedRoom map[string]map[int]string      //process code -> room order -> room code
}

type RoomData struct {
	Name      string `mapstructure:"name"`
	Code      string `mapstructure:"code"`
	GroupCode string `mapstructure:"group-code"`
	Order     int    `mapstructure:"order"`
}

type RoomDisplay struct {
	IsActive bool
	Name     string
	Time     string
}

type BranchData struct {
	Name string `mapstructure:"name"`
	Code string `mapstructure:"code"`

	DatabaseAddr string `mapstructure:"db-addr"`
	DatabaseUser string `mapstructure:"db-user"`
	DatabasePswd string `mapstructure:"db-pswd"`
	DatabaseName string `mapstructure:"db-name"`
}

func validateBranch(branchesRef []BranchData, branchCode string) bool {
	exp := regexp.MustCompile(`^[a-z]{3}$`)
	if valid := exp.MatchString(branchCode); !valid {
		return false
	}

	for _, branchRef := range branchesRef {
		if branchCode == branchRef.Code {
			return true
		}
	}
	return false
}

func (theApp *App) GetBranchInfo(branchCode string) (string, int) {
	// Match URL path {branch} with config file
	branch := ""
	i := 0
	for i < len(theApp.Branches) {
		if theApp.Branches[i].Code == branchCode {
			branch = theApp.Branches[i].Name
			break
		}
		i++
	}

	return branch, i
}

type neuteredFileSystem struct {
	fs http.FileSystem
}

var fns = template.FuncMap{
	"last": func(x int, a interface{}) bool {
		return x == reflect.ValueOf(a).Len()-1
	},
}

func (nfs neuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if s.IsDir() {
		if _, err := nfs.fs.Open("/index.html"); err != nil {
			closeErr := f.Close()
			if closeErr != nil {
				return nil, closeErr
			}

			return nil, err
		}
	}

	return f, nil
}

func (theApp *App) ReadConfig() bool {
	var err error

	// Read configuration file
	viper.SetConfigFile("./config.json")
	err = viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			panic(fmt.Errorf("ER001: Fatal error - config not found"))
		} else {
			panic(fmt.Errorf("ER002: Fatal error - config file: %s \n", err.Error()))
		}
	}

	// Read branch configuration
	err = viper.UnmarshalKey("branch", &theApp.Branches)
	if err != nil {
		panic(fmt.Errorf("ER003: Fatal error - reading config file: %s \n", err.Error()))
	}
	if len(theApp.Branches) == 0 {
		panic(fmt.Errorf("ER005: Fatal error - no Branch endpoint defined"))
	}

	// Read room configuration
	theApp.RoomMap = make(map[string]map[string]RoomData)
	theApp.RoomMap["opr"] = make(map[string]RoomData)
	theApp.RoomMap["pol"] = make(map[string]RoomData)

	theApp.OrderedRoom = make(map[string]map[int]string)
	theApp.OrderedRoom["opr"] = make(map[int]string)
	theApp.OrderedRoom["pol"] = make(map[int]string)

	theApp.readRoomConfig("opr")
	theApp.readRoomConfig("pol")

	return true
}

func (theApp *App) readRoomConfig(process string) {
	var rooms []RoomData
	var key string

	key = "process." + process + ".room"
	err := viper.UnmarshalKey(key, &rooms)
	if err != nil {
		panic(fmt.Errorf("ER003: Fatal error - reading config file: %s \n", err.Error()))
	}
	// Limit the number of visible room regardless of config file
	// (hard-coded limitation for Released application)
	key = "process." + process + ".visible-room"
	roomCount := viper.GetInt(key)
	if roomCount < 0 {
		roomCount = 0
	} else if roomCount > MAX_ROOM {
		roomCount = MAX_ROOM
	}
	rooms = rooms[:roomCount] //prune

	// Validate data
	if len(rooms) == 0 {
		panic(fmt.Errorf("ER004: Fatal config error - missing room details"))
	}

	// Map room. We can't directly marshal to map because we add hard-coded limitation with trimming
	// which is easier done in slice
	collision := 1
	for i := 0; i < len(rooms); i++ {
		theApp.RoomMap[process][rooms[i].Code] = rooms[i]

		if _, exist := theApp.OrderedRoom[process][rooms[i].Order]; !exist {
			theApp.OrderedRoom[process][rooms[i].Order] = rooms[i].Code
		} else {
			theApp.OrderedRoom[process][rooms[i].Order+collision] = rooms[i].Code
			collision++
		}
	}
}

func (theApp *App) Initialize() {
	var err error

	theApp.DB = make([]*sql.DB, len(theApp.Branches))
	for i, branch := range theApp.Branches {
		// Connect to database
		connectionString := fmt.Sprintf("%s:%s@tcp(%s)/%s",
			branch.DatabaseUser,
			branch.DatabasePswd,
			branch.DatabaseAddr,
			branch.DatabaseName)

		theApp.DB[i], err = sql.Open("mysql", connectionString)
		if err != nil {
			panic("sql open err" + err.Error())
		}
	}

	// Initialize routes
	theApp.Router = mux.NewRouter()
	theApp.Router.HandleFunc("/", theApp.HomeHandler).Methods("GET")
	theApp.Router.HandleFunc("/search", theApp.DisplayQueueHandler).Methods("GET")
	theApp.Router.NotFoundHandler = http.HandlerFunc(theApp.NotFoundHandler)

	fileserver := http.FileServer(neuteredFileSystem{http.Dir("static")})
	theApp.Router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileserver))

	// Template caching: parse templates here instead in request to avoid delay
	theApp.TemplateHome = template.Must(template.ParseFiles("template/index.html", "template/_header.html"))
	theApp.TemplateDisplay = template.Must(template.New("queue.html").Funcs(fns).ParseFiles("template/queue.html", "template/_header.html", "template/_footer.html"))
	theApp.TemplateNoData = template.Must(template.ParseFiles("template/nodata.html", "template/_header.html"))
	theApp.TemplateError = template.Must(template.ParseFiles("template/error404.html", "template/_header.html"))
}

func (theApp *App) Run(addr string) {
	theApp.serverCert = "env/localhost.crt"
	theApp.serverKey = "env/localhost.key"

	server := &http.Server{
		Handler: theApp.Router,
		Addr:    ":" + addr,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  60 * time.Second,
		// TLSConfig: &tls.Config{
		// 	ServerName: "localhost",
		// },
	}
	fmt.Println("Launched at localhost", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		panic(err.Error())
	}

	// if err := server.ListenAndServeTLS(theApp.serverCert, theApp.serverKey); err != nil {
	// 	panic(err.Error())
	// }
}

func SanitizeID(id string) (string, error) {
	// remove leading & trailing whitespace
	// expNoPrefixZeroes, err := regexp.Compile(`^0+`)
	// if err != nil {
	// 	fmt.Printf("err cleaning ID! %s\n", err.Error())
	// 	return "", err
	// }
	// cleanId := expNoPrefixZeroes.ReplaceAllString(id, "")
	// fmt.Println(cleanId)

	// capitalize
	cleanId := strings.ToUpper(id)

	return cleanId, nil
}

func validateID(id string) bool {
	validQueueExp := regexp.MustCompile(`^[A-Z][0-9]{3}$`)
	return validQueueExp.MatchString(id)
}

func GetFooterText(branchCode string) string {
	var footer string

	// Read footer from runningtext.json
	viper.SetConfigFile("./runningtext.json")
	err := viper.ReadInConfig()
	if err != nil {
		// by keeping footer as empty text, it won't be displayed
		footer = ""
	} else {
		footer = viper.GetString(branchCode)
	}

	return footer
}

func (theApp *App) HomeHandler(w http.ResponseWriter, r *http.Request) {
	var branchCopy []BranchData
	for _, branch := range theApp.Branches {
		branchCopy = append(branchCopy, BranchData{
			Name: branch.Name,
			Code: branch.Code,
		})
	}

	payload := map[string]interface{}{
		"Branches":  branchCopy,
		"Processes": getProcess(),
	}
	if err := theApp.TemplateHome.Execute(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (theApp *App) DisplayQueueHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Validate and sanitize branch
	branch := r.FormValue("branch")
	// fmt.Println(branch)
	if valid := validateBranch(theApp.Branches, branch); !valid {
		http.Error(w, "ERR: Invalid Branch selection", http.StatusBadRequest)
		// [TODO] redirect to index/search
		return
	}
	branchString, branchID := theApp.GetBranchInfo(branch)

	// Validate and sanitize process
	process := r.FormValue("process")
	// fmt.Println(process)
	if valid := validateProcess(process); !valid {
		http.Error(w, "ERR: Invalid Process selection", http.StatusBadRequest)
		// [TODO] redirect to index/search
		return
	}

	// Validate and sanitize queue number
	fullId := r.FormValue("qinput1") + r.FormValue("qinput2") + r.FormValue("qinput3") + r.FormValue("qinput4")
	fullId, _ = SanitizeID(fullId)
	if valid := validateID(fullId); !valid {
		http.Error(w, "ERR: Invalid Queue number", http.StatusBadRequest)
		// [TODO] redirect to index/search
		return
	}

	// [TODO] update database query based on actual database design
	logs, err := GetQueueLogs(theApp.DB[branchID], fullId)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			theApp.NoDataTemplateDisplay(w, r, fullId)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Arrange logs to room
	var roomDisplay []RoomDisplay = make([]RoomDisplay, 0)
	switch process {
	case "opr":
		roomDisplay = theApp.ConstructRoomListBasedOnOrder(logs, process)
	case "pol":
		roomDisplay = theApp.ConstructRoomListBasedOnTime(logs, process)
	}

	if len(roomDisplay) == 0 {
		theApp.NoDataTemplateDisplay(w, r, fullId)
	}

	payload := map[string]interface{}{
		"Branch": branchString,
		"Id":     fullId,
		"Rooms":  roomDisplay,
		"Footer": GetFooterText(branch),
	}

	// Render output
	if err := theApp.TemplateDisplay.Execute(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func (theApp *App) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	if tmplErr := theApp.TemplateError.Execute(w, nil); tmplErr != nil {
		http.Error(w, tmplErr.Error(), http.StatusInternalServerError)
	}
}

func (theApp *App) NoDataTemplateDisplay(w http.ResponseWriter, r *http.Request, id string) {
	payload := map[string]interface{}{
		"Id": id,
	}

	if err := theApp.TemplateNoData.Execute(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (theApp *App) ConstructRoomListBasedOnTime(logs []PatientLog, processCode string) []RoomDisplay {
	var roomDisplays []RoomDisplay

	// Sort PatientLog array based on time
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Time.Before(logs[j].Time)
	})

	// Translate Room code into Room name, and populate array result
	for _, log := range logs {
		if roomMap, exist := theApp.RoomMap[processCode][log.Room]; exist {
			var rd = RoomDisplay{
				Name:     roomMap.Name,
				Time:     log.Time.Format("15:04:05"),
				IsActive: false,
			}
			roomDisplays = append(roomDisplays, rd)
		}
	}

	// Set last room as active room
	roomDisplays[len(roomDisplays)-1].IsActive = true

	return roomDisplays
}

func (theApp *App) ConstructRoomListBasedOnOrder(logs []PatientLog, processCode string) []RoomDisplay {
	var roomDisplays []RoomDisplay = make([]RoomDisplay, 0)

	// Sort PatientLog array based on time
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Time.Before(logs[j].Time)
	})

	// Remember last room data (closest to current time), would be set as active room later
	activeRoom := logs[len(logs)-1].Room

	n := len(theApp.RoomMap[processCode])
	for i := 0; i < n; i++ {
		// order is 1-based
		if code, exist := theApp.OrderedRoom[processCode][i+1]; exist {
			var rd = RoomDisplay{
				Name:     theApp.RoomMap[processCode][code].Name,
				Time:     "",
				IsActive: code == activeRoom,
			}

			for _, log := range logs {
				if log.Room == code {
					rd.Time = log.Time.Format("15:04:05")
					break
				}
			}

			roomDisplays = append(roomDisplays, rd)
		}
	}

	return roomDisplays
}
