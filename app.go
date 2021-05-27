package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

const MAX_ROOM int = 10

type App struct {
	Router *mux.Router

	TemplateHome    *template.Template
	TemplateSearch  *template.Template
	TemplateDisplay *template.Template
	TemplateError   *template.Template

	DB       []*sql.DB
	Branches []BranchData
	Rooms    []RoomData
}

type RoomData struct {
	Name string `mapstructure:"name"`
	Code string `mapstructure:"code"`
}

type RoomDisplay struct {
	IsActive bool
	Name     string
	Time     string
}

type BranchData struct {
	Name         string `mapstructure:"name"`
	Code         string `mapstructure:"code"`
	DatabaseAddr string `mapstructure:"db-addr"`
	DatabaseUser string `mapstructure:"db-user"`
	DatabasePswd string `mapstructure:"db-pswd"`
	DatabaseName string `mapstructure:"db-name"`
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

func (theApp *App) Initialize() {
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
	err = viper.UnmarshalKey("room", &theApp.Rooms)
	if err != nil {
		panic(fmt.Errorf("ER003: Fatal error - reading config file: %s \n", err.Error()))
	}
	// Limit the number of visible room regardless of config file
	roomCount := viper.GetInt("visible-room")
	if roomCount < 0 {
		roomCount = 0
	} else if roomCount > MAX_ROOM {
		roomCount = MAX_ROOM
	}
	// Prune the slice
	theApp.Rooms = theApp.Rooms[:roomCount]

	// Validate data
	if len(theApp.Rooms) == 0 {
		panic(fmt.Errorf("ER004: Fatal config error - no Queue to be displayed"))
	}

	theApp.DB = make([]*sql.DB, len(theApp.Branches))
	for i, branch := range theApp.Branches {
		// Connect to database
		connectionString := fmt.Sprintf("%s:%s@tcp(%s)/%s",
			branch.DatabaseUser,
			branch.DatabasePswd,
			branch.DatabaseAddr,
			branch.DatabaseName)

		fmt.Println(connectionString)

		theApp.DB[i], err = sql.Open("mysql", connectionString)
		if err != nil {
			panic("sql open err" + err.Error())
		}
	}

	// Initialize routes
	theApp.Router = mux.NewRouter()
	theApp.Router.HandleFunc("/", theApp.HomeHandler).Methods("GET")
	theApp.Router.HandleFunc("/", theApp.HomePostHandler).Methods("POST")
	theApp.Router.HandleFunc("/{branch}", theApp.SearchHandler).Methods("GET")
	theApp.Router.HandleFunc("/{branch}", theApp.SearchPostHandler).Methods("POST")
	theApp.Router.HandleFunc("/{branch}/{id}", theApp.DisplayQueueHandler).Methods("GET")
	theApp.Router.NotFoundHandler = http.HandlerFunc(theApp.NotFoundHandler)

	fileserver := http.FileServer(neuteredFileSystem{http.Dir("static")})
	theApp.Router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileserver))

	// Parse templates here instead in request to avoid delay
	theApp.TemplateHome = template.Must(template.ParseFiles("template/index.html", "template/_header.html", "template/_footer.html"))
	theApp.TemplateSearch = template.Must(template.ParseFiles("template/search.html", "template/_header.html", "template/_footer.html"))
	theApp.TemplateDisplay = template.Must(template.New("queue.html").Funcs(fns).ParseFiles("template/queue.html", "template/_header.html", "template/_footer.html"))
	theApp.TemplateError = template.Must(template.ParseFiles("template/error404.html", "template/_header.html", "template/_footer.html"))
}

func (theApp *App) Run(addr string) {
	// Start API
	server := &http.Server{
		Handler: theApp.Router,
		Addr:    addr,
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
	expNoPrefixZeroes, err := regexp.Compile(`^0+`)
	if err != nil {
		fmt.Printf("err cleaning ID! %s\n", err.Error())
		return "", err
	}
	cleanId := expNoPrefixZeroes.ReplaceAllString(id, "")
	fmt.Println(cleanId)

	return cleanId, nil
}

func ValidateID(id string) bool {
	// Validate queue number
	validQueueExp := regexp.MustCompile(`^[a-zA-Z]{1}[0-9]{3}$`)
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

func (theApp *App) HomeHandler(w http.ResponseWriter, r *http.Request) {
	var branchNames []string
	for _, branch := range theApp.Branches {
		branchNames = append(branchNames, branch.Name)
	}

	payload := map[string]interface{}{
		"Branches": branchNames,
		"Footer":   "",
	}
	if err := theApp.TemplateHome.Execute(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (theApp *App) HomePostHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if err = r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Validate form data, Branch ID
	branchID, err := strconv.Atoi(r.FormValue("branch"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if branchID < 0 || branchID > len(theApp.Branches) {
		http.Error(w, "Invalid payload", http.StatusInternalServerError)
	}

	// Translate Branch ID into Branch Code, then construct redirect link
	// [TODO] locked to http:// ?
	userURL := fmt.Sprintf("http://%s/%s", r.Host, theApp.Branches[branchID].Code)
	fmt.Println(userURL)
	http.Redirect(w, r, userURL, http.StatusSeeOther)
}

func (theApp *App) SearchHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	// Since we use variable URL, NotFoundHandler can't catch this
	branchCode := vars["branch"]
	branchString, _ := theApp.GetBranchInfo(branchCode)
	if branchString == "" {
		theApp.NotFoundHandler(w, r)
		return
	}

	payload := map[string]interface{}{
		"Branch": branchString,
		"Footer": GetFooterText(branchCode),
	}
	if err := theApp.TemplateSearch.Execute(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (theApp *App) SearchPostHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if err = r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fullId := r.FormValue("qinput1") + r.FormValue("qinput2") + r.FormValue("qinput3") + r.FormValue("qinput4")
	fmt.Println(fullId)

	// Validate queue number
	if valid := ValidateID(fullId); !valid {
		http.Error(w, "ERR: Invalid Queue number", http.StatusInternalServerError)
		return
	}

	// [TODO] locked to http:// ?
	userURL := fmt.Sprintf("http://%s%s/%s", r.Host, r.URL.Path, string(fullId))
	fmt.Println(userURL)
	http.Redirect(w, r, userURL, http.StatusSeeOther)
}

func (theApp *App) DisplayQueueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// Translate branch code to name and id
	branchCode := vars["branch"]
	branchString, branchID := theApp.GetBranchInfo(branchCode)
	if branchString == "" || branchID == -1 {
		theApp.NotFoundHandler(w, r)
		return
	}

	// Validate queue number
	idClean := vars["id"]
	if valid := ValidateID(idClean); !valid {
		http.Error(w, "ERR: Invalid Queue number", http.StatusInternalServerError)
		// [TODO] redirect to index/search
		return
	}

	// [TODO] update database query based on actual database design
	logs, err := GetQueueLogs(theApp.DB[branchID], idClean)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			http.Error(w, "200 Data not found", http.StatusInternalServerError)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	roomDisplay := theApp.AssignLogsToTemplate(logs)

	payload := map[string]interface{}{
		"Branch": branchString,
		"Id":     idClean,
		"Rooms":  roomDisplay,
		"Footer": GetFooterText(branchCode),
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

func (theApp *App) AssignLogsToTemplate(logs []QueueLog) []RoomDisplay {
	var roomDisplays []RoomDisplay

	for i, room := range theApp.Rooms {
		var rd = RoomDisplay{
			Name:     room.Name,
			Time:     "pk -",
			IsActive: false,
		}

		for _, log := range logs {
			if theApp.Rooms[i].Code == log.Room[:1] {
				rd.Time = "pk. " + log.Time
				rd.Name = fmt.Sprintf("%s %s", theApp.Rooms[i].Name, log.Room[1:])
				rd.IsActive = true
				if i > 0 {
					roomDisplays[i-1].IsActive = false
				}
				break
			}
		}

		roomDisplays = append(roomDisplays, rd)
	}

	return roomDisplays
}
