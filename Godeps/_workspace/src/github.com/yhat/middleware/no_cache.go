package middleware

import "net/http"

func NoCaching(handler http.Handler) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		// see: http://goo.gl/itaIDo
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		handler.ServeHTTP(w, r)
	}
	return http.HandlerFunc(hf)
}
