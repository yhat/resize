package resize

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/mitchellh/goamz/ec2"
)

func awsCreds(t *testing.T) (accessKey, secretKey string) {
	accessKey = os.Getenv("AWS_ACCESS_KEY")
	if accessKey == "" {
		t.Skip("AWS_ACCESS_KEY environment variable not setting, skipping test")
	}
	secretKey = os.Getenv("AWS_SECRET_KEY")
	if secretKey == "" {
		t.Skip("AWS_SECRET_KEY environment variable not setting, skipping test")
	}
	return
}

func TestBadLogin(t *testing.T) {
	app, err := NewApp("../static", "../templates", nil)
	if err != nil {
		t.Fatal(err)
	}
	hf := func(w http.ResponseWriter, r *http.Request) {
		err := app.login(w, r, "foo", "bar")
		switch err := err.(type) {
		case *ec2.Error:
		default:
			t.Errorf("expected error of type *ec2.Error got %s", reflect.TypeOf(err))
		}
		w.WriteHeader(http.StatusOK)
	}
	s := httptest.NewServer(http.HandlerFunc(hf))
	defer s.Close()
	resp, err := http.Get(s.URL + "/")
	if err != nil {
		t.Error(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("bad response from server %s", resp.Status)
	}
}

func TestLogin(t *testing.T) {
	accessKey, secretKey := awsCreds(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	app, err := NewApp("../static", "../templates", nil)
	if err != nil {
		t.Fatal(err)
	}
	hf := func(w http.ResponseWriter, r *http.Request) {
		err := app.login(w, r, accessKey, secretKey)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
	s := httptest.NewServer(http.HandlerFunc(hf))
	cli := &http.Client{Jar: jar}
	resp, err := cli.Get(s.URL + "/")
	s.Close()
	if err != nil {
		t.Error(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("bad response from server %s", resp.Status)
	}
	hf = func(w http.ResponseWriter, r *http.Request) {
		creds, ok := app.creds(w, r)
		if !ok {
			t.Errorf("no credentials found for client who logged in")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if creds.Auth.AccessKey != accessKey {
			t.Errorf("incorrect access key saved")
		}
		if creds.Auth.SecretKey != secretKey {
			t.Errorf("incorrect secret key saved")
		}
		_, err := creds.Instances(nil, nil)
		if err != nil {
			t.Errorf("could not get instances with stored creds: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}
	s = httptest.NewServer(http.HandlerFunc(hf))
	resp, err = cli.Get(s.URL + "/")
	s.Close()
	if err != nil {
		t.Error(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("bad response from server %s", resp.Status)
	}
}
