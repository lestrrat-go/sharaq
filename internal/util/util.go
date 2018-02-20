package util

import (
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/lestrrat-go/sharaq/internal/crc64"
	"github.com/pkg/errors"
)

var ErrInvalidPreset = errors.New("invalid preset parameter")

// GetPresetFromRequest gets the "preset" parameter from the request
func GetPresetFromRequest(r *http.Request) (string, error) {
	if preset := r.FormValue("preset"); preset != "" {
		return preset, nil
	}

	// Work with deprecated "device" parameter
	if device := r.FormValue("device"); device != "" {
		return device, nil
	}

	return "", ErrInvalidPreset
}

func GetTargetURL(r *http.Request) (*url.URL, error) {
	rawValue := r.FormValue("url")
	u, err := url.Parse(rawValue)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.Errorf("scheme '%s' not supported", u.Scheme)
	}

	if u.Host == "" {
		return nil, errors.New("empty host")
	}

	return u, nil
}

func HashedPath(s ...string) string {
	v := crc64.EncodeString(s...)
	// given "abcdef", generates "a/ab/abc/abcd/abcdef"
	return filepath.Join(v[0:1], v[0:2], v[0:3], v[0:4], v)
}

