package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	"github.com/microcosm-cc/bluemonday"

	// blackfriday v2 has a bug where it doesn't make paragraphs
	// properly.  github_flavored_markdown uses blackfriday v1, which
	// doesn't seem to suffer from this problem.  Leave the code
	// present but commented out, in case we can get it fixed.  (It's
	// more fully-featured.)

	// "gopkg.in/russross/blackfriday.v2"
	"github.com/shurcooL/github_flavored_markdown"
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
	data["IsTestMode"] = Event.TestMode
	data["IsActive"] = Event.Active
	data["ShowToolbar"] = Event.Active || (cur != nil && cur.IsAdmin)

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
	//unsafe := blackfriday.Run([]byte(input), blackfriday.WithExtensions(blackfriday.HardLineBreak))
	//unsafe := blackfriday.Run([]byte(input))
	unsafe := github_flavored_markdown.Markdown([]byte(input))
	return template.HTML(sanitizer.SanitizeBytes(unsafe))
}
