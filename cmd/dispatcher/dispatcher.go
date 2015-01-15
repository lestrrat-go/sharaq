package main

import (
	"net/http"
	"os"

	"github.com/Peatix/sharaq"
)

func main() {
	os.Exit(_main())
}

func _main() int {
	d, err := sharaq.NewDispatcher()
	if err != nil {
		return 1
	}

	http.ListenAndServe(":9191", d)

	return 0
}