package resize

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
	"golang.org/x/net/websocket"
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

	addresses, err := openIps(ec2Cli)
	if err != nil {
		app.render500(w, r, err)
		return
	}
	data["Addresses"] = addresses

	filter := ec2.NewFilter()
	filter.Add("instance-id", instanceId)
	addrResp, err := ec2Cli.Addresses(nil, nil, filter)
	if err == nil && (len(addrResp.Addresses) == 1) {
		data["Address"] = addrResp.Addresses[0]
	}
	types, err := InstanceTypes(nil)
	if err != nil {
		app.render500(w, r, err)
		return
	}
	data["InstanceTypes"] = types

	app.render(w, r, "instance.html", data)
}

type Event struct {
	Status  string
	Message string
}

func (app *App) handleResize(ws *websocket.Conn) {
	defer ws.Close()

	r := ws.Request()
	ec2Cli, ok := app.creds(r)
	if !ok {
		app.wsErr(ws, "Unauthorized")
		return
	}

	instanceId := mux.Vars(r)["instance"]
	if instanceId == "" {
		app.wsErr(ws, "No instance ID included")
		return
	}

	currentStatus := r.URL.Query().Get("status")

	var newType string
	if err := websocket.Message.Receive(ws, &newType); err != nil {
		app.wsErr(ws, fmt.Sprintf("error receiving websocket message: %v", err))
		return
	}

	//The instance must be stopped before we can change it
	switch currentStatus {
	case "running":
		if err := stopAndWait(ec2Cli, ws, instanceId); err != nil {
			app.wsErr(ws, fmt.Sprintf("error stopping instance: %v", err))
			return
		}
	case "stopped":
		break
	default:
		app.wsErr(ws, "The server is not in a state from which its size can be changed. The server's state must be either 'stopped' or 'running.'")
		return
	}
	if err := resize(ec2Cli, instanceId, newType); err != nil {
		app.wsErr(ws, fmt.Sprintf("error resizing instance: %v", err))
		return
	}
	//If the server was running initially, we'll return it to its original
	//state and keep the user informed of this process
	if currentStatus == "running" {
		if _, err := ec2Cli.StartInstances(instanceId); err != nil {
			app.wsErr(ws, fmt.Sprintf("error starting instance: %v", err))
			return
		}
		if err := pollUntilRunning(ec2Cli, ws, instanceId); err != nil {
			app.wsErr(ws, fmt.Sprintf("error checking instance status: %v", err))
			return
		}
	}
	e := Event{Status: "success"}
	websocket.JSON.Send(ws, &e)
}

func (app *App) handleAssignIp(ws *websocket.Conn) {
	defer ws.Close()

	r := ws.Request()
	ec2Cli, ok := app.creds(r)
	if !ok {
		app.wsErr(ws, "Unauthorized")
		return
	}

	instanceId := mux.Vars(r)["instance"]
	if instanceId == "" {
		app.wsErr(ws, "No instance ID included")
		return
	}
	currentStatus := r.URL.Query().Get("status")

	var allocId string
	if err := websocket.Message.Receive(ws, &allocId); err != nil {
		app.wsErr(ws, fmt.Sprintf("error receiving websocket message: %v", err))
		return
	}

	switch currentStatus {
	case "running":
		if err := stopAndWait(ec2Cli, ws, instanceId); err != nil {
			app.wsErr(ws, fmt.Sprintf("error stopping instance: %v", err))
			return
		}
	case "stopped":
		break
	default:
		app.wsErr(ws, "The server is not in a state from which its size can be changed. The server's state must be either 'stopped' or 'running.'")
		return
	}

	err := allocateIp(ec2Cli, instanceId, allocId)
	if err != nil {
		app.wsErr(ws, fmt.Sprintf("could not allocate elastic IP: %v", err))
		return
	}
	if currentStatus == "running" {
		if _, err := ec2Cli.StartInstances(instanceId); err != nil {
			app.wsErr(ws, fmt.Sprintf("error starting instance: %v", err))
			return
		}
		if err := pollUntilRunning(ec2Cli, ws, instanceId); err != nil {
			app.wsErr(ws, fmt.Sprintf("error checking instance status: %v", err))
			return
		}
	}
	e := Event{Status: "success"}
	websocket.JSON.Send(ws, &e)
}
