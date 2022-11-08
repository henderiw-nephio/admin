/*
Copyright 2021 NDD.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"strings"

	nddv1 "github.com/yndd/ndd-runtime/apis/common/v1"
	//"github.com/yndd/ndd_runtime/pkg/resource"
	//"sigs.k8s.io/controller-runtime/pkg/client"
)

// +k8s:deepcopy-gen=false

// GetCondition of this Network Node.
func (x *Tenant) GetCondition(ct nddv1.ConditionKind) nddv1.Condition {
	return x.Status.GetCondition(ct)
}

// SetConditions of the Network Node.
func (x *Tenant) SetConditions(c ...nddv1.Condition) {
	x.Status.SetConditions(c...)
}

func (x *Tenant) SetHealthConditions(c nddv1.HealthConditionedStatus) {
	x.Status.Health = c
}

func (x *Tenant) GetDeletionPolicy() nddv1.DeletionPolicy {
	return x.Spec.Lifecycle.DeletionPolicy
}

func (x *Tenant) SetDeletionPolicy(c nddv1.DeletionPolicy) {
	x.Spec.Lifecycle.DeletionPolicy = c
}

func (x *Tenant) GetDeploymentPolicy() nddv1.DeploymentPolicy {
	return x.Spec.Lifecycle.DeploymentPolicy
}

func (x *Tenant) SetDeploymentPolicy(c nddv1.DeploymentPolicy) {
	x.Spec.Lifecycle.DeploymentPolicy = c
}

func (x *Tenant) GetTargetReference() *nddv1.Reference {
	return x.Spec.TargetReference
}

func (x *Tenant) SetTargetReference(p *nddv1.Reference) {
	x.Spec.TargetReference = p
}

func (x *Tenant) GetRootPaths() []string {
	return x.Status.RootPaths
}

func (x *Tenant) SetRootPaths(rootPaths []string) {
	x.Status.RootPaths = rootPaths
}

func (x *Tenant) GetNamespacedName() string {
	return strings.Join([]string{x.Namespace, x.Name}, "/")
}

func (x *PackageRevisionReference) IsFound(selector *PackageRevisionReference) bool {
	if selector == nil {
		return true
	}
	if len(selector.RepositoryName) > 0 {
		if x.RepositoryName != selector.RepositoryName {
			return false
		}
	}

	if len(selector.PackageName) > 0 {
		if x.PackageName != selector.PackageName {
			return false
		}
	}

	if len(selector.Revision) > 0 {
		if x.Revision != selector.Revision {
			return false
		}
	}
	return true
}
