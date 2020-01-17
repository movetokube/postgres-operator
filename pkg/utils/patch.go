package utils

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Patch(cl client.Client, ctx context.Context, before runtime.Object, after runtime.Object) error {
	resourcePatch := client.MergeFrom(before.DeepCopyObject())
	statusPatch := client.MergeFrom(before.DeepCopyObject())
	// Convert resources to unstructured for easier comparison
	beforeUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(before)
	if err != nil {
		return err
	}
	afterUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(after)
	if err != nil {
		return err
	}

	beforeHasStatus := false
	afterHasStatus := false
	// Attempt to remove the status for easier comparison
	beforeStatus, ok, err := unstructured.NestedFieldCopy(beforeUnstructured, "status")
	if err != nil {
		return err
	}
	if ok {
		beforeHasStatus = true
		// Remove status from object so they can patched separately
		unstructured.RemoveNestedField(beforeUnstructured, "status")
	}
	afterStatus, ok, err := unstructured.NestedFieldCopy(afterUnstructured, "status")
	if err != nil {
		return err
	}
	if ok {
		afterHasStatus = true
		// Remove status from object so they can patched separately
		unstructured.RemoveNestedField(afterUnstructured, "status")
	}

	var errs []error

	// Check if there's any difference to patch
	if !reflect.DeepEqual(beforeUnstructured, afterUnstructured) {
		err = cl.Patch(ctx, after.DeepCopyObject(), resourcePatch)
		if err != nil {
			errs = append(errs, err)
		}
	}

	// Check if there's any difference in status to patch
	if (beforeHasStatus || afterHasStatus) && !reflect.DeepEqual(beforeStatus, afterStatus) {
		err = cl.Status().Patch(ctx, after.DeepCopyObject(), statusPatch)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}
