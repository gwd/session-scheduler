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

var templates = template.Must(template.New("t").ParseGlob("templates/**/*.html"))

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

	data["CurrentUser"] = RequestUser(r)
	data["Flash"] = r.URL.Query().Get("flash")
	data["IsTestMode"] = Event.TestMode

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
