package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/microcosm-cc/bluemonday"

	"github.com/russross/blackfriday/v2"
)

var layoutFuncs = template.FuncMap{
	"yield": func() (string, error) {
		return "", fmt.Errorf("yield called inappropriately")
	},
}
var layout = template.Must(
	template.
		New("layout.html").
		Funcs(layoutFuncs).
		ParseFiles("templates/layout.html"),
)

var utilFuncs = template.FuncMap{
	"dict": func(values ...interface{}) (map[string]interface{}, error) {
		if len(values)%2 != 0 {
			return nil, fmt.Errorf("invalid dict call")
		}
		dict := make(map[string]interface{}, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict keys must be strings")
			}
			dict[key] = values[i+1]
		}
		return dict, nil
	},
}
var templates = template.Must(
	template.New("t").
		Funcs(utilFuncs).
		ParseGlob("templates/**/*.html"))

var errorTemplate = `
<html>
	<body>
		<h1>Error rendering template %s</h1>
		<p>%s</p>
	</body>
</html>
`

func RenderTemplate(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}

	cur := RequestUser(r)

	data["CurrentUser"] = cur
	data["Flash"] = r.URL.Query().Get("flash")
	data["IsTestMode"] = kvs.GetBoolDef(FlagTestMode)
	data["IsWebsiteActive"] = kvs.GetBoolDef(FlagActive)
	data["IsScheduleActive"] = kvs.GetBoolDef(FlagScheduleActive)
	data["IsVcodeSent"] = kvs.GetBoolDef(FlagVerificationCodeSent)
	data["RequireVerification"] = kvs.GetBoolDef(FlagRequireVerification)
	data["ShowToolbar"] = kvs.GetBoolDef(FlagActive) || (cur != nil && cur.IsAdmin)

	funcs := template.FuncMap{
		"yield": func() (template.HTML, error) {
			buf := bytes.NewBuffer(nil)
			err := templates.ExecuteTemplate(buf, name, data)
			return template.HTML(buf.String()), err
		},
	}

	layoutClone, _ := layout.Clone()
	layoutClone.Funcs(funcs)
	err := layoutClone.Execute(w, data)

	if err != nil {
		http.Error(
			w,
			fmt.Sprintf(errorTemplate, name, err),
			http.StatusInternalServerError,
		)
	}
}

var sanitizer = bluemonday.UGCPolicy()

func ProcessText(input string) template.HTML {
	// HTML standard requires CRLF for submitted text, but blackfriday
	// for some reason only handles LF ATM; see
	// https://github.com/russross/blackfriday/issues/423
	// Brute-force the problem.
	unix := strings.ReplaceAll(input, "\r\n", "\n")

	unsafe := blackfriday.Run([]byte(unix))

	html := template.HTML(sanitizer.SanitizeBytes(unsafe))

	return html
}
