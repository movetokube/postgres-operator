package utils

import (
	"bytes"
	"fmt"
	"text/template"
)

type TemplateContext struct {
	Host     string
	Role     string
	Database string
	Password string
	Hostname string // Hostname is different from Host as it does not contain the port number.
	Port     string
}

func RenderTemplate(data map[string]string, tc TemplateContext) (map[string][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var out = make(map[string][]byte, len(data))
	for key, templ := range data {
		parsed, err := template.New("").Parse(templ)
		if err != nil {
			return nil, fmt.Errorf("parse template %q: %w", key, err)
		}
		var content bytes.Buffer
		if err := parsed.Execute(&content, tc); err != nil {
			return nil, fmt.Errorf("execute template %q: %w", key, err)
		}
		out[key] = content.Bytes()
	}
	return out, nil
}
