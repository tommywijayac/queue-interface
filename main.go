package main

import (
	"log"
	"os"
)

var (
	AppConfig Config

	// Tools
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
)

func main() {
	// Initialize logger
	file, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal("Fail to initialize logger!")
	}
	InfoLogger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	// Read config (static values)
	AppConfig.readConfig()

	// Initialize handler, database, and several other tools
	Initialize()

	// Starting the app
	Run(AppConfig.Port)
}
