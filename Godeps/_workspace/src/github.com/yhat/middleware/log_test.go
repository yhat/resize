package middleware

import (
	"bytes"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestLogger(t *testing.T) {
	nbytes := 65536
	data := make([]byte, nbytes)
	n, err := rand.Read(data)
	if err != nil {
		t.Errorf("error reading random bytes: %v", err)
		return
	}
	if n != nbytes {
		t.Errorf("short read")
		return
	}
	errMsg := "this did not work"
	hf := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
				t.Error(err)
			}
		} else {
			http.Error(w, errMsg, http.StatusInternalServerError)
		}
	}
	s := httptest.NewServer(Log(os.Stderr, http.HandlerFunc(hf)))
	http.Get(s.URL + "/ok")
	http.Get(s.URL + "/error")
}
