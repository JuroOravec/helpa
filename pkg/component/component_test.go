package component

import (
	"fmt"
	"testing"

	"github.com/jurooravec/helpa/pkg/utils"
	assert "github.com/stretchr/testify/assert"
	k8s "k8s.io/api/apps/v1"
)

type Input struct {
	Number int
	Name   string
}

type Context struct {
	Number string
	Catify func(s string) string
}

type FromFileSpec struct {
	My   string   `json:"my"`
	Spec []string `json:"spec"`
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
			Template:       `../../examples/fromfile/fromfile.yaml`,
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

func setupComponentMulti[T any](
	makeInstances func(Input, Context) ([]T, error),
	render func(Input, Context, []string) ([]T, error),
) (ComponentMulti[T, Input], error) {
	return CreateComponentMulti(
		DefMulti[T, Input, Context]{
			Template: `
			my: cool
			spec:
			- Hello
			- There
			- {{ .Number | quote }}
			- {{ Catify "I LOVE CATS" }}
			---
			my: cool
			spec:
			- Hello
			- There
			- {{ .Number | quote }}
			- {{ Catify "I LOVE CATS" }}
			`,
			GetInstances: makeInstances,
			Render:       render,
			Setup: func(i Input) (Context, error) {
				return Context{
					Catify: func(s string) string {
						return s
					},
				}, nil
			},
			Options: Options[Input]{
				TabSize: utils.PointerOf(2),
			},
		},
	)
}

func setupComponentFromFileFrontload[T any](
	setup func(input Input) (Context, error),
	frontloadInput Input,
) (Component[T, Input], error) {
	return CreateComponent(
		Def[T, Input, Context]{
			Template:       `../../examples/fromfile/fromfile.yaml`,
			TemplateIsFile: true,
			Options: Options[Input]{
				FrontloadEnabled: true,
				FrontloadInput:   frontloadInput,
			},
			Setup: setup,
		},
	)
}

func setupComponentMultiFrontload[T any](
	makeInstances func(Input, Context) ([]T, error),
	frontloadInput Input,
) (ComponentMulti[T, Input], error) {
	return CreateComponentMulti(
		DefMulti[T, Input, Context]{
			Template: `
			my: cool
			spec:
			- Hello
			- There
			- {{ .Number | quote }}
			- {{ Catify "I LOVE CATS" }}
			---
			my: cool
			spec:
			- Hello
			- There
			- {{ .Number | quote }}
			- {{ Catify "I LOVE CATS" }}
			`,
			GetInstances: makeInstances,
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
				TabSize:          utils.PointerOf(2),
				FrontloadEnabled: true,
				FrontloadInput:   frontloadInput,
			},
		},
	)
}

func TestCreateComponentFromFile(t *testing.T) {
	assert := assert.New(t)

	comp, err := setupComponentFromFile[FromFileSpec](nil)
	assert.Nil(err)

	instance, contents, err := comp.Render(Input{Number: 2})
	assert.Nil(err)
	assert.Len(contents, 65)
	assert.Equal("my: cool\nspec:\n  - Hello\n  - There\n  - \n  - 🐈 I LOVE CATS 🐈", contents)
	assert.Equal([]string{"Hello", "There", "", "🐈 I LOVE CATS 🐈"}, instance.Spec)
}

func TestCreateComponentFromFileFailsOnInvalidUnmarshal(t *testing.T) {
	assert := assert.New(t)
	comp, err := setupComponentFromFile[k8s.DaemonSet](nil)
	assert.Nil(err)

	_, _, err = comp.Render(Input{Number: 2})
	assert.NotNil(err)
	assert.Containsf(err.Error(), "json: unknown field \"my\"", "Expected different error, got %v", err)
}

func TestCreateComponentInline(t *testing.T) {
	assert := assert.New(t)
	comp, err := setupComponentInline[any](`Hello: {{ Catify .Helpa.Number }}`, nil)
	assert.Nil(err)

	_, content, err := comp.Render(Input{Number: 2})
	assert.Nil(err)
	assert.Equal("Hello: 🐈 2 🐈", content)
}

func TestComponentInlineEscape(t *testing.T) {
	assert := assert.New(t)
	comp, err := setupComponentInline[any](`Hello: {{ Catify .Helpa.Number }} {{! .Releases.Some.Path }}`, nil)
	assert.Nil(err)

	_, content, err := comp.Render(Input{Number: 2})
	assert.Nil(err)
	assert.Equal("Hello: 🐈 2 🐈 {{ .Releases.Some.Path }}", content)
}

func TestComponentFrontloadFailsAtInit(t *testing.T) {
	assert := assert.New(t)
	inputAtInit := Input{}
	_, err := setupComponentFromFileFrontload[k8s.Deployment](
		func(input Input) (Context, error) {
			inputAtInit = input
			return Context{
				Catify: func(s string) string {
					return s
				},
			}, nil
		},
		Input{Number: 3},
	)

	assert.NotNilf(err, "Expected error, got %v", err)
	assert.Containsf(err.Error(), "json: unknown field \"my\"", "Expected different error, got %v", err)
	assert.Equal(3, inputAtInit.Number)
}

func TestCreateComponentFromMulti(t *testing.T) {
	assert := assert.New(t)
	comp, err := setupComponentMulti(
		func(Input, Context) ([]FromFileSpec, error) {
			return []FromFileSpec{{}, {}}, nil
		},
		nil,
	)
	assert.Nil(err)

	instances, contents, err := comp.Render(Input{Number: 2})
	assert.Nil(err)
	assert.Len(contents, 2)
	assert.NotEqual("", contents[0])
	assert.NotEqual("", contents[1])
	assert.Len(instances, 2)
	assert.Equal(instances[0].My, "cool")
	assert.Equal(instances[1].My, "cool")
}

func TestCreateComponentFromMultiFailsOnInvalidUnmarshal(t *testing.T) {
	assert := assert.New(t)
	comp, err := setupComponentMulti(
		func(Input, Context) ([]k8s.DaemonSet, error) {
			return []k8s.DaemonSet{{}, {}}, nil
		},
		nil,
	)
	assert.Nil(err)

	_, _, err = comp.Render(Input{Number: 2})
	assert.NotNilf(err, "expected error, got %v", err)
	assert.Containsf(err.Error(), `json: unknown field "my"`, "expected different error, got %v", err)
}

func TestComponentMultiFrontloadFailsAtInit(t *testing.T) {
	assert := assert.New(t)
	inputAtInit := Input{}
	_, err := setupComponentMultiFrontload(
		func(input Input, context Context) ([]k8s.DaemonSet, error) {
			inputAtInit = input
			return []k8s.DaemonSet{{}, {}}, nil
		},
		Input{Number: 3},
	)
	assert.NotNilf(err, "expected error, got %v", err)
	assert.Containsf(err.Error(), `json: unknown field "my"`, "expected different error, got %v", err)
	assert.Equal(3, inputAtInit.Number)
}

func TestComponentRender(t *testing.T) {
	assert := assert.New(t)

	didCallRender := false
	comp, err := setupComponentFromFile(
		func(Input, Context, string) (FromFileSpec, error) {
			didCallRender = true
			return FromFileSpec{
				Spec: []string{
					"My super container",
					"gcr.io/wow-so-great:1",
				},
			}, nil
		},
	)
	assert.Nil(err)

	instance, content, err := comp.Render(Input{Number: 2})
	assert.Nil(err)
	assert.True(didCallRender)
	assert.Len(content, 65)

	// NOTE: Currently, when user overrides the `Render` function, we still keep around the rendered
	// content, so that user may work with it inside the `Render` function.
	// So the `content` var should match the "old" spec.
	assert.Contains(content, "cool")
	// But the `instance` was was returned from the Render function, so that should match
	// the the "new" spec.
	assert.Equal([]string{"My super container", "gcr.io/wow-so-great:1"}, instance.Spec)
}

func TestComponentMultiRender(t *testing.T) {
	assert := assert.New(t)

	didCallInstances := false
	didCallRender := false
	comp, err := setupComponentMulti(
		func(Input, Context) ([]FromFileSpec, error) {
			didCallInstances = true
			return []FromFileSpec{{}, {}}, nil
		},
		func(Input, Context, []string) ([]FromFileSpec, error) {
			didCallRender = true
			return []FromFileSpec{
				{
					Spec: []string{
						"My super container",
						"gcr.io/wow-so-great:1",
					},
				},
			}, nil
		},
	)
	assert.Nil(err)

	instances, contents, err := comp.Render(Input{Number: 2})
	assert.Nil(err)
	assert.True(didCallInstances)
	assert.True(didCallRender)
	assert.Len(contents, 2)
	assert.NotEmpty(contents[0])
	assert.NotEmpty(contents[1])

	// NOTE: Currently, when user overrides the `Render` function, we still keep around the rendered
	// content, so that user may work with it inside the `Render` function.
	// So the `content` var should match the "old" spec.
	assert.Contains(contents[0], "cool")
	// But the `instance` was was returned from the Render function, so that should match
	// the the "new" spec.
	assert.Len(instances, 1)
	assert.Equal([]string{"My super container", "gcr.io/wow-so-great:1"}, instances[0].Spec)
}

func BenchmarkCreateComponentFromMulti(b *testing.B) {
	for i := 0; i < b.N; i++ {
		comp, _ := setupComponentMulti(
			func(Input, Context) ([]k8s.Deployment, error) {
				return []k8s.Deployment{{}, {}}, nil
			},
			nil,
		)
		comp.Render(Input{Number: 2})
	}
}

func TestRender(t *testing.T) {
	assert := assert.New(t)
	content, err := Render(
		"Test1",
		"HelmFn: {{ snakecase .Helpa.Name }}, HelmfileFn: {{ isFile \"lol\" }}",
		Input{Number: 2, Name: "BoB"},
	)
	assert.Nil(err)
	assert.Equal("HelmFn: bo_b, HelmfileFn: false", content)
}
