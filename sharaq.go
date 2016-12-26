package sharaq

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"regexp"

	"github.com/lestrrat/sharaq/aws"
	"github.com/lestrrat/sharaq/gcp"
	"github.com/pkg/errors"
)

func NewServer(c *Config) (*Server, error) {
	s := &Server{
		config: c,
	}

	whitelist := make([]*regexp.Regexp, len(c.Whitelist))
	for i, pat := range c.Whitelist {
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, err
		}
		whitelist[i] = re
	}
	if c.Debug {
		s.dumpConfig()
	}
	return s, nil
}

func (s *Server) dumpConfig() {
	j, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(j))
	for scanner.Scan() {
		l := scanner.Text()
		log.Print(l)
	}
}

func (s *Server) newBackend() error {
	switch s.config.Backend.Type {
	case "aws":
		b, err := aws.NewBackend(
			&s.config.Backend.Amazon,
			s.cache,
			s.transformer,
			s.config.Presets,
		)
		if err != nil {
			return errors.Wrap(err, `failed to create aws backend`)
		}
		s.backend = b
	case "gcp":
		b, err := gcp.NewBackend(
			&s.config.Backend.Google,
			s.cache,
			s.transformer,
			s.config.Presets,
		)
		if err != nil {
			return errors.Wrap(err, `failed to create gcp backend`)
		}
		s.backend = b
	default:
		return errors.Errorf(`invalid storage backend %s`, s.config.Backend.Type)
	}
	return nil
}
