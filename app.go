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

type App struct {
	Router *mux.Router
	DB     *sql.DB

	TemplateHome    *template.Template
	TemplateDisplay *template.Template
	BranchNames     []BranchData
}

type BranchData struct {
	Name string `mapstructure:"name"`
	Code string `mapstructure:"code"`
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
			panic(fmt.Errorf("ER001: Fatal error config not found"))
		} else {
			panic(fmt.Errorf("ER002: Fatal error config file: %s \n", err.Error()))
		}
	}

	fmt.Println("Config has been read")
	err = viper.UnmarshalKey("branch", &theApp.BranchNames)
	if err != nil {
		panic(fmt.Errorf("ER003: Fatal error while reading config file: %s \n", err.Error()))
	}

	// Connect to database
	connectionString := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s", user, password, dbname)
	theApp.DB, err = sql.Open("mysql", connectionString)
	if err != nil {
		panic(err.Error())
	}

	theApp.Router = mux.NewRouter()
	// Initialize routes
	theApp.Router.HandleFunc("/", theApp.HomeHandler).Methods("GET")
	theApp.Router.HandleFunc("/", theApp.HomePostHandler).Methods("POST")
	theApp.Router.HandleFunc("/{id:[0-9]+}", theApp.DisplayQueueHandler).Methods("GET") // Sanitize input: valid ID is a 4 digit number
	theApp.Router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	// Parse templates here instead in request to avoid delay
	theApp.TemplateHome = template.Must(template.ParseFiles("template/index.html", "template/_header.html"))
	//theApp.TemplateDisplay = template.Must(template.ParseFiles("template/queue.html", "template/_header.html"))
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

func (theApp *App) HomeHandler(w http.ResponseWriter, r *http.Request) {
	if err := theApp.TemplateHome.Execute(w, theApp.BranchNames); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (theApp *App) HomePostHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if err = r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// [TODO] sanitize ID
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println(id)

	// [TODO] how to allow 0001, preceeding 0? or just make 1
	// Construct user details URL
	userURL := fmt.Sprintf("http://%s/%d", r.Host, id)
	fmt.Println(userURL)
	http.Redirect(w, r, userURL, http.StatusSeeOther)
}

func (theApp *App) DisplayQueueHandler(w http.ResponseWriter, r *http.Request) {
	// Get sanitized ID from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// [TODO] Sanitize input

	// [TODO] Remove any leading zeroes
	idClean, err := SanitizeID(id)

	room_id, logs, err := GetQueueLogs(theApp.DB, idClean)
	fmt.Println(logs)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			http.Error(w, "Product not found", http.StatusInternalServerError)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var q = Queue{
		Date:      time.Now().Format("02-01-2006"),
		Id:        room_id,
		Highlight: logs[0],
		Logs:      logs[1:],
	}

	// Render output
	if err := theApp.TemplateDisplay.Execute(w, q); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}
