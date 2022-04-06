package utils

import "strings"

const INSTANCE_ANNOTATION = "postgres.db.movetokube.com/instance"

func MatchesInstanceAnnotation(annotationMap map[string]string, configuredInstance string) bool {
	if v, found := annotationMap[INSTANCE_ANNOTATION]; found {
		return strings.EqualFold(v, configuredInstance)
	}
	return false
}
