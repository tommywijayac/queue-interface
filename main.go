package main

import (
	"fmt"
	"os"
)

func main() {
	theApp := App{}
	theApp.ReadConfig()
	theApp.ReadRegisterdUserConfig()
	theApp.Initialize()

	port := os.Getenv("PORT")
	if port == "" {
		fmt.Println("Port set to default")
		port = "8080"
	}
	theApp.Run(port)
}
