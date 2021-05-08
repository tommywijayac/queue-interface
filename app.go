package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"regexp"

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

	DB           []*sql.DB
	Branches     []BranchData
	Rooms        []RoomData
	limitRooms   int
	visibleRooms int
	footer       string
}

type BranchData struct {
	Name         string `mapstructure:"name"`
	Code         string `mapstructure:"code"`
	DatabaseAddr string `mapstructure:"db-addr"`
	DatabaseUser string `mapstructure:"db-user"`
	DatabasePswd string `mapstructure:"db-pswd"`
	DatabaseName string `mapstructure:"db-name"`
}

type RoomData struct {
	IsActive bool
	Name     string `mapstructure:"name"`
	Code     string `mapstructure:"code"`
	Time     string
}

func (theApp *App) Initialize(user, password, dbname string) {
	var err error

	// Read configuration file
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.SetConfigType("json")
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

	// Connect to database
	theApp.DB = make([]*sql.DB, len(theApp.Branches))
	for i, branch := range theApp.Branches {
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

	viper.SetConfigName("runningtext")
	err = viper.ReadInConfig()
	if err == nil {
		theApp.footer = viper.GetString("text")
	} else {
		theApp.footer = ""
	}

	// Initialize routes
	theApp.Router = mux.NewRouter()
	theApp.Router.HandleFunc("/", theApp.HomeHandler).Methods("GET")
	theApp.Router.HandleFunc("/", theApp.HomePostHandler).Methods("POST")
	theApp.Router.HandleFunc("/{branch}", theApp.SearchHandler).Methods("GET")
	theApp.Router.HandleFunc("/{branch}", theApp.SearchPostHandler).Methods("POST")
	theApp.Router.HandleFunc("/{branch}/{id}", theApp.DisplayQueueHandler).Methods("GET")
	theApp.Router.NotFoundHandler = http.HandlerFunc(theApp.NotFoundHandler)

	theApp.Router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Parse templates here instead in request to avoid delay
	theApp.TemplateHome = template.Must(template.ParseFiles("template/index.html", "template/_header.html"))
	theApp.TemplateSearch = template.Must(template.ParseFiles("template/search.html", "template/_header.html"))
	theApp.TemplateDisplay = template.Must(template.ParseFiles("template/queue.html", "template/_header.html"))
	theApp.TemplateError = template.Must(template.ParseFiles("template/error.html", "template/_header.html"))
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
	expNoPrefixZeroes, err := regexp.Compile("^0+(?!$)")
	if err != nil {
		return id, err
	}
	idClean := expNoPrefixZeroes.ReplaceAllString(id, "")
	fmt.Println(idClean)

	return idClean, nil
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
		"Text":     theApp.footer,
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
	branchString, _ := theApp.GetBranchInfo(vars["branch"])
	if branchString == "" {
		theApp.NotFoundHandler(w, r)
		return
	}

	payload := Queue{
		Branch: branchString,
		Id:     "",
		Logs:   nil,
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
	// [TODO] basic validation should be done in front end with JS? like 4 digit must be full
	// 		  first digit must be alphabet and 3 others must be number
	fullId := r.FormValue("qinput1") + r.FormValue("qinput2") + r.FormValue("qinput3") + r.FormValue("qinput4")
	fmt.Println(fullId)

	// Validate queue number
	validQueueExp := regexp.MustCompile("^[a-zA-Z]{1}[0-9]{3}$")
	valid := validQueueExp.MatchString(fullId)
	if !valid {
		http.Error(w, "ERR: Invalid Queue number", http.StatusInternalServerError)
		return
	}

	// [TODO] locked to http:// ?
	userURL := fmt.Sprintf("http://%s/%s/%s", r.Host, r.URL.Path, string(fullId[1:]))
	fmt.Println(userURL)
	http.Redirect(w, r, userURL, http.StatusSeeOther)
}

func (theApp *App) DisplayQueueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// Translate branch code to name and id
	branchString, branchID := theApp.GetBranchInfo(vars["branch"])
	if branchString == "" || branchID == -1 {
		theApp.NotFoundHandler(w, r)
		return
	}

	// Get id from URL. Remove any leading zeroes
	idClean, err := SanitizeID(vars["id"])
	//fmt.Println(idClean)

	// [TODO] update database query based on actual database design
	room_id, logs, err := GetQueueLogs(theApp.DB[branchID], idClean)
	fmt.Println(logs)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			http.Error(w, "200 Data not found", http.StatusInternalServerError)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// [TODO] Match logs with displayed data
	// [TODO] Inactive logs must be greyed out
	// Assignment must access direct object, for-range makes a copy
	for i := 0; i < len(theApp.Rooms); i++ {
		theApp.Rooms[i].Time = "pk -"
		if i == len(theApp.Rooms)-2 {
			theApp.Rooms[i].IsActive = true
		} else {
			theApp.Rooms[i].IsActive = false
		}
	}

	payload := map[string]interface{}{
		"Branch": branchString,
		"Id":     room_id,
		"Rooms":  theApp.Rooms,
		"Text":   theApp.footer,
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
