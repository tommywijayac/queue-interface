package main

import (
	"database/sql"
	"fmt"
	"html"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/microcosm-cc/bluemonday"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

type ProcessData struct {
	Name string
	Code string
}

// if new process were to be added, must modify below function and also config file
var (
	ProcessLibMap = map[string]string{
		"opr": "Operasi",
		"pol": "Poli / Rawat Jalan",
	}

	// For populating HTML controls, we want it to be consistent, so array is used
	ProcessLibArr = []ProcessData{
		{
			Code: "opr",
			Name: "Operasi",
		}, {
			Code: "pol",
			Name: "Poli / Rawat Jalan",
		},
	}
)

func validateProcess(processCode string) bool {
	exp := regexp.MustCompile(`^[a-z]{3}$`)
	if valid := exp.MatchString(processCode); !valid {
		return false
	}

	_, exist := ProcessLibMap[processCode]
	return exist
}

const MAX_ROOM int = 10

type App struct {
	Router *mux.Router

	TemplateHome             *template.Template
	TemplateSearch           *template.Template
	TemplateDisplay          *template.Template
	TemplateError            *template.Template
	TemplateLogin            *template.Template
	TemplateEditNotification *template.Template

	DB       []*sql.DB
	Branches []BranchData

	RoomMap     map[string]map[string]RoomData //process code -> room code -> room data
	OrderedRoom map[string]map[int]string      //process code -> room order -> room code

	notificationViper  *viper.Viper
	notificationPolicy *bluemonday.Policy
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

// Prevent directory traversal by serving index.html in our static web server
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

	s, _ := f.Stat()
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
		panic(fmt.Errorf("ER002: Fatal error - config file: %s", err.Error()))
	}

	// Read branch configuration
	err = viper.UnmarshalKey("branch", &theApp.Branches)
	if err != nil {
		panic(fmt.Errorf("ER003: Fatal error - reading config file: %s", err.Error()))
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

	// Read registered users
	viper.UnmarshalKey("registeredUsers", &creds)

	return true
}

func (theApp *App) readRoomConfig(process string) {
	var rooms []RoomData
	var key string

	key = fmt.Sprintf("process.%s.room", process)
	err := viper.UnmarshalKey(key, &rooms)
	if err != nil {
		panic(fmt.Errorf("ER003: Fatal error - reading config file: %s", err.Error()))
	}
	// Limit the number of visible room regardless of config file
	// (hard-coded limitation for Released application)
	key = fmt.Sprintf("process.%s.visible-room", process)
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
	theApp.Router.HandleFunc("/kmn-internal", theApp.InternalLoginHandler).Methods("GET", "POST")
	theApp.Router.HandleFunc("/kmn-internal/notification", theApp.InternalNotificationSettingGetHandler).Methods("GET")
	theApp.Router.HandleFunc("/kmn-internal/notification", theApp.InternalNotificationSettingPostHandler).Methods("POST")
	theApp.Router.HandleFunc("/kmn-internal/logout", theApp.InternalLogoutHandler).Methods("POST")

	theApp.Router.NotFoundHandler = http.HandlerFunc(theApp.NotFoundHandler)

	fileserver := http.FileServer(neuteredFileSystem{http.Dir("static")})
	theApp.Router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileserver))

	// Template caching: parse templates here instead in request to avoid delay
	theApp.TemplateHome = template.Must(template.ParseFiles("template/index.html", "template/_header.html"))
	theApp.TemplateDisplay = template.Must(template.New("queue.html").Funcs(fns).ParseFiles("template/queue.html", "template/_header.html", "template/_footer.html"))
	theApp.TemplateError = template.Must(template.ParseFiles("template/error.html", "template/_header.html"))

	theApp.TemplateLogin = template.Must(template.ParseFiles("template/login.html"))
	theApp.TemplateEditNotification = template.Must(template.ParseFiles("template/editnotification.html"))

	// Initialize notification database
	theApp.notificationViper = viper.New()
	theApp.notificationViper.SetConfigFile(notificationConfig)
	theApp.notificationPolicy = bluemonday.UGCPolicy()
}

func (theApp *App) Run(addr string) {
	server := &http.Server{
		Handler: theApp.Router,
		Addr:    ":" + addr,
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

func (theApp *App) GetNotification(branchCode string, roomCode string) (string, string) {
	theApp.notificationViper.ReadInConfig()
	var key string

	key = fmt.Sprintf("%s.branch", branchCode)
	branch := theApp.notificationViper.GetString(key)

	key = fmt.Sprintf("%s.%s", branchCode, roomCode)
	room := theApp.notificationViper.GetString(key)

	return branch, room
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
		"Processes": ProcessLibArr,
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
			theApp.NoDataTemplateDisplay(w, r, fullId, process)
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

	// If logs were not empty, but they are all OPR sequence, then result array would be nil.
	// Trying to modify the active with below method would crash
	if len(roomDisplay) == 0 {
		theApp.NoDataTemplateDisplay(w, r, fullId, process)
		return
	}

	// Get notification
	branchNotification, roomNotification := theApp.GetNotification(branch, r.FormValue("qinput1"))

	payload := map[string]interface{}{
		"Branch":             branchString,
		"Id":                 fullId,
		"Rooms":              roomDisplay,
		"BranchNotification": branchNotification,
		"RoomNotification":   roomNotification,
	}

	// Render output
	if err := theApp.TemplateDisplay.Execute(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (theApp *App) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)

	message := fmt.Sprintf("Halaman tidak ditemukan")

	if err := theApp.TemplateError.Execute(w, message); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (theApp *App) NoDataTemplateDisplay(w http.ResponseWriter, r *http.Request, id, process string) {
	w.WriteHeader(http.StatusOK) // for clarity

	processName, _ := ProcessLibMap[process]

	message := fmt.Sprintf("Data pasien %s untuk %s tidak tersedia", id, processName)

	if err := theApp.TemplateError.Execute(w, message); err != nil {
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

	// If logs were not empty, but they are all OPR sequence, then result array would be nil.
	// Trying to modify the active with below method would crash
	if len(roomDisplays) > 0 {
		// Set last room as active room
		roomDisplays[len(roomDisplays)-1].IsActive = true
	}

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
		if code, exist := theApp.OrderedRoom[processCode][i]; exist {
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

//========================================================================//
// ** Internal Pages Implementation **//
type Credential struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type RoomNotification struct {
	Code         string
	Name         string
	Notification string
}

var (
	creds             = []Credential{}
	loggedUserSession = sessions.NewCookieStore([]byte("super-secret-key")) // [TODO] change

	notificationConfig = "./notification.json"
)

func (theApp *App) InternalLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// With .Get() method, if not found, it created a new session immediately. So it's never nil
		session, _ := loggedUserSession.Get(r, "authenticated-user-session")

		// validate cookie session! [TODO] check hash.. need to store the hash then

		if session.IsNew {
			// Either no session or session exist but can't be decoded. gorilla.sessions create a new one
			err := session.Save(r, w) // save the session
			if err != nil {
				fmt.Println(err)
			} else {
				// Serve login page
				if err := theApp.TemplateLogin.Execute(w, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		} else {
			if theApp.CheckRequestSession(session) {
				// Redirect to notification page immediately
				url := r.URL.Path + "/notification"
				http.Redirect(w, r, url, http.StatusSeeOther)
			} else {
				// Serve login page
				if err := theApp.TemplateLogin.Execute(w, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		}
	} else if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Evaluate login input
		username := r.FormValue("username")
		password := r.FormValue("password")

		auth := false
		for _, cred := range creds {
			if cred.Username == username {
				err := bcrypt.CompareHashAndPassword([]byte(cred.Password), []byte(password))
				if err != nil { // user found but password doesn't match
					http.Error(w, "User and password combination doesn't match any records", http.StatusUnauthorized)
					return
				}

				auth = true
				break
			}
		}
		if !auth { // user not found
			http.Error(w, "User and password combination doesnt match any records", http.StatusForbidden)
			return
		}

		// Success
		// Store session
		session, _ := loggedUserSession.New(r, "authenticated-user-session")
		session.Options = &sessions.Options{
			MaxAge: 60 * 30, // 30 minute
		}
		session.Values["username"] = username
		session.Values["authenticated"] = true
		session.Values["changes-saved"] = false
		err := session.Save(r, w)
		if err != nil {
			fmt.Println(err)
		}

		// Redirect to notification page
		url := r.URL.Path + "/notification"
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}

func (theApp *App) InternalLogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := loggedUserSession.Get(r, "authenticated-user-session")
	session.Options.MaxAge = -1
	session.Save(r, w)

	http.Redirect(w, r, "/kmn-internal", http.StatusSeeOther)
}

// Check if session is authenticated
func (theApp *App) CheckRequestSession(session *sessions.Session) bool {
	auth, ok := session.Values["authenticated"]
	if !session.IsNew && ok && auth.(bool) {
		return true
	} else {
		return false
	}
}

func (theApp *App) InternalNotificationSettingGetHandler(w http.ResponseWriter, r *http.Request) {
	// Reject unauthenticated access
	session, _ := loggedUserSession.Get(r, "authenticated-user-session")
	if !theApp.CheckRequestSession(session) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Personalize page according to cookies data (username), and also notification config for latest value
	// 1. Translate username, which is branch code, into branch name
	branchCode := fmt.Sprintf("%v", session.Values["username"])
	branchName := ""
	for _, branchData := range theApp.Branches {
		if branchData.Code == branchCode {
			branchName = branchData.Name
		}
	}
	// 2. Get rooms in config.json and prepare controls for each one
	oprRoomCount := len(theApp.RoomMap["opr"])
	polRoomCount := len(theApp.RoomMap["pol"])

	oprRooms := make([]RoomNotification, 0)
	for i := 0; i < oprRoomCount; i++ {
		if code, exist := theApp.OrderedRoom["opr"][i]; exist {
			roomData := theApp.RoomMap["opr"][code]

			oprRooms = append(oprRooms, RoomNotification{
				Name:         roomData.Name,
				Code:         roomData.Code,
				Notification: "",
			})
		}
	}
	polRooms := make([]RoomNotification, 0)
	for i := 0; i < polRoomCount; i++ {
		if code, exist := theApp.OrderedRoom["pol"][i]; exist {
			roomData := theApp.RoomMap["pol"][code]

			polRooms = append(polRooms, RoomNotification{
				Name:         roomData.Name,
				Code:         roomData.Code,
				Notification: "",
			})
		}
	}

	// 3. Read existing notification text from config file
	theApp.notificationViper.ReadInConfig()
	notification := make(map[string]string)
	theApp.notificationViper.UnmarshalKey(branchCode, &notification)
	// Branch
	branchNotification := ""
	if text, exist := notification["branch"]; exist {
		branchNotification = text
	}
	// Opr Rooms
	for i := 0; i < oprRoomCount; i++ {
		// Because branch code entry are capital, but json key is always lowercase
		code := strings.ToLower(oprRooms[i].Code)

		if text, exist := notification[code]; exist {
			oprRooms[i].Notification = text
		}
	}
	// Pol Rooms
	for i := 0; i < polRoomCount; i++ {
		// Because branch code entry are capital, but json key is always lowercase
		code := strings.ToLower(polRooms[i].Code)

		if text, exist := notification[code]; exist {
			polRooms[i].Notification = text
		}
	}

	payload := map[string]interface{}{
		"Branch":             branchName,
		"BranchNotification": branchNotification,
		"OprRooms":           oprRooms,
		"PolRooms":           polRooms,
		"ChangesSaved":       session.Values["changes-saved"],
	}

	// Reset changes-saved flag in cookie
	session.Values["changes-saved"] = false
	session.Save(r, w)

	if err := theApp.TemplateEditNotification.Execute(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (theApp *App) SanitizeNotificationInput(text string) string {
	// Strip malicious html markup
	cleanHTML := theApp.notificationPolicy.Sanitize(text)
	// Escape all html markup
	noMarkUpHTML := html.EscapeString(cleanHTML)

	return noMarkUpHTML
}

func (theApp *App) InternalNotificationSettingPostHandler(w http.ResponseWriter, r *http.Request) {
	// Reject unauthenticated access
	session, _ := loggedUserSession.Get(r, "authenticated-user-session")
	if !theApp.CheckRequestSession(session) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	branchCode := fmt.Sprintf("%v", session.Values["username"])

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var key string // used for storing input value into config

	branchNotificationRaw := r.FormValue("branch")
	key = fmt.Sprintf("%s.branch", branchCode)
	// Sanitize input
	branchNotification := theApp.notificationPolicy.Sanitize(branchNotificationRaw)
	theApp.notificationViper.Set(key, branchNotification)

	// Because we create control based on RoomMap, then we can assume the control exist with our defined ID (room code)
	oprRoomCount := len(theApp.RoomMap["opr"])
	polRoomCount := len(theApp.RoomMap["pol"])

	// Get and sanitize all input
	for i := 0; i < oprRoomCount; i++ {
		if code, exist := theApp.OrderedRoom["opr"][i]; exist {
			roomData := theApp.RoomMap["opr"][code]

			// Sanitize input
			notification := theApp.SanitizeNotificationInput(r.FormValue(roomData.Code))

			// Set config value in memory with Viper
			key = fmt.Sprintf("%s.%s", branchCode, roomData.Code)
			theApp.notificationViper.Set(key, notification)
		}
	}
	for i := 0; i < polRoomCount; i++ {
		if code, exist := theApp.OrderedRoom["pol"][i]; exist {
			roomData := theApp.RoomMap["pol"][code]

			// Sanitize input
			notification := theApp.SanitizeNotificationInput(r.FormValue(roomData.Code))

			// Set config value in memory with Viper
			key = fmt.Sprintf("%s.%s", branchCode, roomData.Code)
			theApp.notificationViper.Set(key, notification)
		}
	}

	// Overwrite config file
	theApp.notificationViper.WriteConfig()

	// Redirect back to notification page
	session.Values["changes-saved"] = true
	session.Save(r, w)
	http.Redirect(w, r, "/kmn-internal/notification", http.StatusSeeOther)
}
