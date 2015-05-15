package resize

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilteTemplates(t *testing.T) {
	app := &App{}
	tmplDir := "../templates"
	err := app.CompileTemplates(tmplDir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := ioutil.ReadDir(tmplDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := filepath.Base(file.Name())
		if !strings.HasSuffix(name, ".html") {
			continue
		}
		tmpl, ok := app.tmpl[name]
		if !ok {
			t.Errorf("no template named %s", name)
			continue
		}
		err = tmpl.ExecuteTemplate(ioutil.Discard, "base.html", nil)
		if err != nil {
			t.Errorf("executing template %s %v", name, err)
		}
	}
}
