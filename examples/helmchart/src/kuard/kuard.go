package kuard

import (
	"log"

	component "github.com/jurooravec/helpa/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

type Input struct {
	Name      string
	Container corev1.Container
	Port      corev1.ContainerPort
}

type Context struct {
	Input
}

var Component component.ComponentMulti[runtime.Object, Input]

func init() {
	err := error(nil)
	Component, err = component.CreateComponentMulti(
		component.DefMulti[runtime.Object, Input, Context]{
			Name:           "Kuard",
			Template:       `./helmchart/src/kuard/kuard.yaml`,
			TemplateIsFile: true,
			Setup: func(input Input) (Context, error) {
				return Context{Input: input}, nil
			},
			GetInstances: func(input Input, context Context) ([]runtime.Object, error) {
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
