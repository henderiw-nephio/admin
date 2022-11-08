package porch

import (
	"context"
	"fmt"

	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	adminv1alpha1 "github.com/yndd/admin/api/v1alpha1"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Operations interface {
	GetPackageRevision(ctx context.Context, pkgRef adminv1alpha1.PackageRevisionReference) (*porchv1alpha1.PackageRevision, error)
	GetPackageRevisionResources(ctx context.Context, prName string) (*porchv1alpha1.PackageRevisionResources, error)
	GetClient() client.Client
	GetRestClient() rest.Interface
}

type Porch struct {
	PorchClient     client.Client
	PorchRESTClient rest.Interface
	Log             logging.Logger
}

func New(p *Porch) Operations {
	return &porch{
		porchClient:     p.PorchClient,
		porchRESTClient: p.PorchRESTClient,
		log:             p.Log,
	}
}

type porch struct {
	porchClient     client.Client
	porchRESTClient rest.Interface
	log             logging.Logger
}

func (r *porch) GetPackageRevision(ctx context.Context, pkgRef adminv1alpha1.PackageRevisionReference) (*porchv1alpha1.PackageRevision, error) {
	//log := r.log.WithValues("pkgref", pkgRef)
	prl := &porchv1alpha1.PackageRevisionList{}
	if err := r.porchClient.List(ctx, prl); err != nil {
		if client.IgnoreNotFound(err) != nil {
			r.log.Debug("cannot list package revisions", "error", err)
			return nil, fmt.Errorf("cannot list package revisions, error: %s", err.Error())
		}
	}
	for _, pr := range prl.Items {
		//log.Debug("porch get", "spec", pr.Spec)
		x := &adminv1alpha1.PackageRevisionReference{
			RepositoryName: pr.Spec.RepositoryName,
			PackageName:    pr.Spec.PackageName,
			Revision:       pr.Spec.Revision,
		}
		if x.IsFound(&pkgRef) {
			return &pr, nil
		}
	}
	return nil, fmt.Errorf("package reference not found")
}

func (r *porch) GetPackageRevisionResources(ctx context.Context, prName string) (*porchv1alpha1.PackageRevisionResources, error) {
	var prResources porchv1alpha1.PackageRevisionResources
	if err := r.porchClient.Get(ctx, client.ObjectKey{
		Namespace: "default",
		Name:      prName,
	}, &prResources); err != nil {
		return nil, err
	}
	return &prResources, nil
}

func (r *porch) GetClient() client.Client {
	return r.porchClient
}

func (r *porch) GetRestClient() rest.Interface {
	return r.porchRESTClient
}
