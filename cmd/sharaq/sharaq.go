package main

import (
	"flag"
	"log"
	"os"

	"github.com/Peatix/sharaq"
)

const version = "0.0.8"

func main() {
	os.Exit(_main())
}

func _main() int {
	cfgfile := flag.String("config", "sharaq.json", "config file")
	showVersion := flag.Bool("version", false, "show sharaq version")
	flag.Parse()

	if *showVersion {
		os.Stdout.WriteString("sharaq version " + version + "\n")
		return 0
	}

	config := &sharaq.Config{}
	log.Printf("Using config file %s", *cfgfile)
	if err := config.ParseFile(*cfgfile); err != nil {
		log.Printf("Failed to parse '%s': %s", *cfgfile, err)
		return 1
	}

	s := sharaq.NewServer(config)
	if err := s.Run(); err != nil {
		log.Printf("Failed to run server: %s", err)
		return 1
	}

	return 0
}
