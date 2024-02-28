package ingress

import (
	"log"

	component "github.com/jurooravec/helpa/pkg/component"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

type Input struct {
	Name  string
	Rules []netv1.IngressRule
}

type Context struct {
	Input
}

var Component component.ComponentMulti[runtime.Object, Input]

func init() {
	err := error(nil)
	Component, err = component.CreateComponentMulti(
		component.DefMulti[runtime.Object, Input, Context]{
			Name:           "Ingress",
			Template:       `./helmchart/src/ingress/ingress.yaml`,
			TemplateIsFile: true,
			Setup: func(input Input) (Context, error) {
				return Context{Input: input}, nil
			},
			GetInstances: func(input Input, context Context) ([]runtime.Object, error) {
				instances := []runtime.Object{
					&netv1.Ingress{},
				}
				return instances, nil
			},
		})

	if err != nil {
		log.Panic(err)
	}
}

type IngressRule struct {
	Host  string
	Paths []IngressRulePath
}

type IngressRulePath struct {
	Path        string
	ServiceName string
	ServicePort int32
}

func CreatePrefixIngressRule(input IngressRule) netv1.IngressRule {
	pathType := netv1.PathType("Prefix")

	return netv1.IngressRule{
		Host: input.Host,
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: lo.Map(input.Paths, func(path IngressRulePath, _ int) netv1.HTTPIngressPath {
					return netv1.HTTPIngressPath{
						PathType: &pathType,
						Path:     path.Path,
						Backend: CreateServiceIngressBackend(netv1.IngressServiceBackend{
							Name: path.ServiceName,
							Port: netv1.ServiceBackendPort{
								Number: path.ServicePort,
							},
						}),
					}
				}),
			},
		},
	}
}

// NOTE: Service and Resource backends are mutually exclusive
func CreateServiceIngressBackend(input netv1.IngressServiceBackend) netv1.IngressBackend {
	return netv1.IngressBackend{
		Service: &input,
	}
}

// NOTE: Service and Resource backends are mutually exclusive
func CreateResourceIngressBackend(input corev1.TypedLocalObjectReference) netv1.IngressBackend {
	return netv1.IngressBackend{
		Resource: &input,
	}
}
