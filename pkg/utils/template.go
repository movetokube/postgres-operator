package utils

import (
	"bytes"
	"fmt"
	"net/url"
	"text/template"
)

type TemplateContext struct {
	Host     string
	Role     string
	Database string
	Password string
	Hostname string // Hostname is different from Host as it does not contain the port number.
	Port     string
	UriArgs  string
}

func RenderTemplate(data map[string]string, tc TemplateContext) (map[string][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var out = make(map[string][]byte, len(data))
	for key, templ := range data {
		tmplObj := template.New("")
		tmplObj.Funcs(template.FuncMap{
			"mergeUriArgs": func(uriArgs string) (string, error) {
				inputArgs, err := url.ParseQuery(uriArgs)
				if err != nil {
					return uriArgs, fmt.Errorf("unable to parse input uri args: %w", err)
				}
				pgArgs, err := url.ParseQuery(tc.UriArgs)
				if err != nil {
					return uriArgs, fmt.Errorf("unable to parse pg uri args: %w", err)
				}
				for argName, values := range pgArgs {
					inputArgs.Set(argName, values[0])
				}
				return inputArgs.Encode(), nil
			},
		})
		parsed, err := tmplObj.Parse(templ)
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
