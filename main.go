package main

import (
	"fmt"
	"os"
)

func init() {
	fmt.Println("Hello from main init!")
}

func main() {
	theApp := App{}
	theApp.ReadConfig()
	theApp.Initialize()

	port := os.Getenv("PORT")
	if port == "" {
		fmt.Println("Port set to default")
		port = "8080"
	}
	theApp.Run(port)
}
