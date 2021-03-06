package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/gorilla/sessions"
	"github.com/yhat/middleware"
	"github.com/yhat/resize/resize"
)

var defaultAddr = ":4040"

func main() {

	httpAddr := flag.String("http", defaultAddr, "HTTP address for the app")

	httpsAddr := flag.String("https", "", "HTTPS address for the app")
	tlsCert := flag.String("tlscert", "", "cert.crt file for TLS")
	tlsKey := flag.String("tlskey", "", "cert.key file for TLS")

	public := flag.String("public", "./public", "`path` of the directory holding static content")
	templates := flag.String("templates", "./templates", "`path` of the directory holding app templates")
	reloadTmpl := flag.Bool("reload-templates", false, "should the app recompile templates on each request")

	sessionkey := flag.String("sessionkey", "", "secret key for session cookies")

	accessLog := flag.String("accesslog", "", "file for access log")

	flag.Parse()

	var store *sessions.CookieStore
	if *sessionkey != "" {
		store = sessions.NewCookieStore([]byte(*sessionkey))
	}

	app, err := resize.NewApp(*public, *templates, store)
	if err != nil {
		log.Fatal(err)
	}
	app.ReloadTemplates = *reloadTmpl
	h := middleware.GZip(app)

	var logDest io.Writer
	if *accessLog == "" {
		logDest = os.Stderr
	} else {
		file, err := os.OpenFile(*accessLog, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal(err)
		}
		logDest = file
	}
	h = middleware.Log(logDest, h)

	httpURL := (&url.URL{Scheme: "http", Host: expandHost(*httpAddr), Path: "/"}).String()

	if *httpsAddr == "" {
		log.Println("listening on " + httpURL)
		log.Fatal(http.ListenAndServe(*httpAddr, h))
	}

	httpsURL := (&url.URL{Scheme: "https", Host: expandHost(*httpsAddr), Path: "/"}).String()

	// redirect all HTTP requests to HTTPS
	redirect := func(w http.ResponseWriter, r *http.Request) {
		to := (&url.URL{Scheme: "https", Host: expandHost(*httpsAddr), Path: r.URL.Path}).String()
		http.Redirect(w, r, to, http.StatusMovedPermanently)
	}

	go func() {
		log.Fatal(http.ListenAndServe(*httpAddr, http.HandlerFunc(redirect)))
	}()

	log.Println("listening on " + httpsURL)
	log.Fatal(http.ListenAndServeTLS(*httpsAddr, *tlsCert, *tlsKey, h))
}

// expand ':4040' to '0.0.0.0:4040'
func expandHost(addr string) string {
	if addr == "" {
		return ""
	}
	if addr[0] == ':' {
		return "0.0.0.0" + addr
	}
	return addr
}
