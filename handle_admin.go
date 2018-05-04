package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func HandleAdminConsole(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := RequestUser(r)

	if user.IsAdmin {
		RenderTemplate(w, r, "admin/console", map[string]interface{}{
			"User": user,
		})
		return
	}
}
