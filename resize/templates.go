package resize

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/mitchellh/goamz/aws"
)

// CompileTemplates parses a template directory
func (app *App) compileTemplates(tmplDir string) error {
	tmpl, err := compileTemplates(tmplDir)
	if err != nil {
		return err
	}
	app.tmpl = tmpl
	return nil
}

func compileTemplates(tmplDir string) (map[string]*template.Template, error) {
	join := filepath.Join

	includes := join(tmplDir, "includes")
	layouts := join(tmplDir, "layouts")

	var tmpl *template.Template
	var err error
	tmpl, err = template.ParseGlob(join(includes, "*.html"))
	if err != nil {
		return nil, err
	}
	if _, err = tmpl.ParseGlob(join(layouts, "*.html")); err != nil {
		return nil, err
	}

	files, err := ioutil.ReadDir(tmplDir)
	if err != nil {
		return nil, err
	}
	m := make(map[string]*template.Template)

	for _, info := range files {
		name := info.Name()
		if info.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		t, err := tmpl.Clone()
		if err != nil {
			return nil, err
		}
		_, err = t.ParseFiles(join(tmplDir, name))
		if err != nil {
			return nil, err
		}
		m[name] = t
	}
	return m, nil
}

// Render renders a template to the ResponseWriter with a 200 status code.
func (app *App) render(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	ec2Cli, ok := app.creds(r)
	if ok {
		// if the user is logged in display the list of available regions
		regions := []struct {
			Name     string
			Selected bool
		}{
			{aws.APNortheast.Name, false},
			{aws.APSoutheast.Name, false},
			{aws.APSoutheast2.Name, false},
			{aws.EUWest.Name, false},
			{aws.EUCentral.Name, false},
			{aws.USEast.Name, false},
			{aws.USWest.Name, false},
			{aws.USWest2.Name, false},
			{aws.SAEast.Name, false},
			{aws.USGovWest.Name, false},
			{aws.CNNorth.Name, false},
		}
		for i, r := range regions {
			if r.Name == ec2Cli.Region.Name {
				regions[i].Selected = true
				break
			}
		}
		if data == nil {
			data = make(map[string]interface{})
		}
		data["Regions"] = regions
	}
	app.renderStatus(w, r, name, data, http.StatusOK)
}

// Render500 renders the 500.html template with the error message displayed to
// the user.
func (app *App) render500(w http.ResponseWriter, r *http.Request, err error) {
	data := map[string]string{
		"Error": err.Error(),
	}
	app.renderStatus(w, r, "500.html", data, http.StatusInternalServerError)
}

// Render404 renders the 404.html template to the user.
func (app *App) render404(w http.ResponseWriter, r *http.Request) {
	app.Logf("%s not found", r.RequestURI)
	app.renderStatus(w, r, "404.html", nil, http.StatusNotFound)
}

func (app *App) renderStatus(
	w http.ResponseWriter,
	r *http.Request,
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
