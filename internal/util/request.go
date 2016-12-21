package util

import (
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

var ErrInvalidPreset = errors.New("invalid preset parameter")

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
