package sharaq

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newImageSource() *httptest.Server {
	return httptest.NewServer(http.FileServer(http.Dir("etc")))
}

func newSharaq(c *Config) (*Server, *httptest.Server, error) {
	s, err := NewServer(c)
	if err != nil {
		return nil, nil, err
	}

	return s, httptest.NewServer(s), nil
}

func newURL(s *httptest.Server, paths ...string) string {
	return s.URL + "/" + path.Join(paths...)
}

func TestBadRequest(t *testing.T) {
	_, st, err := newSharaq(nil)
	if !assert.NoError(t, err, "sharaq.NewServer should succeed") {
		return
	}
	defer st.Close()

	res, err := http.Get(st.URL)
	if !assert.NoError(t, err, "http.Get should succeed") {
		return
	}

	if !assert.Equal(t, http.StatusBadRequest, res.StatusCode, "http.Get should return 400") {
		return
	}
}

// Silly, but we just make sure that our utility is working
func TestImageSource(t *testing.T) {
	src := newImageSource()
	defer src.Close()

	res, err := http.Get(newURL(src, "sharaq.png"))
	if !assert.NoError(t, err, "http.Get should succeed") {
		return
	}
	defer res.Body.Close()

	inbytes, err := ioutil.ReadAll(res.Body)
	if !assert.NoError(t, err, "ioutil.ReadAll(res.Body) should succeed") {
		return
	}

	srcbytes, err := ioutil.ReadFile(filepath.Join("etc", "sharaq.png"))
	if !assert.NoError(t, err, `ioutil.ReadFile("etc/sharaq.png") should succeed`) {
		return
	}

	if !assert.Equal(t, srcbytes, inbytes, "file contents should match") {
		return
	}
}

func TestStore(t *testing.T) {
	c := Config{
		Tokens: []string{"AbCdEfG"},
	}
	_, st, err := newSharaq(&c)
	if !assert.NoError(t, err, "creating sharaq server should succeed") {
		return
	}
	defer st.Close()

	req, err := http.NewRequest(http.MethodPut, st.URL, nil)
	if !assert.NoError(t, err, "http.NewRequest should succeed") {
		return
	}

	res, err := http.DefaultClient.Do(req)
	if !assert.NoError(t, err, "http.Do should succeed") {
		return
	}

	if !assert.Equal(t, http.StatusForbidden, res.StatusCode, "status code should be forbidden") {
		return
	}

	req.Header.Set("Sharaq-Token", "AbCdEfG")
	res, err = http.DefaultClient.Do(req)
	if !assert.NoError(t, err, "http.Do should succeed") {
		return
	}

	// We didn't provide url so, we should bail there
	if !assert.Equal(t, http.StatusBadRequest, res.StatusCode, "status code should be bad request") {
		return
	}
}

func TestDelete(t *testing.T) {
	c := Config{
		Tokens: []string{"AbCdEfG"},
	}
	_, st, err := newSharaq(&c)
	if !assert.NoError(t, err, "creating sharaq server should succeed") {
		return
	}
	defer st.Close()

	req, err := http.NewRequest(http.MethodDelete, st.URL, nil)
	if !assert.NoError(t, err, "http.NewRequest should succeed") {
		return
	}

	res, err := http.DefaultClient.Do(req)
	if !assert.NoError(t, err, "http.Do should succeed") {
		return
	}

	if !assert.Equal(t, http.StatusForbidden, res.StatusCode, "status code should be forbidden") {
		return
	}

	req.Header.Set("Sharaq-Token", "AbCdEfG")
	res, err = http.DefaultClient.Do(req)
	if !assert.NoError(t, err, "http.Do should succeed") {
		return
	}

	// We didn't provide url so, we should bail there
	if !assert.Equal(t, http.StatusBadRequest, res.StatusCode, "status code should be bad request") {
		return
	}
}
