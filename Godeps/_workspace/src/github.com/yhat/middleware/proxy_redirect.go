package middleware

import (
	"bufio"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// ProxyRedirect joins basePath to the location header of all redirects
// generated the by handler. It also changes the scheme as provided.
//
//     p := httputil.NewSingleHostReverseProxy(u)
//     // force redirects to use https and prepend "/foo/bar" to all paths
//     h := middleware.ProxyRedirect("https", "/foo/bar", p)
//
// TODO: Add refresh header handling
func ProxyRedirect(scheme, basePath string, handler http.Handler) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		pp := &proxyRedirectWrapper{
			scheme:   scheme,
			basePath: basePath,
			wr:       w,
		}
		hijacker, ok := w.(http.Hijacker)
		if ok {
			w = &proxyRedirectHijacker{pp, hijacker}
		} else {
			w = pp
		}
		handler.ServeHTTP(w, r)
	}
	return http.HandlerFunc(hf)
}

type proxyRedirectWrapper struct {
	scheme   string
	basePath string
	wr       http.ResponseWriter
}

func (pp *proxyRedirectWrapper) Header() http.Header {
	return pp.wr.Header()
}

func (pp *proxyRedirectWrapper) Write(p []byte) (int, error) {
	// If WriteHeader was not called before this, then the status is 200 and
	// proxyRedirectWrapper won't edit the rsponse
	return pp.wr.Write(p)
}

func (pp *proxyRedirectWrapper) WriteHeader(status int) {
	if status/100 == 3 {
		location := pp.wr.Header().Get("Location")
		u, err := url.Parse(location)
		if err == nil {
			if pp.scheme != "" {
				u.Scheme = pp.scheme
			}
			u.Path = singleJoiningSlash(pp.basePath, u.Path)
			pp.wr.Header().Set("Location", u.String())
		}
	}
	pp.wr.WriteHeader(status)
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

type proxyRedirectHijacker struct {
	*proxyRedirectWrapper
	hijacker http.Hijacker
}

func (pp *proxyRedirectHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return pp.hijacker.Hijack()
}
