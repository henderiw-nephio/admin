package pkgrev

import (
	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	adminv1alpha1 "github.com/yndd/admin/api/v1alpha1"
	"github.com/yndd/ndd-runtime/pkg/meta"
	"github.com/yndd/ndd-runtime/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildPackageRevision(namespace, repositoryRef, packageName, revision string, mr resource.Object) *porchv1alpha1.PackageRevision {
	return &porchv1alpha1.PackageRevision{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageRevision",
			APIVersion: porchv1alpha1.SchemeGroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			// Name is dynamically allocated
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(mr, adminv1alpha1.TenantGroupVersionKind))},
		},
		Spec: porchv1alpha1.PackageRevisionSpec{
			PackageName:    packageName,
			Revision:       revision,
			RepositoryName: repositoryRef,
			Tasks: []porchv1alpha1.Task{
				{
					Type: porchv1alpha1.TaskTypeInit,
					Init: &porchv1alpha1.PackageInitTaskSpec{
						Description: packageName,
					},
				},
			},
		},
	}
}
