package pkgutil

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yndd/admin/pkg/appresource"
	kptfilev1 "github.com/yndd/admin/pkg/kptfile/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Code adapted from Porch internal cmdrpkgpull and cmdrpkgpush
func ResourcesToPackageBuffer(resources map[string]string) (*kio.PackageBuffer, error) {
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

func MergePackages(packageName string, res map[string]appresource.Resources) (appresource.Resources, error) {
	resources := appresource.New()
	k, err := kptfilev1.BuildKptFile(packageName)
	if err != nil {
		return nil, err
	}
	// add the resource to the resource list
	resources.Add(k)

	for subPkgName, appres := range res {
		for _, r := range appres.Get() {
			x := r.GetAnnotations()
			x["internal.config.kubernetes.io/path"] = filepath.Join(subPkgName, x["internal.config.kubernetes.io/path"])

			if err := r.SetAnnotations(x); err != nil {
				return nil, err
			}
			resources.Add(r)
		}
	}
	return resources, nil
}

func CreateUpdatedResource(origResources map[string]string, pb *kio.PackageBuffer) (map[string]string, error) {
	fmt.Printf("orig resources\n")
	for k, v := range origResources {
		fmt.Printf("orig resources: key %s, value %s", k, v)
	}
	newResources := map[string]string{}
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

	//pb.Nodes = append(pb.Nodes, localKptFiles...)

	if err := (kio.Pipeline{
		Inputs:  []kio.Reader{pb},
		Outputs: []kio.Writer{rw},
	}.Execute()); err != nil {
		return nil, err
	}
	fmt.Printf("new resources\n")
	for k, v := range rw.resources {
		fmt.Printf("new resources: key %s, value %s", k, v)
	}
	return rw.resources, nil
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
