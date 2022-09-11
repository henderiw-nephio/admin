package porch2

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func getRevisionNumber(revision string) uint64 {
	if strings.HasPrefix(revision, "v") {
		revision = strings.ReplaceAll(revision, "v", "")
		rev, err := strconv.Atoi(revision)
		if err != nil {
			return 0
		}
		return uint64(rev)
	}
	return 0
}

func getNewRevision(revision string) string {
	nbr := getRevisionNumber(revision)
	return "v" + strconv.Itoa(int(nbr)+1)
}

// Code adapted from Porch internal cmdrpkgpull and cmdrpkgpush
func resourcesToPackageBuffer(resources map[string]string) (*kio.PackageBuffer, error) {
	keys := make([]string, 0, len(resources))
	//fmt.Printf("ResourcesToPackageBuffer: resources: %v\n", len(resources))
	for k := range resources {
		//fmt.Printf("ResourcesToPackageBuffer: resources key %s\n", k)
		if !includeFile(k) {
			continue
		}
		//fmt.Printf("ResourcesToPackageBuffer: resources key append %s\n", k)
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create kio readers
	inputs := []kio.Reader{}
	for _, k := range keys {
		//fmt.Printf("ResourcesToPackageBuffer: key %s\n", k)
		v := resources[k]
		inputs = append(inputs, &kio.ByteReader{
			Reader: strings.NewReader(v),
			SetAnnotations: map[string]string{
				kioutil.PathAnnotation: k,
			},
			DisableUnwrapping: true,
		})
	}

	var pb kio.PackageBuffer
	err := kio.Pipeline{
		Inputs:  inputs,
		Outputs: []kio.Writer{&pb},
	}.Execute()

	if err != nil {
		return nil, err
	}

	return &pb, nil
}

type resourceWriter struct {
	resources map[string]string
}

var _ kio.Writer = &resourceWriter{}

func (w *resourceWriter) Write(nodes []*yaml.RNode) error {
	paths := map[string][]*yaml.RNode{}
	for _, node := range nodes {
		path := getPath(node)
		paths[path] = append(paths[path], node)
	}

	buf := &bytes.Buffer{}
	for path, nodes := range paths {
		bw := kio.ByteWriter{
			Writer: buf,
			ClearAnnotations: []string{
				kioutil.PathAnnotation,
				kioutil.IndexAnnotation,
			},
		}
		if err := bw.Write(nodes); err != nil {
			return err
		}
		w.resources[path] = buf.String()
		buf.Reset()
	}
	return nil
}

func getPath(node *yaml.RNode) string {
	ann := node.GetAnnotations()
	if path, ok := ann[kioutil.PathAnnotation]; ok {
		return path
	}
	/*
		ns := node.GetNamespace()
		if ns == "" {
			ns = "non-namespaced"
		}
	*/
	name := node.GetName()
	if name == "" {
		name = "unnamed"
	}
	// TODO: harden for escaping etc.
	//	return path.Join(ns, fmt.Sprintf("%s.yaml", name))
	return fmt.Sprintf("%s.yaml", name)
}

func createUpdatedResource(origResources map[string]string, pb *kio.PackageBuffer, localKptFiles []*yaml.RNode) (map[string]string, error) {
	newResources := make(map[string]string, len(origResources))
	for k, v := range origResources {
		// Copy ALL non-KRM files
		if !includeFile(k) {
			newResources[k] = v
		}
	}

	// Copy the KRM resources from the PackageBuffer
	rw := &resourceWriter{
		resources: newResources,
	}

	pb.Nodes = append(pb.Nodes, localKptFiles...)

	if err := (kio.Pipeline{
		Inputs:  []kio.Reader{pb},
		Outputs: []kio.Writer{rw},
	}.Execute()); err != nil {
		return nil, err
	}
	return rw.resources, nil
}

// import fails with kptfile/v1
var matchResourceContents = append(kio.MatchAll, "Kptfile")

func includeFile(path string) bool {
	//fmt.Printf("includeFile matchResourceContents: %v path: %s\n", matchResourceContents, path)
	for _, m := range matchResourceContents {
		file := filepath.Base(path)
		if matched, err := filepath.Match(m, file); err == nil && matched {
			//fmt.Printf("includeFile match: %v\n", path)
			return true
		}
	}
	//fmt.Printf("includeFile no match: %v\n", path)
	return false
}

func updatePackageRevisionApproval(ctx context.Context, client rest.Interface, key client.ObjectKey, new porchv1alpha1.PackageRevisionLifecycle) error {
	scheme := runtime.NewScheme()
	if err := porchv1alpha1.SchemeBuilder.AddToScheme(scheme); err != nil {
		return err
	}

	codec := runtime.NewParameterCodec(scheme)
	var pr porchv1alpha1.PackageRevision
	if err := client.Get().
		Namespace(key.Namespace).
		Resource("packagerevisions").
		Name(key.Name).
		VersionedParams(&metav1.GetOptions{}, codec).
		Do(ctx).
		Into(&pr); err != nil {
		return err
	}

	switch lifecycle := pr.Spec.Lifecycle; lifecycle {
	case porchv1alpha1.PackageRevisionLifecycleProposed:
		// ok
	case new:
		// already correct value
		return nil
	default:
		return fmt.Errorf("cannot change approval from %s to %s", lifecycle, new)
	}

	// Approve - change the package revision kind to "final".
	pr.Spec.Lifecycle = new

	opts := metav1.UpdateOptions{}
	result := &porchv1alpha1.PackageRevision{}
	if err := client.Put().
		Namespace(pr.Namespace).
		Resource("packagerevisions").
		Name(pr.Name).
		SubResource("approval").
		VersionedParams(&opts, codec).
		Body(&pr).
		Do(ctx).
		Into(result); err != nil {
		return err
	}
	return nil
}
