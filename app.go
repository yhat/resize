package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/yhat/middleware"
	"github.com/yhat/resize/resize"
)

var defaultAddr = ":4040"

func main() {

	public := flag.String("public", "./public", "`path` of the directory holding static content")
	templates := flag.String("templates", "./templates", "`path` of the directory holding app templates")
	httpAddr := flag.String("http", defaultAddr, "HTTP address for the app")
	reloadTmpl := flag.Bool("recompile", false, "should the app recompile templates on each request")

	flag.Parse()

	app, err := resize.NewApp(*public, *templates)
	if err != nil {
		log.Fatal(err)
	}
	app.ReloadTemplates = *reloadTmpl
	h := middleware.GZip(app)
	log.Println("listening on " + *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, h))
}
