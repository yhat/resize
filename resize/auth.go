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
	ec2Cli := ec2.New(aws.Auth{
		AccessKey: accessKeyID,
		SecretKey: secretKey,
	}, defaultRegion)

	_, err := ec2Cli.Instances(nil, nil)
	if err != nil {
		return err
	}

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

func (app *App) creds(w http.ResponseWriter, r *http.Request) (*ec2.EC2, bool) {
	session, _ := app.store.Get(r, "yhat-resize")
	ec2Cli, ok := session.Values["ec2"].(*ec2.EC2)
	if !ok {
		return nil, false
	}
	return ec2.New(ec2Cli.Auth, ec2Cli.Region), ok
}

// restrict a handler to only request which have been logged in
func (app *App) restrict(h http.Handler) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		if _, ok := app.creds(w, r); ok {
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
