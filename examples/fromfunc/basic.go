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
// should use.
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
var FromFuncComponent helpa.Component[Spec, Input]
var FromFuncComponentMulti helpa.ComponentMulti[Spec, Input]

func init() {
	err := error(nil)

	// Each component must define 3 types: Spec, Input, Context
	FromFuncComponent, err = helpa.CreateComponent(
		helpa.Def[Spec, Input, Context]{
			Name: "FromFuncComponent",
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
			// Instead of defining a textual template, this component has a `Render`
			// method that returns the expected data, bypassing the need for the template.
			Render: func(input Input, context Context, content string) (Spec, error) {
				spec := Spec{
					My: "cool - but changed",
					Spec: []string{
						"Hello",
						"There",
						fmt.Sprint(context.Number),
						context.Catify("I LOVE CATS"),
					},
				}
				return spec, nil
			},
		})

	// Same as above, but using the `CreateComponentMulti` variant
	FromFuncComponentMulti, err = helpa.CreateComponentMulti(
		helpa.DefMulti[Spec, Input, Context]{
			Name: "FromFuncComponentMulti",
			Setup: func(input Input) (Context, error) {
				context := Context{
					Number: input.Number,
					Catify: func(s string) string {
						return fmt.Sprintf("🐈 %s 🐈", s)
					},
				}
				return context, nil
			},
			Render: func(input Input, context Context, contentParts []string) ([]Spec, error) {
				specs := []Spec{
					{
						My: "cool - but changed",
						Spec: []string{
							"Hello",
							"There",
							fmt.Sprint(context.Number),
							context.Catify("I LOVE CATS"),
						},
					},
					{
						My: "cool - but changed",
						Spec: []string{
							"Hello",
							"There",
							fmt.Sprint(context.Number),
							context.Catify("I LOVE CATS"),
						},
					},
				}
				return specs, nil
			},
		})

	if err != nil {
		log.Panic(err)
	}
}
