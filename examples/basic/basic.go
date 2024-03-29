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

// Use input to cetralize all inputs and variables that the underlying template
// should use/
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
var BasicComponent helpa.Component[Spec, Input]

func init() {
	err := error(nil)

	// Each component must define 3 types: Spec, Input, Context
	BasicComponent, err = helpa.CreateComponent(
		helpa.Def[Spec, Input, Context]{
			Name: "BasicComponent",
			// Configure behavour
			Options: helpa.Options[Input]{
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
			// The template uses Helm's renderer, which is based on `text/template`.
			// Hence, you will find most of Helm's functions like `toYaml`.
			Template: `
            my: cool
            spec:
              - Hello
              - There
              - {{ .Number | quote }}
              - {{ Catify "I LOVE CATS" }}
            `,
		})

	if err != nil {
		log.Panic(err)
	}
}
