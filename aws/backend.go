package aws

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
	"github.com/lestrrat-go/sharaq/internal/bbpool"
	"github.com/lestrrat-go/sharaq/internal/errors"
	"github.com/lestrrat-go/sharaq/internal/log"
	"github.com/lestrrat-go/sharaq/internal/transformer"
	"github.com/lestrrat-go/sharaq/internal/urlcache"
	"github.com/lestrrat-go/sharaq/internal/util"
)

type S3Backend struct {
	bucketName  string
	bucket      *s3.Bucket
	cache       *urlcache.URLCache
	presets     map[string]string
	transformer *transformer.Transformer
}

type redirectContent string

func (s redirectContent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debugf(util.RequestCtx(r), "Object %s exists. Redirecting to proper location", string(s))
	w.Header().Add("Location", string(s))
	w.WriteHeader(http.StatusMovedPermanently)
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

func (s *S3Backend) Get(ctx context.Context, u *url.URL, preset string) (http.Handler, error) {
	cacheKey := urlcache.MakeCacheKey("aws", preset, u.String())
	if cachedURL := s.cache.Lookup(ctx, cacheKey); cachedURL != "" {
		log.Debugf(ctx, "Cached entry found for %s:%s -> %s", preset, u.String(), cachedURL)
		return redirectContent(cachedURL), nil
	}

	// create the proper url
	specificURL := "http://" + s.bucketName + ".s3.amazonaws.com/" + preset + u.Path

	log.Debugf(ctx, "Making HEAD request to %s...", specificURL)
	res, err := http.Head(specificURL)
	if err != nil {
		return nil, errors.TransformationRequiredError{}
	}

	log.Debugf(ctx, "HEAD request for %s returns %d", specificURL, res.StatusCode)
	if res.StatusCode != http.StatusOK {
		return nil, errors.TransformationRequiredError{}
	}

	return redirectContent(specificURL), nil
}

func (s *S3Backend) StoreTransformedContent(ctx context.Context, u *url.URL) error {
	log.Debugf(ctx, "S3Backend: transforming image at url %s", u)

	// Transformation is completely done by the transformer, so just
	// hand it over to it
	var grp *errgroup.Group
	grp, ctx = errgroup.WithContext(ctx)

	for preset, rule := range s.presets {
		t := s.transformer
		preset := preset
		rule := rule
		grp.Go(func() error {
			buf := bbpool.Get()
			defer bbpool.Release(buf)

			var res transformer.Result
			res.Content = buf

			if err := t.Transform(ctx, rule, u.String(), &res); err != nil {
				return errors.Wrap(err, `failed to transform image`)
			}

			// good, done. save it to S3
			path := "/" + preset + u.Path
			log.Debugf(ctx, "Sending PUT to S3 %s...", path)
			if err := s.bucket.PutReader(path, buf, res.Size, res.ContentType, s3.PublicRead, s3.Options{}); err != nil {
				return errors.Wrapf(err, `failed to write data to %s`, path)
			}
			cacheKey := urlcache.MakeCacheKey("gcp", preset, u.String())
			specificURL := "http://" + s.bucketName + ".s3.amazonaws.com/" + preset + u.Path
			s.cache.Set(ctx, cacheKey, specificURL)
			return nil
		})
	}
	return grp.Wait()
}

func (s *S3Backend) Delete(ctx context.Context, u *url.URL) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(s.presets))
	for preset := range s.presets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, preset string, errCh chan error) {
			defer wg.Done()
			path := "/" + preset + u.Path
			log.Debugf(ctx, " + DELETE S3 entry %s\n", path)
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
