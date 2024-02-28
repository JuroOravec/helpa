package ingress

import (
	"testing"

	netv1 "k8s.io/api/networking/v1"
)

func TestIngressTemplateRendersEmpty(t *testing.T) {
	_, _, err := Component.Render(Input{})
	if err != nil {
		t.Error(err)
	}
}

func TestIngressTemplateRenders(t *testing.T) {
	_, _, err := Component.Render(Input{
		Name: "ingress",
		Rules: []netv1.IngressRule{
			CreatePrefixIngressRule(IngressRule{
				Host: "chart-example.local",
				Paths: []IngressRulePath{
					{Path: "/", ServiceName: "kuard", ServicePort: 8080},
				},
			}),
		},
	})
	if err != nil {
		t.Error(err)
	}
}
