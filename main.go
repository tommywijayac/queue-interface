package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	theApp := App{}
	theApp.ReadConfig()
	theApp.Initialize()

	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
		return
	}

	port := os.Getenv("PORT")
	if port == "" {
		fmt.Println("Port set to default")
		port = "8080"
	}

	theApp.Run(port)
	return
}
