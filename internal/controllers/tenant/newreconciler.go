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
	"strings"
	"time"

	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	"github.com/pkg/errors"
	adminv1alpha1 "github.com/yndd/admin/api/v1alpha1"
	"github.com/yndd/admin/pkg/appresource"
	namespaceresrv1 "github.com/yndd/admin/pkg/namespace/v1"
	"github.com/yndd/admin/pkg/porch2"
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
)

const (
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
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(nddcopts.Copts).
		For(&adminv1alpha1.Tenant{}).
		Owns(&adminv1alpha1.Tenant{}).
		WithEventFilter(resource.IgnoreUpdateWithoutGenerationChangePredicate()).
		Complete(r)
}

// TargetAllocationReconciler reconciles a TargetAllocation object
type reconciler struct {
	client          client.Client
	porchClient     client.Client
	porchRESTClient rest.Interface
	pollInterval    time.Duration

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

	// Finalizer TBD

	record := r.record.WithAnnotations("name", meta.GetExternalName(cr))

	if meta.WasDeleted(cr) {
		log = log.WithValues("deletion-timestamp", cr.GetDeletionTimestamp())

		/*
			if err := r.RemoveFinalizer(ctx, cr); err != nil {
				// If this is the first time we encounter this issue we'll be
				// requeued implicitly when we update our status with the new error
				// condition. If not, we requeue explicitly, which will trigger
				// backoff.
				record.Event(cr, event.Warning(reasonCannotDeleteFInalizer, err))
				log.Debug("Cannot remove managed resource finalizer", "error", err)
				cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unavailable())
				return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
			}
		*/

		log.Debug("Successfully deleted resource")
		return reconcile.Result{Requeue: false}, nil

	}

	// packageName is a combination of group, kind and resource name
	// version is ignored to be abale to support multiple versions
	// namespace tbd ?
	//packageName := filepath.Join(adminv1alpha1.Group, adminv1alpha1.TenantKind, cr.GetName())
	packageName := strings.Join([]string{adminv1alpha1.Group, adminv1alpha1.TenantKind, cr.GetName()}, "_")

	// initalize porch
	p := porch2.New(&porch2.PrStruct{
		PorchClient:     r.porchClient,
		PorchRESTClient: r.porchRESTClient,
		Namespace:       "default",
		RepoName:        cr.Spec.Properties.RepositoryRef,
		PackageName:     packageName,
		Log:             r.log,
	})

	// get the package revision from porch. If it does not exist it will be created
	if err := p.GetOrCreate(ctx, cr); err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot get or create package revisions", "packageName", packageName, "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	newResources, err := r.populateSchema(ctx, cr)
	if err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot populate schema", "packageName", packageName, "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	newResources.Print("new")

	actualResources, err := p.GetResources(ctx, cr)
	if err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot get actual resources", "packageName", packageName, "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	actualResources.Print("actual")

	// compare the new resources with the actual resources
	isEqual, err := newResources.IsEqual(actualResources.Get())
	if err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot handle isEqual", "packageName", packageName, "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}
	if !isEqual {
		log.Info("package is NOT up to date")
		// check if the pr is in draft state, if not create a new pr
		if p.GetLifecycle() != porchv1alpha1.PackageRevisionLifecycleDraft {
			log.Info("new PR created")
			// create a new pr
			if err := p.Create(ctx, cr); err != nil {
				record.Event(cr, event.Warning(reconcileFailed, err))
				log.Debug("Cannot create PR", "packageName", packageName, "error", err)
				cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
				return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
			}
		}
		if err := p.Update(ctx, newResources); err != nil {
			record.Event(cr, event.Warning(reconcileFailed, err))
			log.Debug("Cannot update PR", "packageName", packageName, "error", err)
			cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}
		// all good the package is up to date
	} else {
		log.Info("package is up to date")
	}

	if err := p.Approve(ctx); err != nil {
		record.Event(cr, event.Warning(reconcileFailed, err))
		log.Debug("Cannot approve PR", "packageName", packageName, "error", err)
		cr.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	log.Debug("reconcile success", "requeue-after", time.Now().Add(r.pollInterval))
	cr.SetConditions(nddv1.ReconcileSuccess(), nddv1.Available())
	return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) populateSchema(ctx context.Context, cr *adminv1alpha1.Tenant) (appresource.Resources, error) {
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

	return resources, nil
}
