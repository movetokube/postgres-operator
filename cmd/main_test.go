package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestClientOptionsReadSecretsUncached(t *testing.T) {
	opts := clientOptions()
	if opts.Cache == nil {
		t.Fatal("client cache options are not set")
	}
	for _, obj := range opts.Cache.DisableFor {
		if _, ok := obj.(*corev1.Secret); ok {
			return
		}
	}
	t.Fatalf("Secrets are read through the cache: DisableFor = %v", opts.Cache.DisableFor)
}
