package resize

import (
	"crypto/rand"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/mitchellh/goamz/aws"
	"golang.org/x/net/websocket"
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

	// The HTTP client used for all request to AWS.
	// If nil, the aws.Retrying client is used.
	HTTPClient *http.Client

	store *sessions.CookieStore

	tmplDir string

	tmpl   map[string]*template.Template
	router http.Handler
}

// NewApp initializes an App by parsing templates, and initializing
// the internal path router.
// If store is nil, a CookieStore with a random secret key is provided.
func NewApp(static, templates string, store *sessions.CookieStore) (*App, error) {
	app := &App{tmplDir: templates}

	err := app.compileTemplates(templates)
	if err != nil {
		return nil, fmt.Errorf("compiling templates %v", err)
	}

	if store != nil {
		app.store = store
	} else {
		secretKey := make([]byte, 32)
		_, err = io.ReadFull(rand.Reader, secretKey)
		if err != nil {
			return nil, err
		}
		app.store = sessions.NewCookieStore(secretKey)
	}

	// helper functions for serving static assets
	serveDir := func(path string) http.Handler {
		return http.FileServer(http.Dir(filepath.Join(static, path)))
	}
	serveFile := func(path string) http.Handler {
		fp := filepath.Join(static, path)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, fp)
		})
	}

	// restrict a handle to only those which have logged in
	restrict := func(hf http.HandlerFunc) http.Handler { return app.restrict(hf) }

	// Define routes
	r := mux.NewRouter()

	r.PathPrefix("/css/").Handler(http.StripPrefix("/css/", serveDir("css")))
	r.PathPrefix("/js/").Handler(http.StripPrefix("/js/", serveDir("js")))
	r.PathPrefix("/img/").Handler(http.StripPrefix("/img/", serveDir("img")))

	r.Handle("/favicon.ico", serveFile("favicon.ico"))

	r.HandleFunc("/login", app.handleLogin)
	r.HandleFunc("/logout", app.handleLogout)
	r.HandleFunc("/about", app.handleAbout)

	r.Handle("/", restrict(app.handleIndex))
	r.Handle("/region", restrict(app.handleRegion))
	r.Handle("/instance/{instance}", restrict(app.handleInstance))
	r.Handle("/instance/{instance}/resize",
		websocket.Handler(app.handleResize))
	r.Handle("/instance/{instance}/assign-ip",
		websocket.Handler(app.handleAssignIp))

	r.NotFoundHandler = http.HandlerFunc(app.render404)
	app.router = r

	return app, nil
}

// App implements the http.Handler interface
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.router.ServeHTTP(w, r)
}

// Logf prints a message to the apps declared logger
func (app *App) Logf(format string, a ...interface{}) {
	if app.Logger == nil {
		log.Printf(format, a...)
	} else {
		app.Logger.Printf(format, a...)
	}
}

func (app *App) httpClient() *http.Client {
	if app.HTTPClient == nil {
		return aws.RetryingClient
	}
	return app.HTTPClient
}
