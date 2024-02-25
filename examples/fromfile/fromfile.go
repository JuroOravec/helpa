package basic

import (
	"fmt"
	"log"

	helpa "github.com/jurooravec/helpa/pkg/component"
)

// The templates are expected to represent structured data like YAML or JSON
// Hence, each component defines a "Spec" - a data type describing that data.
//
// After the template is rendered, it is Unmarshaled into this Spec to ensure
// that the templated data is valid.
type Spec struct {
	My   string   `json:"my"`
	Spec []string `json:"spec"`
}

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
var FileComponent helpa.Component[Spec, Input]

func init() {
	err := error(nil)

	// Each component must define 3 types: Spec, Input, Context
	FileComponent, err = helpa.CreateComponent[Spec, Input, Context](
		helpa.Def[Spec, Input, Context]{
			Name: "FileComponent",
			// The template uses Helm's renderer, which is based on `text/template`.
			// Hence, you will find most of Helm's functions like `toYaml`.
			Template:       `./fromfile/fromfile.yaml`,
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
				return context, err
			},
		},
	)

	if err != nil {
		log.Panic(err)
	}
}
