package sharaq

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBadRequest(t *testing.T) {
	s, err := NewServer(&Config{})
	if !assert.NoError(t, err, "sharaq.NewServer should succeed") {
		return
	}
	s1 := httptest.NewServer(s)
	defer s1.Close()

	res, err := http.Get(s1.URL)
	if !assert.NoError(t, err, "http.Get should succeed") {
		return
	}

	if !assert.Equal(t, http.StatusBadRequest, res.StatusCode, "http.Get should return 400") {
		return
	}
}
