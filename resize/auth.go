package resize

import (
	"encoding/gob"
	"net/http"

	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
)

func init() {
	gob.Register(&ec2.EC2{})
}

var defaultRegion = aws.USEast

// login attempts to validate the provided credentials with AWS.
// On an authentication error, error will be of type *ec2.Error
func (app *App) login(w http.ResponseWriter, r *http.Request, accessKeyID, secretKey string) error {
	ec2Cli := ec2.NewWithClient(aws.Auth{
		AccessKey: accessKeyID,
		SecretKey: secretKey,
	}, defaultRegion, app.httpClient())

	_, err := ec2Cli.Instances(nil, nil)
	if err != nil {
		return err
	}

	return app.set(w, r, ec2Cli)
}

// set associates a *ec2.EC2 instance with a session
func (app *App) set(w http.ResponseWriter, r *http.Request, ec2Cli *ec2.EC2) error {
	// ignore error from decoding an existing session
	session, _ := app.store.Get(r, "yhat-resize")
	session.Values["ec2"] = ec2Cli
	return session.Save(r, w)
}

func (app *App) logout(w http.ResponseWriter, r *http.Request) {
	session, _ := app.store.Get(r, "yhat-resize")
	delete(session.Values, "ec2")
	session.Save(r, w)
}

// creds returns the EC2 credentials associated with the request session. If
// the session does not
func (app *App) creds(r *http.Request) (ec2Cli *ec2.EC2, ok bool) {
	session, _ := app.store.Get(r, "yhat-resize")
	ec2Cli, ok = session.Values["ec2"].(*ec2.EC2)
	if !ok {
		return nil, false
	}
	// github.com/gorilla/sessions uses encoding/gob to store data which does
	// not capture hidden fields. To recreate the hidden fields call the
	// constructor.
	return ec2.NewWithClient(ec2Cli.Auth, ec2Cli.Region, app.httpClient()), ok
}

// restrict a handler to only request which have been logged in
func (app *App) restrict(h http.Handler) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		if _, ok := app.creds(r); ok {
			h.ServeHTTP(w, r)
			return
		}

		if r.Method == "GET" {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
	return http.HandlerFunc(hf)
}
