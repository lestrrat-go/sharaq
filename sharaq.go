package sharaq

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/lestrrat/sharaq/aws"
	"github.com/lestrrat/sharaq/fs"
	"github.com/lestrrat/sharaq/gcp"
	"github.com/lestrrat/sharaq/internal/errors"
	"github.com/lestrrat/sharaq/internal/log"
	"github.com/lestrrat/sharaq/internal/transformer"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/lestrrat/sharaq/internal/util"
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

	s.whitelist = make([]*regexp.Regexp, len(c.Whitelist))
	for i, pat := range c.Whitelist {
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, err
		}
		s.whitelist[i] = re
	}
	if c.Debug {
		s.dumpConfig()
	}

	return s, nil
}

func (s *Server) Initialize() error {
	var err error
	s.cache, err = urlcache.New(s.config.URLCache)
	if err != nil {
		return errors.Wrap(err, `failed to create urlcache`)
	}
	s.transformer = transformer.New()

	if err := s.newBackend(); err != nil {
		return errors.Wrap(err, `failed to create storage backend`)
	}
	return nil
}

func (s *Server) dumpConfig() {
	j, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return
	}

	ctx := context.Background()
	scanner := bufio.NewScanner(bytes.NewBuffer(j))
	for scanner.Scan() {
		l := scanner.Text()
		log.Debugf(ctx, l)
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
	case "POST":
		s.handleStore(w, r)
	case "DELETE":
		s.handleDelete(w, r)
	default:
		http.Error(w, "What, what, what?", http.StatusBadRequest)
	}
}

func (s *Server) allowedTarget(u *url.URL) bool {
	if len(s.whitelist) == 0 {
		return true
	}

	for _, pat := range s.whitelist {
		if pat.MatchString(u.String()) {
			return true
		}
	}
	return false
}

// handleFetch replies with the proper URL of the image
func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request) {
	ctx := util.RequestCtx(r)

	u, err := util.GetTargetURL(r)
	if err != nil {
		log.Debugf(ctx, "Bad url: %s", err)
		http.Error(w, "Bad url", http.StatusBadRequest)
		return
	}

	if !s.allowedTarget(u) {
		http.Error(w, "Specified url not allowed", http.StatusForbidden)
		return
	}

	preset, err := util.GetPresetFromRequest(r)
	if err != nil {
		log.Debugf(ctx, "Bad preset: %s", err)
		http.Error(w, "Bad preset", http.StatusBadRequest)
		return
	}

	content, err := s.backend.Get(ctx, u, preset)
	if err == nil {
		content.ServeHTTP(w, r)
		return
	}

	if !errors.IsTransformationRequired(err) {
		log.Debugf(ctx, "failed to serve from backend: %s", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	if err := s.deferedTransformAndStore(ctx, u); err != nil {
		log.Debugf(ctx, "failed to transform content: %s", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	// Serve the original file, just so that we don't return an error
	log.Debugf(ctx, "Fallback to serving original content at %s", u)
	w.Header().Add("Location", u.String())
	w.WriteHeader(http.StatusFound)

	return
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

// handleStore accepts POST requests to create resized images and
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
	if err := s.transformAndStore(ctx, u); err != nil {
		log.Debugf(ctx, "Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) transformAndStore(ctx context.Context, u *url.URL) error {
	// Don't process the same url while somebody else is processing it
	if err := s.markProcessing(ctx, u); err != nil {
		return errors.Wrap(err, `failed to mark processing flag`)
	}
	defer s.unmarkProcessing(ctx, u)

	if err := s.backend.StoreTransformedContent(ctx, u); err != nil {
		return errors.Wrap(err, `failed to process content`)
	}
	return nil
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
		log.Debugf(ctx, "Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// w.Header().Add("X-Sharaq-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
}

func (s *Server) authorized(r *http.Request) bool {
	if r.Header.Get("X-Appengine-Request-Log-Id") != "" {
		// Trust inbound taskqueue requests
		return true
	}

	// Must have token in header
	// XXX Allow tokens in database
	tok := r.Header.Get("Sharaq-Token")
	_, ok := s.tokens[tok]
	return ok
}
