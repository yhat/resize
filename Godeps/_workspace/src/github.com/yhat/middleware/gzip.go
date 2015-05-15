// Middleware for the net/http library
package middleware

import (
	"bufio"
	"compress/gzip"
	"net"
	"net/http"
	"strings"
	"sync"
)

type gzipWrapper struct {
	wr            http.ResponseWriter // the underlying ResponseWriter
	gzw           *gzip.Writer        // gzip wrapper on the ResponseWriter
	mu            *sync.Mutex         // to accommodate concurrent writes to the wrapper
	firstWrite    bool                // is this the first call to Write()?
	headerWritten bool                // has the header been written?
}

func (w *gzipWrapper) Header() http.Header { return w.wr.Header() }

func (w *gzipWrapper) WriteHeader(status int) {
	w.headerWritten = true
	// No good way of dealing with content-length without buffering the entire
	// response body
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Encoding", "gzip")
	w.wr.WriteHeader(status)
}

func (wrapper *gzipWrapper) Write(p []byte) (int, error) {
	// do we need to check the content type?
	if !wrapper.firstWrite {
		return wrapper.gzw.Write(p)
	}
	wrapper.mu.Lock()
	if wrapper.firstWrite {
		// Because gzipped content is written to the underlying writer, we have
		// to detect the content type of the non-gzipped bytes.
		if "" == wrapper.Header().Get("Content-Type") {
			contentType := ""
			if len(p) > 512 {
				contentType = http.DetectContentType(p[:512])
			} else {
				contentType = http.DetectContentType(p)
			}
			wrapper.Header().Set("Content-Type", contentType)
		}
		if !wrapper.headerWritten {
			wrapper.WriteHeader(http.StatusOK)
		}
	}
	wrapper.firstWrite = false
	wrapper.mu.Unlock()
	return wrapper.gzw.Write(p)
}

// gzipHijacker implements the http.Hijacker interface for ResponseWriters
// which also implement it.
type gzipHijacker struct {
	*gzipWrapper
	hijacker http.Hijacker
	hijacked bool
}

func (wrapper *gzipHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	wrapper.hijacked = true
	return wrapper.hijacker.Hijack()
}

// GZip returns a handler which gzip's the response for all requests which
// accept that encoding.
func GZip(h http.Handler) http.Handler {
	hfunc := func(w http.ResponseWriter, r *http.Request) {
		ae := strings.Split(r.Header.Get("Accept-Encoding"), ",")
		acceptsGzip := false
		for i := range ae {
			if strings.TrimSpace(ae[i]) == "gzip" {
				acceptsGzip = true
				break
			}
		}
		if acceptsGzip {
			// Remove "Accept-Encoding" header to ensure applications further
			// down the line don't attempt to gzip the response again.
			r.Header.Del("Accept-Encoding")
			gzw := gzip.NewWriter(w)
			wrapper := &gzipWrapper{w, gzw, &sync.Mutex{}, true, false}
			hijacker, ok := w.(http.Hijacker)
			if ok {
				// if writer accepts hijacking pass a hijackable wrapper
				hijackableWrapper := &gzipHijacker{wrapper, hijacker, false}
				defer func() {
					if !hijackableWrapper.hijacked {
						gzw.Close()
					}
				}()
				w = hijackableWrapper
			} else {
				defer gzw.Close()
				w = wrapper
			}
		}
		h.ServeHTTP(w, r)
	}
	return http.HandlerFunc(hfunc)
}
