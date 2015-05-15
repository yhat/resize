package resize

import (
	"fmt"
	"net/http"

	"github.com/mitchellh/goamz/ec2"
)

func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	_, ok := app.creds(w, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not implemented", http.StatusNotImplemented)
		return
	}
	app.render(w, r, "index.html", nil)
}

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if _, ok := app.creds(w, r); ok {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		app.render(w, r, "login.html", nil)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not implemented", http.StatusNotImplemented)
		return
	}

	// Handle POST
	accessKey := r.FormValue("accessKey")
	secretKey := r.FormValue("secretKey")
	if accessKey == "" {
		http.Error(w, "No access key provided", http.StatusBadRequest)
		return
	}
	if secretKey == "" {
		http.Error(w, "No secret key provided", http.StatusBadRequest)
		return
	}
	err := app.login(w, r, accessKey, secretKey)
	if err == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err, ok := err.(*ec2.Error); ok {
		msg := fmt.Sprintf("bad response from AWS '%s'", err.Message)
		http.Error(w, msg, http.StatusBadRequest)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *App) handleAbout(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "about.html", nil)
}
