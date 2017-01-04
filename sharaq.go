package sharaq

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/lestrrat/sharaq/aws"
	"github.com/lestrrat/sharaq/fs"
	"github.com/lestrrat/sharaq/gcp"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/lestrrat/sharaq/internal/util"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func NewServer(c *Config) (*Server, error) {
	// Just so that we don't barf...
	if c == nil {
		c = &Config{}
	}

	s := &Server{
		config: c,
	}

	if len(c.Tokens) > 0 {
		s.tokens = make(map[string]struct{})
		for _, tok := range c.Tokens {
			// Don't allow empty tokens
			tok = strings.TrimSpace(tok)
			if len(tok) > 0 {
				s.tokens[tok] = struct{}{}
			}
		}
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
	case "fs":
		b, err := fs.NewBackend(
			&s.config.Backend.FileSystem,
			s.cache,
			s.transformer,
			s.config.Presets,
		)
		if err != nil {
			return errors.Wrap(err, `failed to create file system backend`)
		}
		s.backend = b
	default:
		return errors.Errorf(`invalid storage backend %s`, s.config.Backend.Type)
	}
	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		s.handleFetch(w, r)
	case "PUT":
		s.handleStore(w, r)
	case "DELETE":
		s.handleDelete(w, r)
	default:
		http.Error(w, "What, what, what?", http.StatusBadRequest)
	}
}

// handleFetch replies with the proper URL of the image
func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request) {
	u, err := util.GetTargetURL(r)
	if err != nil {
		http.Error(w, `url parameter missing`, http.StatusBadRequest)
		return
	}

	allowed := false
	if len(s.whitelist) == 0 {
		allowed = true
	} else {
		for _, pat := range s.whitelist {
			if pat.MatchString(u.String()) {
				allowed = true
				break
			}
		}
	}

	if !allowed {
		http.Error(w, "Specified url not allowed", 403)
		return
	}

	s.backend.Serve(w, r)
}

func (s *Server) markProcessing(ctx context.Context, u *url.URL) error {
	cacheKey := urlcache.MakeCacheKey("processing", u.String())
	return errors.Wrap(
		s.cache.SetNX(ctx, cacheKey, "XXX", urlcache.WithExpires(5*time.Second)),
		`failed to set cache`,
	)
}

func (s *Server) unmarkProcessing(ctx context.Context, u *url.URL) error {
	cacheKey := urlcache.MakeCacheKey("processing", u.String())
	return errors.Wrap(
		s.cache.Delete(ctx, cacheKey),
		`failed to delete cache`,
	)
}

// handleStore accepts PUT requests to create resized images and
// store them in the backend. This only exists so that you may perform
// repairs for existing images: normally the GET method automatically
// fetches and creates the resized images
func (s *Server) handleStore(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, `not authorized`, http.StatusForbidden)
		return
	}

	u, err := util.GetTargetURL(r)
	if err != nil {
		http.Error(w, `url parameter missing`, http.StatusBadRequest)
		return
	}

	ctx := util.RequestCtx(r)
	// Don't process the same url while somebody else is processing it
	if err := s.markProcessing(ctx, u); err != nil {
		log.Printf("URL '%s' is being processed", u.String())
		http.Error(w, "url is being processed", 500)
		return
	}
	defer s.unmarkProcessing(ctx, u)

	// start := time.Now()
	if err := s.backend.StoreTransformedContent(ctx, u); err != nil {
		log.Printf("Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// TODO: allow configuring this later
	// w.Header().Add("X-Sharaq-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
}

// handleDelete accepts DELETE requests to delete all known resized images
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, `not authorized`, http.StatusForbidden)
		return
	}

	u, err := util.GetTargetURL(r)
	if err != nil {
		http.Error(w, `url parameter missing`, http.StatusBadRequest)
		return
	}

	ctx := util.RequestCtx(r)

	// Don't process the same url while somebody else is processing it
	if err := s.markProcessing(ctx, u); err != nil {
		http.Error(w, "url is being processed", 500)
		return
	}
	defer s.unmarkProcessing(ctx, u)

	if err := s.backend.Delete(ctx, u); err != nil {
		log.Printf("Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// w.Header().Add("X-Sharaq-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
}

func (s *Server) authorized(r *http.Request) bool {
	// Must have token in header
	// XXX Allow tokens in database
	tok := r.Header.Get("Sharaq-Token")
	_, ok := s.tokens[tok]
	return ok
}
