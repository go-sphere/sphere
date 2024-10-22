package bot

import (
	"strings"
	textTmpl "text/template"
)

var counterTemplate *textTmpl.Template
var startTemplate *textTmpl.Template

func init() {
	counterTemplate, _ = textTmpl.New("counter").Parse("Counter: {{.}}")
	startTemplate, _ = textTmpl.New("start").Parse("Hello {{.}}, welcome to the bot")
}

func renderText(tmpl *textTmpl.Template, data interface{}) (string, error) {
	var sb strings.Builder
	err := tmpl.Execute(&sb, data)
	return sb.String(), err
}
