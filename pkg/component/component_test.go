package component

import (
	"fmt"
	"strings"
	"testing"

	k8s "k8s.io/api/apps/v1"
)

type Input struct {
	Number int
}

type Context struct {
	Number int
	Catify func(s string) string
}

func setupComponentFromFile[T any]() (Component[T, Input], error) {
	return CreateComponent[T, Input, Context](
		Def[T, Input, Context]{
			Setup: func(input Input) Context {
				return Context{
					Number: input.Number,
					Catify: func(s string) string {
						return fmt.Sprintf("🐈 %s 🐈", s)
					},
				}
			},
			Template:       `../../examples/helm/helm.yaml`,
			TemplateIsFile: true,
		},
	)
}

func setupComponentMultiFromFile[T any](makeInstances func(Input) ([]T, error)) (ComponentMulti[T, Input], error) {
	return CreateComponentMulti[T, Input, Context](
		Def[[]T, Input, Context]{
			Template:       `../../examples/helm/helm.yaml`,
			TemplateIsFile: true,
			MakeInstances: func(input Input) ([]T, error) {
				return makeInstances(input)
			},
			Setup: func(input Input) Context {
				return Context{
					Number: input.Number,
					Catify: func(s string) string {
						return fmt.Sprintf("🐈 %s 🐈", s)
					},
				}
			},
		},
	)
}

func TestCreateComponentFromFile(t *testing.T) {
	err := error(nil)
	comp, err := setupComponentFromFile[k8s.Deployment]()

	if err != nil {
		t.Error(err)
	}

	instance, contents, err := comp.Render(Input{Number: 2})
	if err != nil {
		t.Error(err)
	}

	if len(contents) != 717 {
		t.Errorf("contents != 717, got %v", len(contents))
	}

	if contents == "" {
		t.Error("one of contents is missing value")
	}

	searched := "gcr.io/kuar-demo/kuard-amd64:1"
	if !strings.Contains(contents, searched) {
		t.Error("contents have invalid contant")
	}

	if instance.Spec.Template.Spec.Containers[0].Image != searched {
		t.Error("one of unmarshalled values is invalid")
	}
}

func TestCreateComponentFromFileFailsOnInvalidUnmarshal(t *testing.T) {
	err := error(nil)
	comp, err := setupComponentFromFile[k8s.DaemonSet]()

	if err != nil {
		t.Error(err)
	}

	_, _, err = comp.Render(Input{Number: 2})
	if err == nil {
		t.Error("Expected error")
	}
	if !strings.Contains(err.Error(), "json: unknown field \"replicas\"") {
		t.Errorf("Expected different error, got %v", err)
	}
}

func TestCreateComponentFromFileMulti(t *testing.T) {
	err := error(nil)
	comp, err := setupComponentMultiFromFile[k8s.Deployment](
		func(Input) ([]k8s.Deployment, error) {
			return []k8s.Deployment{{}, {}}, nil
		},
	)

	if err != nil {
		t.Error(err)
	}

	instances, contents, err := comp.Render(Input{Number: 2})
	if err != nil {
		t.Error(err)
	}

	if len(contents) != 2 {
		t.Errorf("contents != 2, got %v", len(contents))
	}

	if contents[0] == "" || contents[1] == "" {
		t.Error("one of contents is missing value")
	}

	searched := "gcr.io/kuar-demo/kuard-amd64:1"
	if !strings.Contains(contents[0], searched) || !strings.Contains(contents[1], searched) {
		t.Error("contents have invalid contant")
	}

	if len(instances) != 2 {
		t.Errorf("contents != 2, got %v", len(contents))
	}

	if instances[0].Spec.Template.Spec.Containers[0].Image != searched || instances[1].Spec.Template.Spec.Containers[0].Image != searched {
		t.Error("one of unmarshalled values is invalid")
	}
}

func TestCreateComponentFromFileMultiFailsOnInvalidUnmarshal(t *testing.T) {
	err := error(nil)
	comp, err := setupComponentMultiFromFile[k8s.DaemonSet](
		func(Input) ([]k8s.DaemonSet, error) {
			return []k8s.DaemonSet{{}, {}}, nil
		},
	)

	if err != nil {
		t.Error(err)
	}

	_, _, err = comp.Render(Input{Number: 2})
	if err == nil {
		t.Error("Expected error")
	}
	if !strings.Contains(err.Error(), "json: unknown field \"replicas\"") {
		t.Errorf("Expected different error, got %v", err)
	}
}

func BenchmarkCreateComponentFromFileMulti(b *testing.B) {
	for i := 0; i < b.N; i++ {
		comp, _ := setupComponentMultiFromFile[k8s.Deployment](
			func(Input) ([]k8s.Deployment, error) {
				return []k8s.Deployment{{}, {}}, nil
			},
		)
		comp.Render(Input{Number: 2})
	}
}
