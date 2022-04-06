package utils

import (
	"testing"
)

func TestProcessAnnotationWithNoCorrectKeyAndFilterDefined(t *testing.T) {
	annotations := map[string]string{
		"invalidkey": "value",
	}
	response := MatchesInstanceAnnotation(annotations, "value")
	if response {
		t.Fail()
	}
}

func TestProcessAnnotationWithCorrectKeyAndFilterDefined(t *testing.T) {
	annotations := map[string]string{
		INSTANCE_ANNOTATION: "value",
	}
	response := MatchesInstanceAnnotation(annotations, "value")
	if !response {
		t.Fail()
	}
}

func TestProcessAnnotationWithNilAnnotationsAndFilterDefined(t *testing.T) {
	response := MatchesInstanceAnnotation(nil, "value")
	if response {
		t.Fail()
	}
}

func TestProcessAnnotationWithNilAnnotationAndNoFilterDefined(t *testing.T) {
	response := MatchesInstanceAnnotation(nil, "")
	if response {
		t.Fail()
	}
}

func TestProcessAnnotationWithNoCorrectKeyAndNoFilterDefined(t *testing.T) {
	annotations := map[string]string{
		"invalidkey": "value",
	}
	response := MatchesInstanceAnnotation(annotations, "")
	if response {
		t.Fail()
	}
}

func TestProcessAnnotationWithCorrectKeyAndNoFilterDefined(t *testing.T) {
	annotations := map[string]string{
		INSTANCE_ANNOTATION: "value",
	}
	response := MatchesInstanceAnnotation(annotations, "")
	if response {
		t.Fail()
	}
}
