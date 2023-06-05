package utils

import "strings"

const INSTANCE_ANNOTATION = "postgres.db.movetokube.com/instance"

func MatchesInstanceAnnotation(annotationMap map[string]string, configuredInstance string) bool {
	if v, found := annotationMap[INSTANCE_ANNOTATION]; found {
		// Annotation Found, check if the value matches with what is configured
		return strings.EqualFold(v, configuredInstance)
	} else {
		// The annotation is not found, so we check if we have configured a filter
		return configuredInstance == ""
	}
}
