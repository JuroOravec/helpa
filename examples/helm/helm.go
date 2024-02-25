package helm

import (
	"fmt"
	"log"

	appsv1 "k8s.io/api/apps/v1"

	helpa "github.com/jurooravec/helpa/pkg/component"
)

// Tracking variables across templates can be hell.
// Instead, each component defines its inputs.
type Input struct {
	Number int
}

// Each component defines a `Setup` function that describes how the `Input`
// is transformed. The result of the `Setup` function is what becomes available
// as variables and as functions in the template.
//
// In this example, we expose a variable `Number`, and a function `Catify`
type Context struct {
	Number int
	Catify func(s string) string
}

// To make it easy to import this component already configured, we declare
// the variable and then populated it in the `init` function.
// See https://tutorialedge.net/golang/the-go-init-function/
var HelmComponent helpa.ComponentMulti[appsv1.Deployment, Input]

func init() {
	err := error(nil)

	// IMPORTANT: Notice that this component uses `CreateComponentMulti` and `DefMulti`.
	// This is because the template in `helm.yaml` actually define multiple documents,
	// separated by `---`.
	HelmComponent, err = helpa.CreateComponentMulti[appsv1.Deployment, Input, Context](
		helpa.DefMulti[appsv1.Deployment, Input, Context]{
			Name: "HelmComponent",
			// The template uses Helm's renderer, which is based on `text/template`.
			// Hence, you will find most of Helm's functions like `toYaml`.
			Template:       `./helm/helm.yaml`,
			TemplateIsFile: true,
			// Configure behavour
			Options: helpa.Options{
				// PanicOnError: false,
				// Unmarshal: func() {...},
			},
			// Transform `Input` into `Context`.
			//
			// Context's fields that are functions will be loaded into
			// the template engine, and may be called as
			// `{{ MyFunction arg1 arg2 }}`
			//
			// Other Context's fields are made available as variables, e.g.
			// `{{ .MyVariable }}`
			Setup: func(input Input) (Context, error) {
				context := Context{
					Number: input.Number,
					Catify: func(s string) string {
						return fmt.Sprintf("🐈 %s 🐈", s)
					},
				}
				return context, nil
			},
			// IMPORTANT: This is a field specific to ComponentMulti. Since each document
			// in the template can represent a different data structure, we have to specify
			// the exact data type (via empty instances) for each item.
			//
			// Component reports error when the number of documents in the template and the
			// number of provided instances does not match.
			GetInstances: func(input Input) ([]appsv1.Deployment, error) {
				return []appsv1.Deployment{{}, {}}, nil
			},
		})

	if err != nil {
		log.Panic(err)
	}
}
