/*
Copyright 2022.

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
	"reflect"
	"strings"

	nddv1 "github.com/yndd/ndd-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TenantProperties struct {
	// OwnerRef identifies the entity that is responsible for the tenant.
	// The ownerRef is a resource that is typically used for resellers and integrators.
	OwnerRef string `json:"ownerRef,omitempty"`
	// OrganizationRef identifies the organization the tenant belongs to
	OrganizationRef string `json:"organizationRef,omitempty"`
	// Deployment identifies the deployment the tenant belongs to
	Deployment string `json:"deployment,omitempty"`
	// Region identifies the region the tenant belongs to
	Region string `json:"region,omitempty"`
	// AvailabilityZone identifies the az the tenant belongs to
	AvailabilityZone string `json:"availabilityZone,omitempty"`
	// ClusterRef identifies the cluster where the tenant operator executes its tasks
	//ClusterRef string `json:"clusterRef"`
	// RepositoryRef identifies the repo where the tenant information is consolidated
	RepositoryRef string `json:"repositoryRef"`
	// Applications identify the application information of the tenant
	Applications *TenantApplications `json:"applications"`
	// Clusters identify the
	Clusters []*TenantCluster `json:"clusters"`
}

type TenantApplications struct {
	// Application identifies if an application should be installed or not
	Installation map[string]bool `json:"install,omitempty"`
	// PackageRef identifies the package revision to deploy
	PackageRef PackageRevisionReference `json:"packageRef"`
}

// PackageRevisionReference is used to reference a particular package revision.
type PackageRevisionReference struct {
	// Namespace is the namespace for both the repository and package revision
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Repository is the name of the repository containing the package
	RepositoryName string `json:"repository"`

	// PackageName is the name of the package for the revision
	PackageName string `json:"packageName"`

	// Revision is the specific version number of the revision of the package
	Revision string `json:"revision"`
}

type TenantCluster struct {
	// ClusterRef identifies a cluster this tenant operates on
	ClusterRef string `json:"clusterRef"`
	// RepositoryRef identifies the repo where the tenant information is consolidated
	// The key of the map identifies the user group and the value the repository name
	RepositoryRef map[string]string `json:"repositoryRef"`
}

// TenantSpec defines the desired state of Tenant
type TenantSpec struct {
	nddv1.ResourceSpec `json:",inline"`
	// Properties define the properties of the Tenant
	Properties *TenantProperties `json:"properties,omitempty"`
}

// TenantStatus defines the observed state of Tenant
type TenantStatus struct {
	nddv1.ResourceStatus `json:",inline"`
	// Namespace that was allocted
	Namespace string `json:"namespace,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNC",type="string",JSONPath=".status.conditions[?(@.kind=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.kind=='Ready')].status"
// +kubebuilder:printcolumn:name="OWNER",type="string",JSONPath=".spec.properties.ownerRef",description="owner of the tenant'"
// +kubebuilder:printcolumn:name="ORG",type="string",JSONPath=".spec.properties.organaizationRef",description="organization of the tenant'"
// +kubebuilder:printcolumn:name="DEPL",type="string",JSONPath=".spec.properties.organaizationRef",description="deployemnt the tenant is deployed in'"
// +kubebuilder:printcolumn:name="REGION",type="string",JSONPath=".spec.properties.organaizationRef",description="region the tenant is deployed in'"
// +kubebuilder:printcolumn:name="AZ",type="string",JSONPath=".spec.properties.availabilityZone",description="availabilityZone the tenant is deployed in'"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories={yndd,admin}

// Tenant is the Schema for the tenants API
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TenantList contains a list of Tenant
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}

// tenant type metadata.
var (
	TenantKind             = reflect.TypeOf(Tenant{}).Name()
	TenantGroupKind        = schema.GroupKind{Group: Group, Kind: TenantKind}.String()
	TenantKindAPIVersion   = TenantKind + "." + GroupVersion.String()
	TenantGroupVersionKind = GroupVersion.WithKind(TenantKind)

	TenantPkgRevLabelKey = strings.ToLower(TenantGroupKind) + "/" + "PackageRevision"
)
