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

/*
const (
	errUnexpectedResource = "unexpected object"
	//errGetK8sResource     = "cannot get organization resource"
)
*/

//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisionresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisionresources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions/approval,verbs=get;update;patch
//+kubebuilder:rbac:groups=admin.yndd.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=admin.yndd.io,resources=tenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=admin.yndd.io,resources=tenants/finalizers,verbs=update

/*
// Setup adds a controller that reconciles infra.
func Setup(mgr ctrl.Manager, nddcopts *shared.NddControllerOptions) error {
	name := strings.Join([]string{adminv1alpha1.Group, strings.ToLower(adminv1alpha1.TenantKind)}, "/")

	c := resource.ClientApplicator{
		Client:     mgr.GetClient(),
		Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(adminv1alpha1.TenantGroupVersionKind),
		managed.WithLogger(nddcopts.Logger.WithValues("controller", name)),
		managed.WithApplogic(&applogic{
			log:             nddcopts.Logger.WithValues("applogic", name),
			client:          c,
			porchClient:     nddcopts.PorchClient,
			porchRESTClient: nddcopts.PorchRESTClient,
		}),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(nddcopts.Copts).
		For(&adminv1alpha1.Tenant{}).
		Owns(&adminv1alpha1.Tenant{}).
		WithEventFilter(resource.IgnoreUpdateWithoutGenerationChangePredicate()).
		Complete(r)
}
*/

/*
type applogic struct {
	client          resource.ClientApplicator
	porchClient     client.Client
	porchRESTClient rest.Interface
	log             logging.Logger
	//abstractions map[string]*abstraction.Compositeabstraction

}

func (r *applogic) Initialize(ctx context.Context, mr resource.Managed) error {
	return nil
}

func (r *applogic) Update(ctx context.Context, mr resource.Managed) (map[string]string, error) {
	// cast the type to the real object/resource we expect
	cr, ok := mr.(*adminv1alpha1.Tenant)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}
	crName := cr.GetNamespacedName()
	log := r.log.WithValues("crName", crName)
	log.Debug("update")
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
	if err := p.GetOrCreate(ctx, mr); err != nil {
		return nil, err
	}

	newResources, err := r.populateSchema(ctx, mr)
	if err != nil {
		return nil, err
	}

	newResources.Print("new")

	actualResources, err := p.GetResources(ctx, mr)
	if err != nil {
		return nil, err
	}

	actualResources.Print("actual")

	// compare the new resources with the actual resources
	isEqual, err := newResources.IsEqual(actualResources.Get())
	if err != nil {
		return nil, err
	}
	if !isEqual {
		log.Info("package is NOT up to date")
		// check if the pr is in draft state, if not create a new pr
		if p.GetLifecycle() != v1alpha1.PackageRevisionLifecycleDraft {
			log.Info("new PR created")
			// create a new pr
			p.Create(ctx, mr)
		}
		if err := p.Update(ctx, newResources); err != nil {
			log.Debug("update failed", "error", err)
			return nil, err
		}
		// all good the package is up to date
	} else {
		log.Info("package is up to date")
	}

	return nil, p.Approve(ctx)
}

func (r *applogic) FinalUpdate(ctx context.Context, mr resource.Managed) {
}

func (r *applogic) Timeout(ctx context.Context, mr resource.Managed) time.Duration {
	return 0
}

func (r *applogic) Delete(ctx context.Context, mr resource.Managed) (bool, error) {
	return true, nil
}

func (r *applogic) FinalDelete(ctx context.Context, mr resource.Managed) {
}

func (r *applogic) populateSchema(ctx context.Context, mr resource.Managed) (appresource.Resources, error) {
	// cast the type to the real object/resource we expect
	cr, ok := mr.(*adminv1alpha1.Tenant)
	if !ok {
		return nil, errors.New(errUnexpectedResource)
	}
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
*/
