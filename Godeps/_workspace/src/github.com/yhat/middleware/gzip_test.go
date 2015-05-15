package middleware

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGzip ensures that requests with 'Accept-Encoding: gzip' produce a
// gzipped response.
func TestGzip(t *testing.T) {
	// ensure the data is long enough that there are multiple writes to the connection
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
	hf := func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
			t.Errorf("error writing to ResponseWriter: %v", err)
		}
	}
	h := http.HandlerFunc(hf)
	s := httptest.NewServer(GZip(h))
	defer s.Close()
	// GET request with no Accept-Encoding header
	resp, err := http.Get(s.URL + "/")
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
		return
	}
	if string(body) != string(data) {
		t.Errorf("response from non encoded request is wrong")
	}
	req, err := http.NewRequest("GET", s.URL+"/", nil)
	if err != nil {
		t.Error(err)
		return
	}
	// GET request with an Accept-Encoding header
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()
	ce := resp.Header.Get("Content-Encoding")
	if ce != "gzip" {
		t.Errorf("content-encoding is not gzip: '%s'", ce)
		return
	}
	r, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Errorf("could not create gzip reader: %v", err)
		return
	}
	body, err = ioutil.ReadAll(r)
	if err != nil {
		t.Errorf("cound not read from gzip reader: %v", err)
		return
	}
	if string(body) != string(data) {
		t.Errorf("response from encoded request is wrong")
	}
}

// TestGzipHijacker test that a wrapped gzip ResponseWriter can be hijacked
// appropriately.
func TestGzipHijacker(t *testing.T) {
	// ensure the data is long enough that there are multiple writes to the connection
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
	hf := func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("ResponseWriter is not a hijacker")
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijacking error: %v", err)
			return
		}
		defer conn.Close()
		if _, err := io.Copy(rw, bytes.NewReader(data)); err != nil {
			t.Errorf("error writing to ResponseWriter: %v", err)
		}
	}
	h := http.HandlerFunc(hf)
	s := httptest.NewServer(GZip(h))
	defer s.Close()
	req, err := http.NewRequest("GET", s.URL+"/", nil)
	if err != nil {
		t.Error(err)
		return
	}
	req.Header.Set("Accept-Encoding", "gzip")
	// write the request to a tcp connection.
	conn, err := net.Dial("tcp", strings.TrimPrefix(s.URL, "http://"))
	if err != nil {
		t.Error(err)
		return
	}
	if err := req.Write(conn); err != nil {
		t.Errorf("could not write to connection: %v", err)
		return
	}
	// reading from the connection should return the random bytes
	body, err := ioutil.ReadAll(conn)
	if err != nil {
		t.Errorf("cound not read from gzip reader: %v", err)
		return
	}
	if string(body) != string(data) {
		t.Errorf("response from encoded request is wrong")
	}
}

// TestGzip ensures that requests with 'Accept-Encoding: gzip' produce a
// gzipped response.
func TestGzipTwice(t *testing.T) {
	// ensure the data is long enough that there are multiple writes to the connection
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
	hf := func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
			t.Errorf("error writing to ResponseWriter: %v", err)
		}
	}
	h := http.HandlerFunc(hf)
	// GZIP twice
	s := httptest.NewServer(GZip(GZip(h)))
	defer s.Close()
	// GET request with no Accept-Encoding header
	resp, err := http.Get(s.URL + "/")
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
		return
	}
	if string(body) != string(data) {
		t.Errorf("response from non encoded request is wrong")
	}
	req, err := http.NewRequest("GET", s.URL+"/", nil)
	if err != nil {
		t.Error(err)
		return
	}
	// GET request with an Accept-Encoding header
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()
	ce := resp.Header.Get("Content-Encoding")
	if ce != "gzip" {
		t.Errorf("content-encoding is not gzip: '%s'", ce)
		return
	}
	r, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Errorf("could not create gzip reader: %v", err)
		return
	}
	body, err = ioutil.ReadAll(r)
	if err != nil {
		t.Errorf("cound not read from gzip reader: %v", err)
		return
	}
	if string(body) != string(data) {
		t.Errorf("response from encoded request is wrong")
	}
}
