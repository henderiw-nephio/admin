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

package tenant

import (
	"context"
	"fmt"
	"strings"
	"time"

	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	"github.com/pkg/errors"
	adminv1alpha1 "github.com/yndd/admin/api/v1alpha1"
	"github.com/yndd/admin/pkg/appresource"
	kptfilev1 "github.com/yndd/admin/pkg/kptfile/v1"
	namespaceresrv1 "github.com/yndd/admin/pkg/namespace/v1"
	"github.com/yndd/admin/pkg/pkgrev"
	"github.com/yndd/admin/pkg/pkgutil"
	"github.com/yndd/admin/pkg/porch"
	nddv1 "github.com/yndd/ndd-runtime/apis/common/v1"
	"github.com/yndd/ndd-runtime/pkg/event"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-runtime/pkg/meta"
	"github.com/yndd/ndd-runtime/pkg/resource"
	"github.com/yndd/ndd-runtime/pkg/shared"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	finalizer = "admin.yndd.io/finalizer"
	// errors
	errGetCr        = "cannot get resource"
	errUpdateStatus = "cannot update status"

	reconcileFailed = "reconcile failed"
)

//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisionresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisionresources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions/approval,verbs=get;update;patch
//+kubebuilder:rbac:groups=admin.yndd.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=admin.yndd.io,resources=tenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=admin.yndd.io,resources=tenants/finalizers,verbs=update

// Setup adds a controller that reconciles infra.
func Setup(mgr ctrl.Manager, nddcopts *shared.NddControllerOptions) error {
	name := strings.Join([]string{adminv1alpha1.Group, strings.ToLower(adminv1alpha1.TenantKind)}, "/")

	c := resource.ClientApplicator{
		Client:     mgr.GetClient(),
		Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
	}

	r := &reconciler{
		client:          c,
		porchClient:     nddcopts.PorchClient,
		porchRESTClient: nddcopts.PorchRESTClient,
		pollInterval:    nddcopts.Poll,
		log:             nddcopts.Logger.WithValues("tenant-controller", name),
		record:          event.NewAPIRecorder(mgr.GetEventRecorderFor(name)),
		finalizer:       resource.NewAPIFinalizer(c, finalizer),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(nddcopts.Copts).
		For(&adminv1alpha1.Tenant{}).
		WithEventFilter(resource.IgnoreUpdateWithoutGenerationChangePredicate()).
		Complete(r)
}

// TargetAllocationReconciler reconciles a TargetAllocation object
type reconciler struct {
	client          client.Client
	porchClient     client.Client
	porchRESTClient rest.Interface
	pollInterval    time.Duration
	finalizer       *resource.APIFinalizer

	log    logging.Logger
	record event.Recorder
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	cr := &adminv1alpha1.Tenant{}
	if err := r.client.Get(ctx, req.NamespacedName, cr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug("Cannot get resource", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetCr)
	}

	// packageName is a combination of group, kind and resource name
	// version is ignored to be abale to support multiple versions
	// namespace tbd ?
	//packageName := filepath.Join(adminv1alpha1.Group, adminv1alpha1.TenantKind, cr.GetName())
	packageName := strings.Join([]string{adminv1alpha1.Group, adminv1alpha1.TenantKind, cr.GetName()}, "_")
	// initalize porch
	pr := pkgrev.New(&pkgrev.PrStruct{
		Porch: porch.New(&porch.Porch{
			PorchClient:     r.porchClient,
			PorchRESTClient: r.porchRESTClient,
			Log:             r.log,
		}),
		Namespace:   "default",
		RepoName:    cr.Spec.Properties.RepositoryRef,
		PackageName: packageName,
		Log:         r.log,
	})

	record := r.record.WithAnnotations("name", meta.GetExternalName(cr))

	// Finalizer TBD
	// this is done for transaction to ensure we add a finalizer
	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot add finalizer", "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if meta.WasDeleted(cr) {
		log = log.WithValues("deletion-timestamp", cr.GetDeletionTimestamp())

		if err := pr.Delete(ctx); err != nil {
			record.Event(cr, event.Warning(reconcileFailed, err))
			log.Debug("Cannot delete packagerevision", "error", err)
			cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unavailable())
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			record.Event(cr, event.Warning(reconcileFailed, err))
			log.Debug("Cannot remove finalizer", "error", err)
			cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unavailable())
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}

		log.Debug("Successfully deleted resource")
		return reconcile.Result{Requeue: false}, nil
	}

	sourceSubPkgResources, err := r.createSourcePackage(ctx, cr)
	if err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot create source sub package", "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	sourceSubPkgResources.Print("new sourceSubPkgResources")

	newSubPkgResources, err := r.createNewPackage(ctx, cr)
	if err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot populate schema", "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}
	newSubPkgResources.Print("new newSubPkgResources")

	newPkgResources, err := pkgutil.MergePackages(packageName, map[string]appresource.Resources{
		cr.Spec.Properties.Applications.PackageRef.PackageName: sourceSubPkgResources,
		"namespace": newSubPkgResources,
	})
	if err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot marge packages", "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	newPkgResources.Print("NEW PACKAGE")

	// get the package revision from porch. If it does not exist it will be created
	if err := pr.GetOrCreate(ctx, cr); err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot get or create package revisions", "packageName", packageName, "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	actualPkgResources, err := pr.GetResources(ctx, cr)
	if err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot get actual resources", "packageName", packageName, "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	actualPkgResources.Print("actual")

	// compare the new resources with the actual resources
	isEqual, err := newPkgResources.IsEqual(actualPkgResources.Get())
	if err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot handle isEqual", "packageName", packageName, "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}
	if !isEqual {
		log.Info("package is NOT up to date")
		// check if the pr is in draft state, if not create a new pr
		if pr.GetLifecycle() != porchv1alpha1.PackageRevisionLifecycleDraft {
			pkgRev := pr.Get()
			log.Info("new PR created", "pkgRev", pkgRev.GetName(), "revision", pkgRev.Spec.Revision)
			// create a new pr
			if err := pr.Create(ctx, cr); err != nil {
				record.Event(cr, event.Warning(reconcileFailed, err))
				log.Debug("Cannot create PR", "packageName", packageName, "error", err)
				cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
				return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
			}
		}
		if err := pr.Update(ctx, newPkgResources); err != nil {
			record.Event(cr, event.Warning(reconcileFailed, err))
			log.Debug("Cannot update PR", "packageName", packageName, "error", err)
			cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}
		// all good the package is up to date
	} else {
		log.Info("package is up to date")
	}

	if err := pr.Approve(ctx); err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot approve PR", "packageName", packageName, "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	log.Debug("reconcile success", "requeue-after", time.Now().Add(r.pollInterval))
	cr.SetConditions(nddv1.ReconcileSuccess(), nddv1.Available())
	return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) createNewPackage(ctx context.Context, cr *adminv1alpha1.Tenant) (appresource.Resources, error) {
	crName := cr.GetNamespacedName()
	log := r.log.WithValues("crName", crName)
	log.Debug("populateSchema")

	// build a fresh list of resources with a fresh look at the situation
	resources := appresource.New()
	// create a unique name per resource using group, version, kind, namespace, anme
	// gvkName := namespaceresrv1.GVKName(cr.GetNamespace(), cr.GetName())

	// populate the object and parse it to yaml RNode
	ns, err := namespaceresrv1.BuildNamespace(cr.GetName())
	if err != nil {
		return nil, err
	}
	// add the resource to the resource list
	resources.Add(ns)

	k, err := kptfilev1.BuildKptFile("namespace")
	if err != nil {
		return nil, err
	}
	// add the resource to the resource list
	resources.Add(k)

	return resources, nil
}

func (r *reconciler) createSourcePackage(ctx context.Context, cr *adminv1alpha1.Tenant) (appresource.Resources, error) {
	p := porch.New(&porch.Porch{
		PorchClient:     r.porchClient,
		PorchRESTClient: r.porchRESTClient,
		Log:             r.log,
	})

	sourcePR, err := p.GetPackageRevision(ctx, cr.Spec.Properties.Applications.PackageRef)
	if err != nil {
		return nil, err
	}

	r.log.Debug("Package reference found", "packageName", sourcePR.GetName())

	res, err := p.GetPackageRevisionResources(ctx, sourcePR.GetName())
	if err != nil {
		return nil, err
	}

	if res == nil {
		return nil, fmt.Errorf("no resources found")
	}

	pkgBuf, err := pkgutil.ResourcesToPackageBuffer(res.Spec.Resources)
	if err != nil {
		return nil, err
	}

	for _, rn := range pkgBuf.Nodes {
		r.log.Debug("pkbuf", "apiVersion", rn.GetApiVersion(), "kind", rn.GetKind(), "name", rn.GetName(), "namespace", rn.GetNamespace())
		if err := rn.SetNamespace(cr.GetName()); err != nil {
			return nil, err
		}
		data, err := yaml.Parse(cr.GetName())
		if err != nil {
			return nil, err
		}
		if rn.GetKind() == "RoleBinding" {
			if err := CopyValueToTarget(rn, data, &types.TargetSelector{FieldPaths: []string{"subjects.[kind=ServiceAccount].namespace"}}); err != nil {
				return nil, err
			}
		}
		if rn.GetKind() == "Deployment" {
			if err := CopyValueToTarget(rn, data, &types.TargetSelector{FieldPaths: []string{"spec.template.metadata.namespace"}}); err != nil {
				return nil, err
			}
		}
		if rn.GetKind() == "ConfigMap" && rn.GetName() == "kptfile.kpt.dev" {
			if err := CopyValueToTarget(rn, data, &types.TargetSelector{FieldPaths: []string{"data.name"}}); err != nil {
				return nil, err
			}
		}
	}
	r.log.Debug("content after mutation")
	resources := appresource.New()
	for _, rn := range pkgBuf.Nodes {
		r.log.Debug("pkbuf", "apiVersion", rn.GetApiVersion(), "kind", rn.GetKind(), "name", rn.GetName(), "namespace", rn.GetNamespace(), "string", rn.MustString())
		resources.Add(rn)
	}

	return resources, nil
}
