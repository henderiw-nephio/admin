package pkgrev

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getRevisionNumber(revision string) uint64 {
	fmt.Printf("getRevisionNumber: %s\n", revision)
	if strings.HasPrefix(revision, "v") {
		revision = strings.ReplaceAll(revision, "v", "")
		rev, err := strconv.Atoi(revision)
		if err != nil {
			fmt.Printf("getRevisionNumber result: %d\n", 0)
			return 0
		}
		fmt.Printf("getRevisionNumber result: %d\n", rev)
		return uint64(rev)
	}
	fmt.Printf("getRevisionNumber result: %d\n", 0)
	return 0
}

func getNewRevision(revision string) string {
	nbr := getRevisionNumber(revision)
	fmt.Printf("revision number: %d\n", nbr)
	return "v" + strconv.Itoa(int(nbr)+1)
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
