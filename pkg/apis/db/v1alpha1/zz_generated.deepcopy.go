//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by operator-sdk. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Postgres) DeepCopyInto(out *Postgres) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Postgres.
func (in *Postgres) DeepCopy() *Postgres {
	if in == nil {
		return nil
	}
	out := new(Postgres)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Postgres) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PostgresList) DeepCopyInto(out *PostgresList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Postgres, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PostgresList.
func (in *PostgresList) DeepCopy() *PostgresList {
	if in == nil {
		return nil
	}
	out := new(PostgresList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PostgresList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PostgresRoles) DeepCopyInto(out *PostgresRoles) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PostgresRoles.
func (in *PostgresRoles) DeepCopy() *PostgresRoles {
	if in == nil {
		return nil
	}
	out := new(PostgresRoles)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PostgresSpec) DeepCopyInto(out *PostgresSpec) {
	*out = *in
	if in.Schemas != nil {
		in, out := &in.Schemas, &out.Schemas
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Extensions != nil {
		in, out := &in.Extensions, &out.Extensions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PostgresSpec.
func (in *PostgresSpec) DeepCopy() *PostgresSpec {
	if in == nil {
		return nil
	}
	out := new(PostgresSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PostgresStatus) DeepCopyInto(out *PostgresStatus) {
	*out = *in
	out.Roles = in.Roles
	if in.Schemas != nil {
		in, out := &in.Schemas, &out.Schemas
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Extensions != nil {
		in, out := &in.Extensions, &out.Extensions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PostgresStatus.
func (in *PostgresStatus) DeepCopy() *PostgresStatus {
	if in == nil {
		return nil
	}
	out := new(PostgresStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PostgresUser) DeepCopyInto(out *PostgresUser) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PostgresUser.
func (in *PostgresUser) DeepCopy() *PostgresUser {
	if in == nil {
		return nil
	}
	out := new(PostgresUser)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PostgresUser) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PostgresUserList) DeepCopyInto(out *PostgresUserList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PostgresUser, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PostgresUserList.
func (in *PostgresUserList) DeepCopy() *PostgresUserList {
	if in == nil {
		return nil
	}
	out := new(PostgresUserList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PostgresUserList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PostgresUserSpec) DeepCopyInto(out *PostgresUserSpec) {
	*out = *in
	if in.SecretTemplate != nil {
		in, out := &in.SecretTemplate, &out.SecretTemplate
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PostgresUserSpec.
func (in *PostgresUserSpec) DeepCopy() *PostgresUserSpec {
	if in == nil {
		return nil
	}
	out := new(PostgresUserSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PostgresUserStatus) DeepCopyInto(out *PostgresUserStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PostgresUserStatus.
func (in *PostgresUserStatus) DeepCopy() *PostgresUserStatus {
	if in == nil {
		return nil
	}
	out := new(PostgresUserStatus)
	in.DeepCopyInto(out)
	return out
}