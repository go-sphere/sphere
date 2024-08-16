//go:build !tmplGen
package tmpl

import "text/template"

type List struct {
	Counter *template.Template
	Hello   *template.Template
	Test    *template.Template
}
