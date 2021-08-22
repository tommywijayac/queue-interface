package main

import (
	"fmt"
)

var (
	AppConfig Config

	// Const
	MAX_ROOM = 10
)

func main() {
	// Read config (static values)
	AppConfig.readConfig()

	// Initialize handler, database, and several other tools
	Initialize()

	// Starting the app
	if AppConfig.Port == "" {
		fmt.Println("Port set to default")
		AppConfig.Port = "8080"
	}
	Run(AppConfig.Port)
}
