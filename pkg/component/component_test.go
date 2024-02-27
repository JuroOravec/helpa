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
	Number string
	Catify func(s string) string
}

func setupComponentInline[T any](
	template string,
	render func(Input, Context, string) (T, error),
) (Component[T, Input], error) {
	return CreateComponent(
		Def[T, Input, Context]{
			Setup: func(input Input) (Context, error) {
				context := Context{
					Number: fmt.Sprint(input.Number),
					Catify: func(s string) string {
						return fmt.Sprintf("🐈 %s 🐈", s)
					},
				}
				return context, nil
			},
			Template: template,
			Render:   render,
		},
	)
}

func setupComponentFromFile[T any](
	render func(Input, Context, string) (T, error),
) (Component[T, Input], error) {
	return CreateComponent(
		Def[T, Input, Context]{
			Template:       `../../examples/helm/helm.yaml`,
			TemplateIsFile: true,
			Setup: func(input Input) (Context, error) {
				context := Context{
					Number: fmt.Sprint(input.Number),
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
	makeInstances func(Input, Context) ([]T, error),
	render func(Input, Context, []string) ([]T, error),
) (ComponentMulti[T, Input], error) {
	return CreateComponentMulti(
		DefMulti[T, Input, Context]{
			Template:       `../../examples/helm/helm.yaml`,
			TemplateIsFile: true,
			GetInstances:   makeInstances,
			Render:         render,
		},
	)
}

func setupComponentFromFileFrontload[T any](
	setup func(input Input) (Context, error),
	frontloadInput Input,
) (Component[T, Input], error) {
	return CreateComponent(
		Def[T, Input, Context]{
			Template:       `../../examples/helm/helm.yaml`,
			TemplateIsFile: true,
			Options: Options[Input]{
				FrontloadEnabled: true,
				FrontloadInput:   frontloadInput,
			},
			Setup: setup,
		},
	)
}

func setupComponentMultiFromFileFrontload[T any](
	makeInstances func(Input, Context) ([]T, error),
	frontloadInput Input,
) (ComponentMulti[T, Input], error) {
	return CreateComponentMulti(
		DefMulti[T, Input, Context]{
			Template:       `../../examples/helm/helm.yaml`,
			TemplateIsFile: true,
			GetInstances:   makeInstances,
			Setup: func(input Input) (Context, error) {
				context := Context{
					Number: fmt.Sprint(input.Number),
					Catify: func(s string) string {
						return fmt.Sprintf("🐈 %s 🐈", s)
					},
				}
				return context, nil
			},
			Options: Options[Input]{
				FrontloadEnabled: true,
				FrontloadInput:   frontloadInput,
			},
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

func TestCreateComponentInline(t *testing.T) {
	err := error(nil)
	comp, err := setupComponentInline[any](`Hello: {{ Catify .Helpa.Number }}`, nil)

	if err != nil {
		t.Error(err)
	}

	_, content, err := comp.Render(Input{Number: 2})
	if err != nil {
		t.Error(err)
	}

	if content != "Hello: 🐈 2 🐈" {
		t.Errorf("Content differs from expeced, got %s", content)
	}
}

func TestComponentInlineEscape(t *testing.T) {
	err := error(nil)
	comp, err := setupComponentInline[any](`Hello: {{ Catify .Helpa.Number }} {{! .Releases.Some.Path }}`, nil)

	if err != nil {
		t.Error(err)
	}

	_, content, err := comp.Render(Input{Number: 2})
	if err != nil {
		t.Error(err)
	}

	if content != "Hello: 🐈 2 🐈 {{ .Releases.Some.Path }}" {
		t.Errorf("Content differs from expeced, got %s", content)
	}
}

func TestComponentFrontloadFailsAtInit(t *testing.T) {
	err := error(nil)
	inputAtInit := Input{}
	_, err = setupComponentFromFileFrontload[k8s.DaemonSet](
		func(input Input) (Context, error) {
			inputAtInit = input
			return Context{}, nil
		},
		Input{Number: 3},
	)

	if err == nil {
		t.Error("Expected error")
	}
	if !strings.Contains(err.Error(), "json: unknown field \"replicas\"") {
		t.Errorf("Expected different error, got %v", err)
	}

	if inputAtInit.Number != 3 {
		t.Errorf("Expected frontload input Number == 3, got %v", inputAtInit.Number)
	}
}

func TestCreateComponentFromFileMulti(t *testing.T) {
	err := error(nil)
	comp, err := setupComponentMultiFromFile[k8s.Deployment](
		func(Input, Context) ([]k8s.Deployment, error) {
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
		func(Input, Context) ([]k8s.DaemonSet, error) {
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

func TestComponentMultiFrontloadFailsAtInit(t *testing.T) {
	err := error(nil)
	inputAtInit := Input{}
	_, err = setupComponentMultiFromFileFrontload[k8s.DaemonSet](
		func(input Input, context Context) ([]k8s.DaemonSet, error) {
			inputAtInit = input
			return []k8s.DaemonSet{{}, {}}, nil
		},
		Input{Number: 3},
	)

	if err == nil {
		t.Error("Expected error")
	}
	if !strings.Contains(err.Error(), "json: unknown field \"replicas\"") {
		t.Errorf("Expected different error, got %v", err)
	}

	if inputAtInit.Number != 3 {
		t.Errorf("Expected frontload input Number == 3, got %v", inputAtInit.Number)
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
		func(Input, Context) ([]k8s.Deployment, error) {
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
			func(Input, Context) ([]k8s.Deployment, error) {
				return []k8s.Deployment{{}, {}}, nil
			},
			nil,
		)
		comp.Render(Input{Number: 2})
	}
}
