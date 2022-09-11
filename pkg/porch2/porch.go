package porch2

import (
	"context"
	"fmt"

	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	"github.com/yndd/admin/pkg/appresource"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type PackageRevision interface {
	GetOrCreate(ctx context.Context, mr resource.Managed) error
	Create(ctx context.Context, mr resource.Managed) error
	GetResources(ctx context.Context, mr resource.Managed) (appresource.Resources, error)
	Update(ctx context.Context, r appresource.Resources) error
	GetLifecycle() porchv1alpha1.PackageRevisionLifecycle
	Approve(ctx context.Context) error
}

type PrStruct struct {
	PorchClient     client.Client
	PorchRESTClient rest.Interface
	Namespace       string
	RepoName        string
	PackageName     string
	Log             logging.Logger
}

func New(prs *PrStruct) PackageRevision {
	return &packageRevision{
		porchClient:     prs.PorchClient,
		porchRESTClient: prs.PorchRESTClient,
		namespace:       prs.Namespace,
		repoName:        prs.RepoName,
		packageName:     prs.PackageName,
		log:             prs.Log,
	}
}

type packageRevision struct {
	porchClient     client.Client
	porchRESTClient rest.Interface
	namespace       string
	repoName        string
	packageName     string
	log             logging.Logger

	pr            *porchv1alpha1.PackageRevision
	prr           *porchv1alpha1.PackageRevisionResources
	localKptFiles []*yaml.RNode
}

func (r *packageRevision) Create(ctx context.Context, mr resource.Managed) error {
	pr := buildPackageRevision(r.namespace, r.repoName, r.packageName, getNewRevision(r.pr.Spec.Revision), mr)
	err := r.porchClient.Create(ctx, pr)
	if err != nil {
		r.log.Debug("cannot create package revision", "error", err)
		return err
	}

	r.log.Info("PackageRevision create", "pr", pr)
	r.pr = pr

	// load the new package revision resources related to the package revision
	return r.loadPackageRevisionResources(ctx)
}

func (r *packageRevision) GetOrCreate(ctx context.Context, mr resource.Managed) error {
	prl := &porchv1alpha1.PackageRevisionList{}
	if err := r.porchClient.List(ctx, prl); err != nil {
		if client.IgnoreNotFound(err) != nil {
			r.log.Debug("cannot list package revision", "error", err)
			return err
		}
	}

	// create a list of all revisions that match the packageName
	pl := []*porchv1alpha1.PackageRevision{}
	for _, pr := range prl.Items {
		//r.log.Info("PackageRevision list matches the options", "prName", pr.Name, "prPackageName", pr.Spec.PackageName, "labels", pr.Labels)
		if pr.Spec.PackageName == r.packageName {
			//r.log.Info("PackageRevision match", "prPackageName", pr.Spec.PackageName, "packageName", r.packageName)
			pl = append(pl, pr.DeepCopy())
		}
	}

	lastPr := &porchv1alpha1.PackageRevision{}
	lastrevision := uint64(0)
	for _, pr := range pl {
		//r.log.Info("PackageRevision retained", "prPackageName", pr.Spec.PackageName, "repoName", pr.Spec.RepositoryName, "revision", pr.Spec.Revision)
		// get the latest revision PR
		if getRevisionNumber(pr.Spec.Revision) > lastrevision {
			lastPr = pr
		}
	}

	if lastPr.Name == "" {
		// package revision not found -> create a new pr
		pr := buildPackageRevision(r.namespace, r.repoName, r.packageName, "v1", mr)
		err := r.porchClient.Create(ctx, pr)
		if err != nil {
			r.log.Debug("cannot create package revision", "error", err)
			return err
		}

		r.log.Info("PackageRevision create", "pr", pr)
		r.pr = pr
		return r.loadPackageRevisionResources(ctx)
	}
	r.pr = lastPr
	r.log.Info("packageRevision selected", "Name", r.pr.Name, "PackageName", r.pr.Spec.PackageName, "Revision", r.pr.Spec.Revision)
	// load the package revision resources related to the selected pr
	return r.loadPackageRevisionResources(ctx)
}

func (r *packageRevision) GetResources(ctx context.Context, mr resource.Managed) (appresource.Resources, error) {
	// keep track of the package resources, so we can use it to update later on
	pkgBuf, err := resourcesToPackageBuffer(r.prr.Spec.Resources)
	if err != nil {
		r.log.Debug("could not parse resources", "err", err)
		return nil, err
	}

	// create a copy of the actual resources
	actualResources := appresource.New()
	for _, rn := range pkgBuf.Nodes {
		//r.log.Debug("actualResources", "apiVersion", rn.GetApiVersion(), "kind", rn.GetKind(), "name", rn.GetName(), "annotations", rn.GetAnnotations())
		ignore := false
		if v, ok := rn.GetAnnotations()["config.kubernetes.io/local-config"]; ok {
			if v == "true" {
				ignore = true
			}
		}
		if !ignore {
			actualResources.Add(rn)
		}

	}
	return actualResources, nil
}

func (r *packageRevision) GetLifecycle() porchv1alpha1.PackageRevisionLifecycle {
	return r.pr.Spec.Lifecycle
}

func (r *packageRevision) Update(ctx context.Context, resources appresource.Resources) error {
	newResources, err := createUpdatedResource(r.prr.Spec.Resources, &kio.PackageBuffer{
		Nodes: resources.Get(),
	}, r.localKptFiles)
	if err != nil {
		r.log.Debug("could create new resource map", "err", err)
		return err
	}
	r.prr.Spec.Resources = newResources
	if err := r.porchClient.Update(ctx, r.prr); err != nil {
		r.log.Debug("could not save updated resources", "err", err)
		return err
	}

	r.log.Info("updated PackageRevisionResources", "resources", resources)
	return nil
}

func (r *packageRevision) Approve(ctx context.Context) error {
	switch r.GetLifecycle() {
	case porchv1alpha1.PackageRevisionLifecycleDraft:
		if err := r.porchClient.Get(ctx, client.ObjectKey{
			Namespace: r.pr.Namespace,
			Name:      r.pr.Name,
		}, r.pr); err != nil {
			r.log.Debug("could NOT get package revision", "err", err)
			return err
		}

		r.pr.Spec.Lifecycle = porchv1alpha1.PackageRevisionLifecycleProposed
		if err := r.porchClient.Update(ctx, r.pr); err != nil {
			r.log.Debug("could NOT update package revision lifecycle to proposed", "err", err)
			return err
		}

		if err := updatePackageRevisionApproval(ctx, r.porchRESTClient, client.ObjectKey{
			Namespace: r.pr.Namespace,
			Name:      r.pr.Name,
		}, porchv1alpha1.PackageRevisionLifecyclePublished); err != nil {
			r.log.Debug("could not approve the packagerevision", "err", err)
			return err
		}

	case porchv1alpha1.PackageRevisionLifecycleProposed:
		if err := updatePackageRevisionApproval(ctx, r.porchRESTClient, client.ObjectKey{
			Namespace: r.pr.Namespace,
			Name:      r.pr.Name,
		}, porchv1alpha1.PackageRevisionLifecyclePublished); err != nil {
			r.log.Debug("could not approve the packagerevision", "err", err)
			return err
		}
	case porchv1alpha1.PackageRevisionLifecyclePublished:
	default:
		return fmt.Errorf("unknown lifecycle status: %s", r.GetLifecycle())
	}
	return nil
}

func (r *packageRevision) loadPackageRevisionResources(ctx context.Context) error {
	var prResources porchv1alpha1.PackageRevisionResources
	if err := r.porchClient.Get(ctx, client.ObjectKey{
		Namespace: r.pr.Namespace,
		Name:      r.pr.Name,
	}, &prResources); err != nil {
		return err
	}
	r.prr = &prResources

	// keep a copy of the local kpt files since they are not supplied in the newresources
	r.getLocalKptFiles()
	return nil
}

func (r *packageRevision) getLocalKptFiles() error {
	pkgBuf, err := resourcesToPackageBuffer(r.prr.Spec.Resources)
	if err != nil {
		r.log.Debug("could not parse resources", "err", err)
		return err
	}
	r.localKptFiles = []*yaml.RNode{}
	for _, rn := range pkgBuf.Nodes {
		if v, ok := rn.GetAnnotations()["config.kubernetes.io/local-config"]; ok {
			if v == "true" {
				r.localKptFiles = append(r.localKptFiles, rn)
			}
		}
	}
	return nil
}
