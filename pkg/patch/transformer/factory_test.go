/*
Copyright 2018 The Kubernetes Authors.

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

package transformer

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/pkg/internal/loadertest"
	"sigs.k8s.io/kustomize/pkg/patch"
	"sigs.k8s.io/kustomize/pkg/resmap"
	"sigs.k8s.io/kustomize/pkg/resource"
)

func TestNewPatchJson6902FactoryNoTarget(t *testing.T) {
	p := patch.PatchJson6902{}
	_, err := NewPatchJson6902Factory(nil).makeOnePatchJson6902Transformer(p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must specify the target field in patchesJson6902") {
		t.Fatalf("incorrect error returned: %v", err)
	}
}

func TestNewPatchJson6902FactoryConflict(t *testing.T) {
	jsonPatch := []byte(`
target:
  name: some-name
  kind: Deployment
`)
	p := patch.PatchJson6902{}
	err := yaml.Unmarshal(jsonPatch, &p)
	if err != nil {
		t.Fatalf("expected error %v", err)
	}
	f := NewPatchJson6902Factory(nil)
	_, err = f.makeOnePatchJson6902Transformer(p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must specify the path for a json patch file") {
		t.Fatalf("incorrect error returned %v", err)
	}
}

func TestNewPatchJson6902FactoryJSON(t *testing.T) {
	ldr := loadertest.NewFakeLoader("/testpath")
	operations := []byte(`[
        {"op": "replace", "path": "/spec/template/spec/containers/0/name", "value": "my-nginx"},
        {"op": "add", "path": "/spec/replica", "value": "3"},
        {"op": "add", "path": "/spec/template/spec/containers/0/command", "value": ["arg1", "arg2", "arg3"]}
]`)
	err := ldr.AddFile("/testpath/patch.json", operations)
	if err != nil {
		t.Fatalf("Failed to setup fake ldr.")
	}

	jsonPatch := []byte(`
target:
  kind: Deployment
  name: some-name
path: /testpath/patch.json
`)
	p := patch.PatchJson6902{}
	err = yaml.Unmarshal(jsonPatch, &p)
	if err != nil {
		t.Fatal("expected error")
	}

	tr, err := NewPatchJson6902Factory(ldr).makeOnePatchJson6902Transformer(p)
	if err != nil {
		t.Fatalf("unexpected error : %v", err)
	}
	if tr == nil {
		t.Fatal("the returned transformer should not be nil")
	}
}

func TestNewPatchJson6902FactoryYAML(t *testing.T) {
	ldr := loadertest.NewFakeLoader("/testpath")
	operations := []byte(`
- op: replace
  path: /spec/template/spec/containers/0/name
  value: my-nginx
- op: add
  path: /spec/replica
  value: 3
- op: add
  path: /spec/template/spec/containers/0/command
  value: ["arg1", "arg2", "arg3"]
`)
	err := ldr.AddFile("/testpath/patch.yaml", operations)
	if err != nil {
		t.Fatalf("Failed to setup fake ldr.")
	}
	jsonPatch := []byte(`
target:
  name: some-name
  kind: Deployment
path: /testpath/patch.yaml
`)
	p := patch.PatchJson6902{}
	err = yaml.Unmarshal(jsonPatch, &p)
	if err != nil {
		t.Fatalf("unexpected error : %v", err)
	}

	tr, err := NewPatchJson6902Factory(ldr).makeOnePatchJson6902Transformer(p)
	if err != nil {
		t.Fatalf("unexpected error : %v", err)
	}
	if tr == nil {
		t.Fatal("the returned transformer should not be nil")
	}
}

func TestNewPatchJson6902FactoryMulti(t *testing.T) {
	ldr := loadertest.NewFakeLoader("/testpath")
	operations := []byte(`[
        {"op": "replace", "path": "/spec/template/spec/containers/0/name", "value": "my-nginx"},
        {"op": "add", "path": "/spec/replica", "value": "3"}
]`)
	err := ldr.AddFile("/testpath/patch.json", operations)
	if err != nil {
		t.Fatalf("Failed to setup fake ldr.")
	}

	operations = []byte(`
- op: add
  path: /spec/template/spec/containers/0/command
  value: ["arg1", "arg2", "arg3"]
`)
	err = ldr.AddFile("/testpath/patch.yaml", operations)
	if err != nil {
		t.Fatalf("Failed to setup fake ldr.")
	}

	jsonPatches := []byte(`
- target:
    kind: foo
    name: some-name
  path: /testpath/patch.json

- target:
    kind: foo
    name: some-name
  path: /testpath/patch.yaml
`)
	var p []patch.PatchJson6902
	err = yaml.Unmarshal(jsonPatches, &p)
	if err != nil {
		t.Fatalf("unexpected error : %v", err)
	}

	f := NewPatchJson6902Factory(ldr)
	tr, err := f.MakePatchJson6902Transformer(p)
	if err != nil {
		t.Fatalf("unexpected error : %v", err)
	}
	if tr == nil {
		t.Fatal("the returned transformer should not be nil")
	}

	id := resource.NewResId(schema.GroupVersionKind{Kind: "foo"}, "some-name")
	base := resmap.ResMap{
		id: resource.NewResourceFromMap(
			map[string]interface{}{
				"kind": "foo",
				"metadata": map[string]interface{}{
					"name": "some-name",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"old-label": "old-value",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"image": "nginx",
									"name":  "nginx",
								},
							},
						},
					},
				},
			}),
	}
	expected := resmap.ResMap{
		id: resource.NewResourceFromMap(
			map[string]interface{}{
				"kind": "foo",
				"metadata": map[string]interface{}{
					"name": "some-name",
				},
				"spec": map[string]interface{}{
					"replica": "3",
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"old-label": "old-value",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"image": "nginx",
									"name":  "my-nginx",
									"command": []interface{}{
										"arg1",
										"arg2",
										"arg3",
									},
								},
							},
						},
					},
				},
			}),
	}
	err = tr.Transform(base)
	if err != nil {
		t.Fatalf("unexpected error : %v", err)
	}
	if !reflect.DeepEqual(base, expected) {
		err = expected.ErrorIfNotEqual(base)
		t.Fatalf("actual doesn't match expected: %v", err)
	}
}

func TestNewPatchJson6902FactoryMultiConflict(t *testing.T) {
	ldr := loadertest.NewFakeLoader("/testpath")
	operations := []byte(`[
        {"op": "add", "path": "/spec/replica", "value": "3"}
]`)
	err := ldr.AddFile("/testpath/patch.json", operations)
	if err != nil {
		t.Fatalf("Failed to setup fake ldr.")
	}
	operations = []byte(`
- op: add
  path: /spec/replica
  value: 4
`)
	err = ldr.AddFile("/testpath/patch.yaml", operations)
	if err != nil {
		t.Fatalf("Failed to setup fake ldr.")
	}

	jsonPatches := []byte(`
- target:
    kind: foo
    name: some-name
  path: /testpath/patch.json

- target:
    kind: foo
    name: some-name
  path: /testpath/patch.yaml
`)
	var p []patch.PatchJson6902
	err = yaml.Unmarshal(jsonPatches, &p)
	if err != nil {
		t.Fatalf("unexpected error : %v", err)
	}

	f := NewPatchJson6902Factory(ldr)
	tr, err := f.MakePatchJson6902Transformer(p)
	if err != nil {
		t.Fatalf("unexpected error : %v", err)
	}
	if tr == nil {
		t.Fatal("the returned transformer should not be nil")
	}

	id := resource.NewResId(schema.GroupVersionKind{Kind: "foo"}, "some-name")
	base := resmap.ResMap{
		id: resource.NewResourceFromMap(
			map[string]interface{}{
				"kind": "foo",
				"metadata": map[string]interface{}{
					"name": "somename",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"old-label": "old-value",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"image": "nginx",
									"name":  "nginx",
								},
							},
						},
					},
				},
			}),
	}

	err = tr.Transform(base)
	if err == nil {
		t.Fatal("expected conflict")
	}
	if !strings.Contains(err.Error(), "found conflict between different patches") {
		t.Fatalf("incorrect error happened %v", err)
	}
}
