{{ range .Errors }}
func {{ .Name }}{{ .CamelValue }}({{ if not .HasMessage }}msg string,{{ end }} errs ...error) error {
	if len(errs) == 0 {
        errs = append(errs, errors.New("{{ .Reason }}"))
    }
	return statuserr.NewError(
	    {{ .Status }},
	    {{ .Code }},
	    {{ if .HasMessage }} "{{ .Message }}" {{ else }} msg {{ end }},
        errors.Join(errs...),
	)
}
{{- end }}