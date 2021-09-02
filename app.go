package main

import (
	"database/sql"
	"encoding/json"
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

	ValidQueueCodeList = []string{
		"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
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

var (
	Router *mux.Router

	TemplateHome             *template.Template
	TemplateSearch           *template.Template
	TemplateDisplay          *template.Template
	TemplateError            *template.Template
	TemplateLogin            *template.Template
	TemplateEditNotification *template.Template

	DB *sql.DB

	notificationViper  *viper.Viper
	notificationPolicy *bluemonday.Policy
)

type RoomDisplay struct {
	IsActive bool
	Name     string
	Time     string
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

func Initialize() {
	var err error

	connectionString := fmt.Sprintf("%s:%s@tcp(%s)/%s",
		AppConfig.DatabaseUser,
		AppConfig.DatabasePswd,
		AppConfig.DatabaseAddr,
		AppConfig.DatabaseName)
	DB, err = sql.Open("mysql", connectionString)
	if err != nil {
		ErrorLogger.Fatalf("fail to open sql connection. %v", err)
	}

	// Initialize routes
	Router = mux.NewRouter()
	Router.HandleFunc("/", HomeHandler).Methods("GET")
	Router.HandleFunc("/search", DisplayQueueHandler).Methods("GET")
	Router.HandleFunc("/kmn-internal", InternalLoginHandler).Methods("GET", "POST")
	Router.HandleFunc("/kmn-internal/notification", InternalNotificationSettingGetHandler).Methods("GET")
	Router.HandleFunc("/kmn-internal/notification", InternalNotificationSettingPostHandler).Methods("POST")
	Router.HandleFunc("/kmn-internal/logout", InternalLogoutHandler).Methods("POST")

	Router.NotFoundHandler = http.HandlerFunc(NotFoundHandler)

	fileserver := http.FileServer(neuteredFileSystem{http.Dir("static")})
	Router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileserver))

	// Template caching: parse templates here instead in request to avoid delay
	TemplateHome = template.Must(template.ParseFiles("template/index.html", "template/_header.html"))
	TemplateDisplay = template.Must(template.New("queue.html").Funcs(fns).ParseFiles("template/queue.html", "template/_header.html", "template/_footer.html"))
	TemplateError = template.Must(template.ParseFiles("template/error.html", "template/_header.html"))

	TemplateLogin = template.Must(template.ParseFiles("template/login.html"))
	TemplateEditNotification = template.Must(template.ParseFiles("template/editnotification.html"))

	// Initialize notification database
	notificationViper = viper.New()
	notificationViper.SetConfigFile(notificationConfig)
	notificationPolicy = bluemonday.UGCPolicy()

	// Set global default value of cookie expiry duration
	loggedUserSession = sessions.NewCookieStore(AppConfig.PrimaryKey.Auth, AppConfig.PrimaryKey.Encrypt)
	loggedUserSession.MaxAge(60 * 30) // 30 minute
}

func Run(addr string) {
	server := &http.Server{
		Handler: Router,
		Addr:    ":" + addr,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	InfoLogger.Printf("app launched at localhost:%v\n", server.Addr)
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

func GetNotification(branchCode string, queueCode string) (string, string) {
	notificationViper.ReadInConfig()
	notifications := []Notification{}
	notificationViper.UnmarshalKey(branchCode, &notifications)

	var branch, room string = "", ""
	for _, n := range notifications {
		if n.Code == "branch" {
			branch = n.Text
		} else if n.Code == queueCode {
			room = n.Text
		}
	}

	return branch, room
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	var branchCopy []BranchData
	for _, branch := range AppConfig.Branches {
		branchCopy = append(branchCopy, BranchData{
			Name: branch.Name,
			Code: branch.Code,
		})
	}

	payload := map[string]interface{}{
		"Branches":  branchCopy,
		"Processes": ProcessLibArr,
	}
	if err := TemplateHome.Execute(w, payload); err != nil {
		ErrorLogger.Printf("fail to execute template for / endpoint. %v\n", err)
		http.Error(w, "halaman gagal dimuat. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
	}
}

func DisplayQueueHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		ErrorLogger.Printf("fail to parse input from / endpoint. %v\n", err)
		http.Error(w, "input gagal diproses. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
		return
	}

	// Validate and sanitize branch
	branch := r.FormValue("branch")
	// fmt.Println(branch)
	if valid := AppConfig.validateBranch(branch); !valid {
		ErrorLogger.Printf("invalid branch selection. got: %v", branch)
		http.Error(w, "input cabang tidak valid. silahkan coba lagi.", http.StatusBadRequest)
		// [TODO] redirect to index/search
		return
	}
	branchName, branchID := AppConfig.getBranchInfo(branch)

	// Validate and sanitize process
	process := r.FormValue("process")
	// fmt.Println(process)
	if valid := validateProcess(process); !valid {
		ErrorLogger.Printf("invalid process selection. got: %v", process)
		http.Error(w, "input proses tidak valid. silahkan coba lagi.", http.StatusBadRequest)
		// [TODO] redirect to index/search
		return
	}

	// Validate and sanitize queue number
	fullID := r.FormValue("qinput1") + r.FormValue("qinput2") + r.FormValue("qinput3") + r.FormValue("qinput4")
	fullID, _ = SanitizeID(fullID)
	if valid := validateID(fullID); !valid {
		ErrorLogger.Printf("invalid queue number. got: %v", fullID)
		http.Error(w, "input antrian tidak valid. silahkan coba lagi.", http.StatusBadRequest)
		// [TODO] redirect to index/search
		return
	}

	logs, err := GetQueueLogs(DB, branchID, fullID)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			InfoLogger.Printf("no room returned by sql query for %v in %v(%v)", fullID, branchID, branchName)
			NoDataTemplateDisplay(w, r, fullID, process)
		default:
			ErrorLogger.Printf("sql query failed. %v", err)
			http.Error(w, "input gagal diproses. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
		}
		return
	}

	// Arrange logs to room
	var roomDisplay []RoomDisplay = make([]RoomDisplay, 0)
	switch process {
	case "opr":
		InfoLogger.Printf("no room left after applying logic. proccess: %v", process)
		roomDisplay = ConstructRoomListBasedOnOrder(logs, process)
	case "pol":
		roomDisplay = ConstructRoomListBasedOnTime(logs, process)
	}

	// If logs were not empty, but they are all OPR sequence, then result array would be nil.
	if len(roomDisplay) == 0 {
		NoDataTemplateDisplay(w, r, fullID, process)
		return
	}

	// Get notification
	branchNotification, roomNotification := GetNotification(branch, r.FormValue("qinput1"))

	payload := map[string]interface{}{
		"Branch":             branchName,
		"Id":                 fullID,
		"Rooms":              roomDisplay,
		"LastUpdated":        time.Now().Format("2006-01-02 15:04:05"),
		"BranchNotification": branchNotification,
		"RoomNotification":   roomNotification,
	}

	// Render output
	if err := TemplateDisplay.Execute(w, payload); err != nil {
		ErrorLogger.Printf("fail to execute template for display. %v\n", err)
		http.Error(w, "halaman gagal dimuat. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
	}
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)

	message := fmt.Sprintf("Halaman tidak ditemukan")

	if err := TemplateError.Execute(w, message); err != nil {
		ErrorLogger.Printf("fail to execute template for error. %v\n", err)
		http.Error(w, "halaman gagal dimuat. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
	}
}

func NoDataTemplateDisplay(w http.ResponseWriter, r *http.Request, id, process string) {
	w.WriteHeader(http.StatusOK) // for clarity

	processName, _ := ProcessLibMap[process]

	message := fmt.Sprintf("Data pasien %s untuk %s tidak tersedia", id, processName)

	if err := TemplateError.Execute(w, message); err != nil {
		ErrorLogger.Printf("fail to execute template for error. %v\n", err)
		http.Error(w, "halaman gagal dimuat. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
	}
}

func ConstructRoomListBasedOnTime(logs []PatientLog, processCode string) []RoomDisplay {
	var roomDisplays []RoomDisplay

	// Sort PatientLog array based on time
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Time.Before(logs[j].Time)
	})

	// Translate Room code into Room name, and populate array result
	added := map[string]bool{}
	for _, log := range logs {
		// Standardize key: lowercase
		log.Group = strings.ToLower(log.Group)
		if room, valid := AppConfig.RoomMap[processCode][log.Group]; valid {
			// Display first log occurence data
			if exist := added[log.Group]; !exist {
				var rd = RoomDisplay{
					Name:     room.Name,
					Time:     log.Time.Format("15:04:05"),
					IsActive: false,
				}
				roomDisplays = append(roomDisplays, rd)

				added[log.Group] = true
			}
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

func ConstructRoomListBasedOnOrder(logs []PatientLog, processCode string) []RoomDisplay {
	defaultTimeTxt := "-"

	var roomDisplays []RoomDisplay = make([]RoomDisplay, 0)
	for _, room := range AppConfig.Rooms[processCode] {
		roomDisplays = append(roomDisplays, RoomDisplay{
			Name:     room.Name,
			Time:     defaultTimeTxt,
			IsActive: false,
		})
	}
	n := len(roomDisplays)

	// Sort PatientLog array based on time
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Time.Before(logs[j].Time)
	})

	// Iterate log and find matching room (NOT group!)
	latest := -1
	added := map[int]bool{}
	for _, log := range logs {
		// Standardize key: lowercase
		log.Room = strings.ToLower(log.Room)

		if room, valid := AppConfig.RoomMap[processCode][log.Room]; valid {
			// Prevent panicking due invalid index
			if room.Order < 0 && room.Order >= n {
				continue
			}

			// Display first log occurence data
			if exist := added[room.Order]; !exist {
				roomDisplays[room.Order].Time = log.Time.Format("15:04:05")

				added[room.Order] = true

				if room.Order > latest {
					latest = room.Order
				}
			}
		}
	}

	// Determine where the patient is (active room) based on last not 'nil' room in order (NOT time)
	// scenario: (x) A -> (.) B (turns out the nurse forget to scan at A, which then she did after scan on B)
	// in above case, if ordered by time, then A would be highlighted. should be B
	if latest != -1 {
		roomDisplays[latest].IsActive = true
	}

	return roomDisplays
}

//========================================================================//
// ** Internal Pages Implementation **//
var (
	loggedUserSession  *sessions.CookieStore
	notificationConfig = "./notification.json"
)

type Notification struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

func InternalLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// With .Get() method, if not found, it created a new session immediately. So it's never nil
		session, _ := loggedUserSession.Get(r, "authenticated-user-session")

		// validate cookie session! [TODO] check hash.. need to store the hash then

		if session.IsNew {
			// Either no session or session exist but can't be decoded. gorilla.sessions create a new one
			err := session.Save(r, w) // save the session
			if err != nil {
				ErrorLogger.Printf("fail to save kmn-internal session. %v\n", err)
				http.Error(w, "input gagal diproses. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
				return
			} else {
				// Serve login page
				if err := TemplateLogin.Execute(w, nil); err != nil {
					ErrorLogger.Printf("fail to execute template for login. %v\n", err)
					http.Error(w, "halaman gagal dimuat. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
					return
				}
			}
		} else {
			if CheckRequestSession(session) {
				// Redirect to notification page immediately
				url := r.URL.Path + "/notification"
				http.Redirect(w, r, url, http.StatusSeeOther)
			} else {
				// Serve login page
				if err := TemplateLogin.Execute(w, nil); err != nil {
					ErrorLogger.Printf("fail to execute template for login. %v\n", err)
					http.Error(w, "halaman gagal dimuat. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
					return
				}
			}
		}
	} else if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			ErrorLogger.Printf("fail to parse input from / endpoint. %v\n", err)
			http.Error(w, "input gagal diproses. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
			return
		}

		// Evaluate login input
		username := r.FormValue("username")
		password := r.FormValue("password")

		auth := false
		for _, branch := range AppConfig.Branches {
			if branch.Code == username {
				err := bcrypt.CompareHashAndPassword([]byte(branch.Password), []byte(password))
				if err != nil { // user found but password doesn't match
					InfoLogger.Printf("No matching user-password combination. Inputted User: %v. Password: %v", username, password)
					http.Error(w, "Kombinasi User and password tidak terdaftar.", http.StatusUnauthorized)
					return
				}

				auth = true
				break
			}
		}
		if !auth { // user not found
			InfoLogger.Printf("No matching user-password combination. Inputted User: %v. Password: %v", username, password)
			http.Error(w, "Kombinasi User and password tidak terdaftar.", http.StatusUnauthorized)
			return
		}

		// Success
		// Store session
		session, _ := loggedUserSession.New(r, "authenticated-user-session")
		session.Values["username"] = username
		session.Values["authenticated"] = true
		err := session.Save(r, w)
		if err != nil {
			ErrorLogger.Printf("fail to save kmn-internal session. %v\n", err)
			http.Error(w, "Fail to initialize session", http.StatusInternalServerError)
			return
		}

		// Redirect to notification page
		url := r.URL.Path + "/notification"
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}

func InternalLogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := loggedUserSession.Get(r, "authenticated-user-session")
	session.Options.MaxAge = -1
	session.Save(r, w)

	http.Redirect(w, r, "/kmn-internal", http.StatusSeeOther)
}

// Check if session is authenticated
func CheckRequestSession(session *sessions.Session) bool {
	auth, ok := session.Values["authenticated"]
	if !session.IsNew && ok && auth.(bool) {
		return true
	} else {
		return false
	}
}

func InternalNotificationSettingGetHandler(w http.ResponseWriter, r *http.Request) {
	// Reject unauthenticated access
	session, _ := loggedUserSession.Get(r, "authenticated-user-session")
	if !CheckRequestSession(session) {
		InfoLogger.Printf("unauthenticated access to kmn-internal page method GET. user: %v\n", session.Values["username"])
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Personalize page according to cookies data (username), and also notification config for latest value
	// 1. Translate username, which is branch code, into branch name
	branchCode := fmt.Sprintf("%v", session.Values["username"])
	branchName, _ := AppConfig.getBranchInfo(branchCode)

	// 3. Read existing notification text from config file
	notificationViper.ReadInConfig()

	notifications := []Notification{}

	notificationViper.UnmarshalKey(branchCode, &notifications)

	// code "branch" is always the first in array
	branchNotification := ""
	n := len(notifications)
	if n > 0 {
		if notifications[0].Code != "branch" {
			ErrorLogger.Printf("kmn-internal: branch in notification.json isn't first entry. structure: %v", notifications)
		} else {
			branchNotification = notifications[0].Text

			if n > 1 {
				notifications = notifications[1:]
			}
		}
	}

	payload := map[string]interface{}{
		"Branch":             branchName,
		"BranchNotification": branchNotification,
		"QueueNotification":  notifications,
		"ValidQueueCodeList": ValidQueueCodeList,
	}

	if err := TemplateEditNotification.Execute(w, payload); err != nil {
		ErrorLogger.Printf("fail to execute template for edit notification. %v\n", err)
		http.Error(w, "halaman gagal dimuat. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
	}
}

func sanitizeNotificationInput(text string) string {
	// Strip malicious html markup
	cleanHTML := notificationPolicy.Sanitize(text)
	// Escape all html markup
	noMarkUpHTML := html.EscapeString(cleanHTML)

	return noMarkUpHTML
}

func validateNotificationCode(code string) bool {
	validQueueExp := regexp.MustCompile(`^[A-Z]{1}$`)
	return validQueueExp.MatchString(code)
}

func InternalNotificationSettingPostHandler(w http.ResponseWriter, r *http.Request) {
	// Reject unauthenticated access
	session, _ := loggedUserSession.Get(r, "authenticated-user-session")
	if !CheckRequestSession(session) {
		InfoLogger.Printf("unauthenticated access to kmn-internal page method POST. user: %v\n", session.Values["username"])
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	branchCode := fmt.Sprintf("%v", session.Values["username"])

	// Read JSON payload
	notifications := []Notification{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&notifications); err != nil {
		ErrorLogger.Printf("kmn-internal: fail to decode edit-notification payload. %v", err)
		http.Error(w, "edit gagal disimpan. silahkan coba beberapa saat lagi.", http.StatusInternalServerError)
		return
	}

	// Construct new array with clean&valid Code and Text
	notificationsClean := []Notification{}
	for i := 0; i < len(notifications); i++ {
		if notifications[i].Code != "branch" && !validateNotificationCode(notifications[i].Code) {
			ErrorLogger.Printf("kmn-internal: dropping entry because invalid code. code: %v", notifications[i].Code)
			continue
		}

		notifications[i].Text = sanitizeNotificationInput(notifications[i].Text)
		notificationsClean = append(notificationsClean, Notification{
			Code: notifications[i].Code,
			Text: sanitizeNotificationInput(notifications[i].Text),
		})
	}

	// Assert array structure where "branch" is first element
	n := len(notificationsClean)
	if n > 0 && notificationsClean[0].Code != "branch" {
		InfoLogger.Printf("kmn-internal: branch isn't first entry. possible custom JSON payload or else. structure: %v", notificationsClean)
	}

	// Overwrite config file
	notificationViper.Set(branchCode, notifications)
	notificationViper.WriteConfig()

	// Send response
	response := map[string]bool{
		"success": true,
	}
	b, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		fmt.Println("Marshal err")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
