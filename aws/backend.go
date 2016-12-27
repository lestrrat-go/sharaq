package aws

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"

	"golang.org/x/net/context"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
	"github.com/lestrrat/sharaq/internal/bbpool"
	"github.com/lestrrat/sharaq/internal/transformer"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/lestrrat/sharaq/internal/util"
)

type S3Backend struct {
	bucketName  string
	bucket      *s3.Bucket
	cache       *urlcache.URLCache
	presets     map[string]string
	transformer *transformer.Transformer
}

func NewBackend(c *Config, cache *urlcache.URLCache, trans *transformer.Transformer, presets map[string]string) (*S3Backend, error) {
	auth := aws.Auth{
		AccessKey: c.AccessKey,
		SecretKey: c.SecretKey,
	}

	s3o := s3.New(auth, aws.APNortheast)
	return &S3Backend{
		bucket:      s3o.Bucket(c.BucketName),
		bucketName:  c.BucketName,
		cache:       cache,
		presets:     presets,
		transformer: trans,
	}, nil
}

func (s *S3Backend) Serve(w http.ResponseWriter, r *http.Request) {
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

	cacheKey := urlcache.MakeCacheKey("s3", preset, u.String())
	if cachedURL := s.cache.Lookup(util.RequestCtx(r), cacheKey); cachedURL != "" {
		log.Printf("Cached entry found for %s:%s -> %s", preset, u.String(), cachedURL)
		w.Header().Add("Location", cachedURL)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	// create the proper url
	specificURL := "http://" + s.bucketName + ".s3.amazonaws.com/" + preset + u.Path

	log.Printf("Making HEAD request to %s...", specificURL)
	res, err := http.Head(specificURL)
	if err != nil {
		log.Printf("Failed to make HEAD request to %s: %s", specificURL, err)
		goto FALLBACK
	}

	log.Printf("HEAD request for %s returns %d", specificURL, res.StatusCode)
	if res.StatusCode == 200 {
		go s.cache.Set(context.Background(), cacheKey, specificURL)
		log.Printf("HEAD request to %s was success. Redirecting to proper location", specificURL)
		w.Header().Add("Location", specificURL)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	go func() {
		if err := s.StoreTransformedContent(util.RequestCtx(r), u); err != nil {
			log.Printf("S3Backend: transformation failed: %s", err)
		}
	}()

FALLBACK:
	w.Header().Add("Location", u.String())
	w.WriteHeader(302)
}

func (s *S3Backend) StoreTransformedContent(ctx context.Context, u *url.URL) error {
	log.Printf("S3Backend: transforming image at url %s", u)

	// Transformation is completely done by the transformer, so just
	// hand it over to it
	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(s.presets))
	for preset, rule := range s.presets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, t *transformer.Transformer, preset string, rule string, errCh chan error) {
			defer wg.Done()

			buf := bbpool.Get()
			defer bbpool.Release(buf)

			var res transformer.Result
			res.Content = buf

			if err := t.Transform(rule, u.String(), &res); err != nil {
				errCh <- err
				return
			}

			// good, done. save it to S3
			path := "/" + preset + u.Path
			log.Printf("Sending PUT to S3 %s...", path)
			err := s.bucket.PutReader(path, buf, res.Size, res.ContentType, s3.PublicRead, s3.Options{})
			if err != nil {
				errCh <- err
				return
			}
		}(wg, s.transformer, preset, rule, errCh)
	}
	wg.Wait()
	close(errCh)

	buf := bbpool.Get()
	defer bbpool.Release(buf)

	for err := range errCh {
		fmt.Fprintf(buf, "Err: %s\n", err)
	}

	if buf.Len() > 0 {
		return fmt.Errorf("error while transforming: %s", buf.String())
	}

	return nil
}

func (s *S3Backend) Delete(ctx context.Context, u *url.URL) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(s.presets))
	for preset := range s.presets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, preset string, errCh chan error) {
			defer wg.Done()
			path := "/" + preset + u.Path
			log.Printf(" + DELETE S3 entry %s\n", path)
			err := s.bucket.Del(path)
			if err != nil {
				errCh <- err
			}

			// fallthrough here regardless, because it's better to lose the
			// cache than to accidentally have one linger
			s.cache.Delete(context.Background(), urlcache.MakeCacheKey(preset, u.String()))
		}(&wg, preset, errCh)
	}

	wg.Wait()
	close(errCh)

	buf := bbpool.Get()
	defer bbpool.Release(buf)

	for err := range errCh {
		fmt.Fprintf(buf, "Err: %s\n", err)
	}

	if buf.Len() > 0 {
		return fmt.Errorf("error while deleting: %s", buf.String())
	}

	return nil
}
