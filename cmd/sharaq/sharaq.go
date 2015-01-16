package main

import (
	"flag"
	"log"
	"os"

	"github.com/Peatix/sharaq"
)

func main() {
	os.Exit(_main())
}

func _main() int {
	var cfgfile string
	flag.StringVar(&cfgfile, "config", "etc/sharaq.json", "config file")
	flag.Parse()

	config := &sharaq.Config{}
	log.Printf("Using config file %s", cfgfile)
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