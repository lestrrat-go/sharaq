// +build !appengine

package main

import (
	"context"
	"flag"
	"os"

	"github.com/lestrrat-go/sharaq"
	"github.com/lestrrat-go/sharaq/internal/log"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var config sharaq.Config
	log.Debugf(ctx, "Using config file %s", *cfgfile)
	if err := config.ParseFile(*cfgfile); err != nil {
		log.Debugf(ctx, "Failed to parse '%s': %s", *cfgfile, err)
		return 1
	}

	s, err := sharaq.NewServer(&config)
	if err != nil {
		log.Debugf(ctx, "Failed to instantiate server: %s", err)
		return 1
	}

	if err := s.Run(ctx); err != nil {
		log.Debugf(ctx, "Failed to run server: %s", err)
		return 1
	}

	return 0
}
