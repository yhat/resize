package middleware

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const defaultStatus int = -1

type statusWrapper struct {
	wr     http.ResponseWriter
	status int
	nbytes int
}

func (w *statusWrapper) Header() http.Header { return w.wr.Header() }

func (w *statusWrapper) WriteHeader(status int) {
	w.wr.WriteHeader(status)
	w.status = status
}

func (w *statusWrapper) Write(p []byte) (n int, err error) {
	if w.status == defaultStatus {
		w.status = http.StatusOK
	}
	n, err = w.wr.Write(p)
	w.nbytes += n
	return
}

type statusHijacker struct {
	*statusWrapper
	hijacker http.Hijacker
}

func (w *statusHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// if the ResponseWriter is hijacked we'll assume the request was a success
	if w.statusWrapper.status == defaultStatus {
		w.statusWrapper.status = http.StatusOK
	}
	return w.hijacker.Hijack()
}

func Log(log io.Writer, h http.Handler) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := r.URL.String()
		method := r.Method
		origin := r.Header.Get("X-Forwarded-For")
		if origin == "" {
			origin = r.RemoteAddr
		}
		userAgent := r.UserAgent()
		wrapper := &statusWrapper{w, defaultStatus, 0}
		hijacker, ok := w.(http.Hijacker)
		if ok {
			w = &statusHijacker{wrapper, hijacker}
		} else {
			w = wrapper
		}

		h.ServeHTTP(w, r)

		diff := time.Since(start)
		args := []interface{}{
			start.Format("2006/01/02 15:04:05"),
			method,
			wrapper.status,
			path,
			origin,
			wrapper.nbytes,
			diff.String(),
			userAgent,
		}
		fmt.Fprintf(log, "%s %s %d %s %s %d %s '%s'\n", args...)
	}
	return http.HandlerFunc(hf)
}
