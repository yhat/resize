package resize

import (
	"fmt"
	"net/http"

	"github.com/mitchellh/goamz/ec2"
)

func (app *App) handleIndex(w http.ResponseWriter, r *http.Request, cli *ec2.EC2) {
}

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		app.render(w, r, "login.html", nil)
		return
	case "POST":
	default:
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
