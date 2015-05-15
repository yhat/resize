package resize

import "net/http"

func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "index.html", nil)
}
