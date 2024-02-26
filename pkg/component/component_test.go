package component

import (
	"fmt"
	"strings"
	"testing"

	k8s "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

type Input struct {
	Number int
}

type Context struct {
	Number int
	Catify func(s string) string
}

func setupComponentFromFile[T any](
	render func(Input, Context, string) (T, error),
) (Component[T, Input], error) {
	return CreateComponent[T, Input, Context](
		Def[T, Input, Context]{
			Template:       `../../examples/helm/helm.yaml`,
			TemplateIsFile: true,
			Setup: func(input Input) (Context, error) {
				context := Context{
					Number: input.Number,
					Catify: func(s string) string {
						return fmt.Sprintf("🐈 %s 🐈", s)
					},
				}
				return context, nil
			},
			Render: render,
		},
	)
}

func setupComponentMultiFromFile[T any](
	makeInstances func(Input) ([]T, error),
	render func(Input, Context, []string) ([]T, error),
) (ComponentMulti[T, Input], error) {
	return CreateComponentMulti[T, Input, Context](
		DefMulti[T, Input, Context]{
			Template:       `../../examples/helm/helm.yaml`,
			TemplateIsFile: true,
			GetInstances:   makeInstances,
			Setup: func(input Input) (Context, error) {
				context := Context{
					Number: input.Number,
					Catify: func(s string) string {
						return fmt.Sprintf("🐈 %s 🐈", s)
					},
				}
				return context, nil
			},
			Render: render,
		},
	)
}

func TestCreateComponentFromFile(t *testing.T) {
	err := error(nil)
	comp, err := setupComponentFromFile[k8s.Deployment](nil)

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
	comp, err := setupComponentFromFile[k8s.DaemonSet](nil)

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
		nil,
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
		t.Errorf("instances != 2, got %v", len(instances))
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
		nil,
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

func TestComponentRender(t *testing.T) {
	err := error(nil)

	didCallRender := false
	comp, err := setupComponentFromFile[k8s.Deployment](
		func(Input, Context, string) (k8s.Deployment, error) {
			didCallRender = true
			return k8s.Deployment{
				Spec: k8s.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "My super container",
									Image: "gcr.io/wow-so-great:1",
								},
							},
						},
					},
				},
			}, nil
		},
	)

	if err != nil {
		t.Error(err)
	}

	instance, content, err := comp.Render(Input{Number: 2})
	if err != nil {
		t.Error(err)
	}

	if !didCallRender {
		t.Errorf("Render was not called")
	}

	if len(content) != 717 {
		t.Errorf("content != 717, got %v", len(content))
	}

	oldSearched := "gcr.io/kuar-demo/kuard-amd64:1"
	newSearched := "gcr.io/wow-so-great:1"

	// NOTE: Currently, when user overrides the `Render` function, we still keep around the rendered
	// content, so that user may work with it inside the `Render` function.
	// So the `content` var should match the "old" spec.
	if !strings.Contains(content, oldSearched) {
		t.Error("invalid contant")
	}

	// But the `instance` was was returned from the Render function, so that should match
	// the the "new" spec.
	if instance.Spec.Template.Spec.Containers[0].Image != newSearched {
		t.Error("one of render values is invalid")
	}
}

func TestComponentMultiRender(t *testing.T) {
	err := error(nil)

	didCallInstances := false
	didCallRender := false
	comp, err := setupComponentMultiFromFile[k8s.Deployment](
		func(Input) ([]k8s.Deployment, error) {
			didCallInstances = true
			return []k8s.Deployment{{}, {}}, nil
		},
		func(Input, Context, []string) ([]k8s.Deployment, error) {
			didCallRender = true
			return []k8s.Deployment{
				{
					Spec: k8s.DeploymentSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "My super container",
										Image: "gcr.io/wow-so-great:1",
									},
								},
							},
						},
					},
				},
			}, nil
		},
	)

	if err != nil {
		t.Error(err)
	}

	instances, contents, err := comp.Render(Input{Number: 2})
	if err != nil {
		t.Error(err)
	}

	if !didCallInstances {
		t.Errorf("GetInstances was not called")
	}
	if !didCallRender {
		t.Errorf("Render was not called")
	}

	if len(contents) != 2 {
		t.Errorf("contents != 2, got %v", len(contents))
	}
	if contents[0] == "" || contents[1] == "" {
		t.Error("one of contents is missing value")
	}

	oldSearched := "gcr.io/kuar-demo/kuard-amd64:1"
	newSearched := "gcr.io/wow-so-great:1"

	// NOTE: Currently, when user overrides the `Render` function, we still keep around the rendered
	// content, so that user may work with it inside the `Render` function.
	// So the `contents` var should match the "old" spec.
	if !strings.Contains(contents[0], oldSearched) || !strings.Contains(contents[1], oldSearched) {
		t.Error("contents have invalid contant")
	}

	// But the `instances` is was was returned from the Render function, so that should match
	// the the "new" spec.
	if len(instances) != 1 {
		t.Errorf("instances != 1, got %v", len(instances))
	}

	if instances[0].Spec.Template.Spec.Containers[0].Image != newSearched {
		t.Error("one of render values is invalid")
	}
}

func BenchmarkCreateComponentFromFileMulti(b *testing.B) {
	for i := 0; i < b.N; i++ {
		comp, _ := setupComponentMultiFromFile[k8s.Deployment](
			func(Input) ([]k8s.Deployment, error) {
				return []k8s.Deployment{{}, {}}, nil
			},
			nil,
		)
		comp.Render(Input{Number: 2})
	}
}
