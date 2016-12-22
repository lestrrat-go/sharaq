package gcp

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"

	"github.com/lestrrat/sharaq/internal/transformer"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/lestrrat/sharaq/internal/util"
	"github.com/pkg/errors"
)

type StorageBackend struct {
	bucketName  string
	cache       *urlcache.URLCache
	presets     map[string]string
	transformer *transformer.Transformer
}

func NewBackend(c *Config, cache *urlcache.URLCache, trans *transformer.Transformer, presets map[string]string) (*StorageBackend, error) {
	return &StorageBackend{
		bucketName:  c.BucketName,
		cache:       cache,
		presets:     presets,
		transformer: trans,
	}, nil
}

func (s *StorageBackend) getClient(ctx context.Context) (*storage.Client, error) {
	tokesrc, err := google.DefaultTokenSource(ctx, storage.ScopeFullControl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get default token source for storage client")
	}

	client, err := storage.NewClient(ctx, option.WithTokenSource(tokesrc))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create storage client")
	}
	return client, nil
}

func (s *StorageBackend) Serve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	u, err := util.GetTargetURL(r)
	if err != nil {
		log.Printf("Bad url: %s", err)
		http.Error(w, "Bad url", 500)
		return
	}

	preset, err := util.GetPresetFromRequest(r)
	if err != nil {
		log.Printf("Bad preset: %s", err)
		http.Error(w, "Bad preset", 500)
		return
	}

	cacheKey := urlcache.MakeCacheKey("gcp", preset, u.String())
	if cachedURL := s.cache.Lookup(cacheKey); cachedURL != "" {
		log.Printf("Cached entry found for %s:%s -> %s", preset, u.String(), cachedURL)
		w.Header().Add("Location", cachedURL)
		w.WriteHeader(301)
		return
	}

	// create the proper url
	specificURL := u.Scheme + "://storage.googleapis.com/" + s.bucketName + "/" + preset + u.Path

	log.Printf("Making HEAD request to %s...", specificURL)
	res, err := http.Head(specificURL)
	if err != nil {
		log.Printf("Failed to make HEAD request to %s: %s", specificURL, err)
		goto FALLBACK
	}

	log.Printf("HEAD request for %s returns %d", specificURL, res.StatusCode)
	if res.StatusCode == 200 {
		go s.cache.Set(cacheKey, specificURL)
		log.Printf("HEAD request to %s was success. Redirecting to proper location", specificURL)
		w.Header().Add("Location", specificURL)
		w.WriteHeader(301)
		return
	}

	go func() {
		if err := s.StoreTransformedContent(ctx, u); err != nil {
			log.Printf("StorageBackend: transformation failed: %s", err)
		}
	}()

FALLBACK:
	w.Header().Add("Location", u.String())
	w.WriteHeader(302)
}

func (s *StorageBackend) StoreTransformedContent(ctx context.Context, u *url.URL) error {
	log.Printf("StorageBackend: transforming image at url %s", u)

	cl, err := s.getClient(ctx)
	if err != nil {
		return errors.Wrap(err, `failed to get client for Store`)
	}

	bkt := cl.Bucket(s.bucketName)

	var grp *errgroup.Group
	grp, ctx = errgroup.WithContext(ctx)

	// Transformation is completely done by the transformer, so just
	// hand it over to it
	for preset, rule := range s.presets {
		t := s.transformer
		preset := preset
		rule := rule
		grp.Go(func() error {
			res, err := t.Transform(rule, u.String())
			if err != nil {
				return errors.Wrap(err, `failed to transform image`)
			}

			// good, done. save it to S3
			path := "/" + preset + u.Path
			log.Printf("Writing to Google Storage %s...", path)

			wc := bkt.Object(path).NewWriter(ctx)

			wc.ContentType = res.ContentType
			wc.ACL = []storage.ACLRule{
				{storage.AllUsers, storage.RoleReader},
			}

			if _, err := io.Copy(wc, res.Content); err != nil {
				return errors.Wrapf(err, `failed to write data to %s`, path)
			}

			return errors.Wrap(wc.Close(), `failed to properly close writer for google storage`)
		})
	}
	return grp.Wait()
}

func (s *StorageBackend) Delete(ctx context.Context, u *url.URL) error {
	cl, err := s.getClient(ctx)
	if err != nil {
		return errors.Wrap(err, `failed to get client for Delete`)
	}

	bkt := cl.Bucket(s.bucketName)

	var grp *errgroup.Group
	grp, ctx = errgroup.WithContext(ctx)

	for preset := range s.presets {
		preset := preset
		grp.Go(func() error {
			// delete the cache regardless, because it's better to lose the
			// cache than to accidentally have one linger
			defer s.cache.Delete(urlcache.MakeCacheKey(preset, u.String()))

			path := "/" + preset + u.Path
			log.Printf(" + DELETE Google Storage entry %s\n", path)
			return bkt.Object(path).Delete(ctx)
		})
	}

	return errors.Wrap(grp.Wait(), `deleting from google storage`)
}
