package resize

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

type App struct {
	// Logger specifies an optional logger for events
	// that occur while serving content.
	// If nil, logging goes to os.Stderr via the log package's
	// standard logger.
	Logger *log.Logger

	// ReloadTemplates specifies if the App will recompile
	// the templates before rendering each response.
	// This option is intended for development, and should
	// not be used on a production server.
	ReloadTemplates bool

	tmplDir string

	tmpl   map[string]*template.Template
	router http.Handler
}

// NewApp initializes an App by parsing templates, and initializing
// the internal path router.
func NewApp(static, templates string) (*App, error) {
	app := &App{tmplDir: templates}

	err := app.compileTemplates(templates)
	if err != nil {
		return nil, fmt.Errorf("compiling templates %v", err)
	}

	// helper functions for serving static assets
	fileServer := func(path string) http.Handler {
		return http.FileServer(http.Dir(filepath.Join(static, path)))
	}
	serveFile := func(path string) http.Handler {
		fp := filepath.Join(static, path)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, fp)
		})
	}

	// Define routes
	r := mux.NewRouter()

	r.PathPrefix("/css/").Handler(http.StripPrefix("/css/", fileServer("css")))
	r.PathPrefix("/js/").Handler(http.StripPrefix("/js/", fileServer("js")))

	r.Handle("/favicon.ico", serveFile("favicon.ico"))

	r.HandleFunc("/", app.handleIndex)
	r.HandleFunc("/about", app.handleAbout)

	r.NotFoundHandler = http.HandlerFunc(app.render404)
	app.router = r

	return app, nil
}

// App implements the http.Handler interface
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.router.ServeHTTP(w, r)
}

// CompileTemplates parses a template directory
func (app *App) compileTemplates(tmplDir string) error {
	join := filepath.Join

	includes := join(tmplDir, "includes")
	layouts := join(tmplDir, "layouts")

	var tmpl *template.Template
	var err error
	tmpl, err = template.ParseGlob(join(includes, "*.html"))
	if err != nil {
		return err
	}
	if _, err = tmpl.ParseGlob(join(layouts, "*.html")); err != nil {
		return err
	}

	files, err := ioutil.ReadDir(tmplDir)
	if err != nil {
		return err
	}
	app.tmpl = map[string]*template.Template{}

	for _, info := range files {
		name := info.Name()
		if info.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		t, err := tmpl.Clone()
		if err != nil {
			return err
		}
		_, err = t.ParseFiles(join(tmplDir, name))
		if err != nil {
			return err
		}
		app.tmpl[name] = t
	}
	return nil
}

// Render renders a template to the ResponseWriter with a 200 status code.
func (app *App) render(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	app.renderStatus(w, name, data, http.StatusOK)
}

// Render500 renders the 500.html template with the error message displayed to
// the user.
func (app *App) render500(w http.ResponseWriter, r *http.Request, err error) {
	data := map[string]string{
		"Error": err.Error(),
	}
	app.renderStatus(w, "500.html", data, http.StatusInternalServerError)
}

// Render404 renders the 404.html template to the user.
func (app *App) render404(w http.ResponseWriter, r *http.Request) {
	app.Logf("%s not found", r.RequestURI)
	app.renderStatus(w, "404.html", nil, http.StatusNotFound)
}

func (app *App) renderStatus(
	w http.ResponseWriter,
	name string,
	data interface{},
	status int) {

	if app.ReloadTemplates {
		err := app.compileTemplates(app.tmplDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	tmpl, ok := app.tmpl[name]
	if !ok {
		app.Logf("no template named %s", name)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)

	err := tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		app.Logf("error rendering template %s %v", name, err)
	}
}

// Logf prints a message to the apps declared logger
func (app *App) Logf(format string, a ...interface{}) {
	if app.Logger == nil {
		log.Printf(format, a...)
	} else {
		app.Logger.Printf(format, a...)
	}
}
