package resize

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
)

func allInstances(resp *ec2.InstancesResp) []ec2.Instance {
	instances := []ec2.Instance{}
	for _, res := range resp.Reservations {
		for _, inst := range res.Instances {
			instances = append(instances, inst)
		}
	}
	return instances
}

// Path: /
func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	ec2Cli, ok := app.creds(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not implemented", http.StatusNotImplemented)
		return
	}
	resp, err := ec2Cli.Instances(nil, nil)
	if err != nil {
		app.render500(w, r, err)
		return
	}
	data := map[string]interface{}{"Instances": allInstances(resp)}
	app.render(w, r, "index.html", data)
}

// Path: /login
func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if _, ok := app.creds(r); ok {
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

// Path: /about
func (app *App) handleAbout(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "about.html", nil)
}

// Path: /logout
func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	app.logout(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Path: /region
func (app *App) handleRegion(w http.ResponseWriter, r *http.Request) {
	ec2Cli, ok := app.creds(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not implemented", http.StatusNotImplemented)
		return
	}
	regionName := r.PostFormValue("region")
	if regionName == "" {
		http.Error(w, "No region provided", http.StatusBadRequest)
		return
	}
	region, ok := aws.Regions[regionName]
	if !ok {
		http.Error(w, "No AWS region named "+regionName, http.StatusBadRequest)
		return
	}

	ec2Cli.Region = region
	if err := app.set(w, r, ec2Cli); err != nil {
		app.Logf("could not set region for cookie: %v", err)
		http.Error(w, "internal error setting cookie", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Path: /instance/{instance}
func (app *App) handleInstance(w http.ResponseWriter, r *http.Request) {
	ec2Cli, ok := app.creds(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not implemented", http.StatusNotImplemented)
		return
	}

	instanceId := mux.Vars(r)["instance"]
	if instanceId == "" {
		app.render404(w, r)
		return
	}

	resp, err := ec2Cli.Instances([]string{instanceId}, nil)
	if err != nil {
		app.render500(w, r, fmt.Errorf("Bad response from AWS %v", err))
		return
	}
	instances := allInstances(resp)
	if len(instances) != 1 {
		app.render404(w, r)
		return
	}
	instance := instances[0]
	data := map[string]interface{}{"Instance": instance}
	app.render(w, r, "instance.html", data)
}
