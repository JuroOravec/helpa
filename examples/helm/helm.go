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
var HelmComponent helpa.Component[appsv1.Deployment, Input]

func init() {
	err := error(nil)
	// Each component must define 3 types: Spec, Input, Context
	HelmComponent, err = helpa.CreateComponentFromFile[appsv1.Deployment, Input, Context](
		helpa.Def[Input, Context]{
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
			Setup: func(input Input) Context {
				return Context{
					Number: input.Number,
					Catify: func(s string) string {
						return fmt.Sprintf("🐈 %s 🐈", s)
					},
				}
			},
			// The template uses Helm's renderer, which is based on `text/template`.
			// Hence, you will find most of Helm's functions like `toYaml`.
			Template: `./helm/helm.yaml`,
		})

	if err != nil {
		log.Panic(err)
	}
}
