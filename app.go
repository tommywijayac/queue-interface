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

	/* [TODO] either multiple DB or multiple tables*/
	DB *sql.DB

	TemplateHome    *template.Template
	TemplateSearch  *template.Template
	TemplateDisplay *template.Template
	Branches        []BranchData
	Rooms           []RoomData
	limitRooms      int
	visibleRooms    int
}

type BranchData struct {
	Name string `mapstructure:"name"`
	Code string `mapstructure:"code"`
}

type RoomData struct {
	Name string `mapstructure:"name"`
	Code string `mapstructure:"code"`
	Time string
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

	// [TODO] database get from config file or environment vars
	// Connect to database
	connectionString := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s", user, password, dbname)
	theApp.DB, err = sql.Open("mysql", connectionString)
	if err != nil {
		panic(err.Error())
	}

	// Initialize routes
	theApp.Router = mux.NewRouter()
	theApp.Router.HandleFunc("/", theApp.HomeHandler).Methods("GET")
	theApp.Router.HandleFunc("/", theApp.HomePostHandler).Methods("POST")
	theApp.Router.HandleFunc("/{branch}", theApp.SearchHandler).Methods("GET")
	theApp.Router.HandleFunc("/{branch}", theApp.SearchPostHandler).Methods("POST")
	// [TODO]: handle this
	theApp.Router.HandleFunc("/{branch}/{id}", theApp.DisplayQueueHandler).Methods("GET")

	theApp.Router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Parse templates here instead in request to avoid delay
	theApp.TemplateHome = template.Must(template.ParseFiles("template/index.html", "template/_header.html"))
	theApp.TemplateSearch = template.Must(template.ParseFiles("template/search.html", "template/_header.html"))
	theApp.TemplateDisplay = template.Must(template.ParseFiles("template/queue.html", "template/_header.html"))
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

func (theApp *App) GetBranchString(branchCode string) string {
	// Match URL path {branch} with config file
	branch := ""
	for i := 0; i < len(theApp.Branches); i++ {
		if theApp.Branches[i].Code == branchCode {
			branch = theApp.Branches[i].Name
		}
	}
	return branch
}

func (theApp *App) HomeHandler(w http.ResponseWriter, r *http.Request) {
	viper.SetConfigName("runningtext")
	viper.AddConfigPath(".")
	viper.SetConfigType("json")
	err := viper.ReadInConfig()
	var txt string
	if err == nil {
		txt = viper.GetString("text")
	} else {
		txt = ""
	}

	payload := map[string]interface{}{
		"Branches": theApp.Branches,
		"Text":     txt,
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
	branchString := theApp.GetBranchString(vars["branch"])
	if branchString == "" {
		http.Error(w, "404 Page not found", http.StatusNotFound)
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

	// Get branch from URL
	branchString := theApp.GetBranchString(vars["branch"])
	if branchString == "" {
		http.Error(w, "404 Page not found", http.StatusNotFound)
		return
	}
	fmt.Println(branchString)

	// Get id from URL. Remove any leading zeroes
	idClean, err := SanitizeID(vars["id"])
	fmt.Println(idClean)

	// [TODO] update database query based on actual database design
	room_id, logs, err := GetQueueLogs(theApp.DB, idClean)
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
	for i := 0; i < len(theApp.Rooms); i++ {
		theApp.Rooms[i].Time = "pk -"
	}

	payload := map[string]interface{}{
		"Branch": branchString,
		"Id":     room_id,
		"Rooms":  theApp.Rooms,
	}

	// Render output
	if err := theApp.TemplateDisplay.Execute(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}
