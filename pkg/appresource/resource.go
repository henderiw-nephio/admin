package appresource

import (
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Resources interface {
	// Add adds a resource to the resource list, the name should be unique e.g. gvknns in k8s context
	Add(rn *yaml.RNode)
	Get() []*yaml.RNode
	Copy() Resources
	IsEqual(arnl []*yaml.RNode) (bool, error)
	Print(prefix string)
}

func New() Resources {
	return &resources{
		resources: []*yaml.RNode{},
	}
}

type resources struct {
	resources []*yaml.RNode
}

func (x *resources) Add(rn *yaml.RNode) {
	x.resources = append(x.resources, rn)
}

func (x *resources) Print(prefix string) {
	for _, rn := range x.resources {
		fmt.Printf("%s resources apiversion: %s, kind: %s, name: %s, annotations: %v, labels: %v \n", prefix, rn.GetApiVersion(), rn.GetKind(), rn.GetName(), rn.GetAnnotations(), rn.GetLabels())
	}
}

func (x *resources) Get() []*yaml.RNode {
	return x.resources
}

func (x *resources) Copy() Resources {
	resources := New()
	for _, r := range x.resources {
		resources.Add(r.Copy())
	}
	return resources
}

// IsEqual validates if the resources are equal or not
func (x *resources) IsEqual(arnl []*yaml.RNode) (bool, error) {

	newResources := x.Copy()
	for _, nr := range newResources.Get() {
		found := false
		for i, ar := range arnl {
			// right now we clear annotations but this is a workaround
			nr.SetAnnotations(map[string]string{})
			nrStr, err := nr.String()
			if err != nil {
				return false, err
			}
			// right now we clear annotations but this is a workaround
			ar.SetAnnotations(map[string]string{})
			arStr, err := ar.String()
			if err != nil {
				return false, err
			}
			if nrStr == arStr {
				found = true
				arnl = append(arnl[:i], arnl[i+1:]...)
			}
		}
		if !found {
			fmt.Printf("new resource not ok: %s\n", nr.MustString())
			return false, nil
		}
	}
	// this means some entries should be deleted
	// hence the resource are not equal
	if len(arnl) != 0 {
		return false, nil
	}
	return true, nil
}
