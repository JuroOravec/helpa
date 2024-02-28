package helm

import (
	"log"

	helpa "github.com/jurooravec/helpa/pkg/component"
	helpaUtils "github.com/jurooravec/helpa/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// Use input to cetralize all inputs and variables that the underlying template
// should use.
type Input struct {
	Name      string
	Container corev1.Container
	Port      corev1.ContainerPort
}

// Each component defines a `Setup` function that describes how the `Input`
// is transformed. The result of the `Setup` function is what becomes available
// as variables and as functions in the template.
type Context struct {
	Input
}

// To make it easy to import this component already configured, we declare
// the variable and then populated it in the `init` function.
// See https://tutorialedge.net/golang/the-go-init-function/
//
// NOTE: We use `runtime.Object` to indicate that, after this component renders the component,
// it should return an array of runtime.Objects, which is an interface referring to k8s resources.
var Component helpa.ComponentMulti[runtime.Object, Input]


func ChartDefaults() Input {
	return Input{
		Name: "kuard",
		Container: corev1.Container{
			Name:            "kuard",
			Image:           "gcr.io/kuar-demo/kuard-amd64:1",
			ImagePullPolicy: "Always",
		},
		Port: corev1.ContainerPort{
			ContainerPort: 8080,
			Protocol:      "TCP",
		},
	}
}

func init() {
	err := error(nil)

	// IMPORTANT: Notice that this component uses `CreateComponentMulti` and `DefMulti`.
	// This is because the template in `helm.yaml` actually define multiple documents,
	// separated by `---`.
	Component, err = helpa.CreateComponentMulti(
		helpa.DefMulti[runtime.Object, Input, Context]{
			Name: "Kuard",
			// The template uses Helm's renderer, which is based on `text/template`.
			// Hence, you will find most of Helm's functions like `toYaml`.
			Template:       `./helm/helm.yaml`,
			TemplateIsFile: true,
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
				// Apply defaults to the given inputs
				inputCopy := input
				err = helpaUtils.ApplyDefaults(&inputCopy, ChartDefaults())
				
				return Context{Input: inputCopy}, err
			},
			// IMPORTANT: `GetInstances` is specific to `ComponentMulti`. Since each document
			// in the template can represent a different data structure, we have to specify
			// the exact data type (via empty instances) for each item.
			//
			// Component reports error when the number of documents in the template and the
			// number of provided instances does not match.
			GetInstances: func(input Input, context Context) ([]runtime.Object, error) {
				// This says that our template defines one Deployment, and one Service,
				// in this order.
				instances := []runtime.Object{
					&appsv1.Deployment{},
					&corev1.Service{},
				}
				return instances, nil
			},
		})

	if err != nil {
		log.Panic(err)
	}
}
