package v1

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

/*
func Init(n *kyaml.RNode) appresource.Resource {
	return &namespace{
		node: n,
	}
}
*/

func GVKName(namespace, name string) string {
	return strings.Join([]string{"core", "v1", "Namespace", namespace, name}, "/")
}

/*
type namespace struct {
	node *kyaml.RNode
}

func (x *namespace) GetResource() *kyaml.RNode {
	return x.node
}

func (x *namespace) AddResource(n *kyaml.RNode) {
	x.node = n
}
*/

func BuildNamespace(name string) (*kyaml.RNode, error) {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	b := new(strings.Builder)
	p := printers.YAMLPrinter{}
	p.PrintObj(ns, b)

	return kyaml.Parse(b.String())
}
