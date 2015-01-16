package main

import (
	"log"
	"os"

	"github.com/Peatix/sharaq"
)

func main() {
	os.Exit(_main())
}

func _main() int {
	cfgfile := "etc/sharaq.json"
	config := &sharaq.Config{}
	if err := config.ParseFile(cfgfile); err != nil {
		log.Printf("Failed to parse '%s': %s", cfgfile, err)
		return 1
	}

	s := sharaq.NewServer(config)
	if err := s.Run(); err != nil {
		log.Printf("Failed to run server: %s", err)
		return 1
	}

	return 0
}