package main

func main() {
	theApp := App{}
	theApp.Initialize(
		"root",
		"",
		"kmn_queue",
	)

	theApp.Run(":8081")
	return
}
