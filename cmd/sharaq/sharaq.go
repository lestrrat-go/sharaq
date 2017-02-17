// +build !appengine

package main

import (
	"context"
	"flag"
	"os"

	"github.com/lestrrat/go-config/env"
	"github.com/lestrrat/sharaq"
	"github.com/lestrrat/sharaq/internal/log"
	"github.com/pkg/errors"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := _main(ctx); err != nil {
		log.Infof(ctx, "%s", err)
		os.Exit(1)
	}
}

func _main(ctx context.Context) error {
	cfgfile := flag.String("config", "", "config file (e.g. sharaq.json)")
	showVersion := flag.Bool("version", false, "show sharaq version")
	flag.Parse()

	if *showVersion {
		os.Stdout.WriteString("sharaq version " + sharaq.Version + "\n")
		return nil
	}

	var config sharaq.Config
	if cfgfile == nil || len(*cfgfile) == 0 {
		if err := env.NewDecoder(env.System).Prefix("SHARAQ").Decode(&config); err != nil {
			return errors.Wrap(err, "error while reading config from environment")
		}
	} else {
		log.Debugf(ctx, "Using config file %s", *cfgfile)
		if err := config.ParseFile(*cfgfile); err != nil {
			return errors.Wrapf(err, "error while to parsing '%s'", *cfgfile)
		}
	}

	s, err := sharaq.NewServer(&config)
	if err != nil {
		return errors.Wrap(err, "error while creating server")
	}

	if err := s.Run(ctx); err != nil {
		return errors.Wrap(err, "error while running server")
	}

	return nil
}
