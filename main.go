package main

func main() {
	theApp := App{}
	theApp.Initialize()
	theApp.Run(":8081")
	return
}
