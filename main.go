package main

func main() {
	theApp := App{}
	theApp.ReadConfig()
	theApp.Initialize()
	theApp.Run(":8080")
	return
}
