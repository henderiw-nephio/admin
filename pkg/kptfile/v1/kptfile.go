package v1

import (
	"bytes"
	"html/template"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func BuildKptFile(pkgName string) (*yaml.RNode, error) {
	var kptfileTemplate = `apiVersion: kpt.dev/v1
kind: Kptfile
metadata:
  name: {{.PkgName}}
  annotations:
    internal.config.kubernetes.io/index: "0"
    internal.config.kubernetes.io/path: "Kptfile"
    config.kubernetes.io/local-config: "true"
info:
  description: {{.PkgName}}
`
	tmpl, err := template.New("test").Parse(kptfileTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{
		"PkgName": pkgName,
	})
	if err != nil {
		return nil, err
	}
	return yaml.Parse(buf.String())

	/*
		k := kptfilev1.KptFile{
			ResourceMeta: yaml.ResourceMeta{
				ObjectMeta: yaml.ObjectMeta{
					NameMeta: yaml.NameMeta{
						Name: pkgName,
					},
					// mark Kptfile as local-config
					Annotations: map[string]string{
						filters.LocalConfigAnnotation: "true",
					},
				},
			},
			Info: &kptfilev1.PackageInfo{
				Description: pkgName,
				//Site:        opts.Site,
				//Keywords:    opts.Keywords,
			},
		}

		// serialize the gvk when writing the Kptfile
		k.Kind = kptfilev1.TypeMeta.Kind
		k.APIVersion = kptfilev1.TypeMeta.APIVersion

		b := new(strings.Builder)
		//p := printers.YAMLPrinter{}
		//p.PrintObj(k, b)

		e := yaml.NewEncoder(b)
		if err := e.Encode(k); err != nil {
			return nil, err
		}

		return yaml.Parse(b.String())
	*/
}
