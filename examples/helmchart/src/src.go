package src

import (
	serializers "github.com/jurooravec/helpa/pkg/serializers"
	helpaUtils "github.com/jurooravec/helpa/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	certbot "helpa/examples/helmchart/src/certbot"
	ingress "helpa/examples/helmchart/src/ingress"
	kuard "helpa/examples/helmchart/src/kuard"
)

type ChartInput struct {
	CertbotInput   certbot.Input
	CertbotEnabled bool
	KuardInput     kuard.Input
	IngressInput   ingress.Input
}

func ChartDefaults() ChartInput {
	return ChartInput{
		CertbotEnabled: true,
		CertbotInput: certbot.Input{
			RunImmediately:      false,
			CertbotNamespace:    "certbot",
			CertbotCronSchedule: "20 3 * * */6", // Every 6th day-of-week at 03:20
			CertbotCmdArgs:      "certonly",
			CertbotContactEmail: "",
			TlsSecretName:       "certbot-tls-secret",
			TlsSecretNamespaces: []string{"default"},
			CertbotContainer: corev1.Container{
				Name:            "certbot",
				Image:           "certbot/certbot",
				ImagePullPolicy: "Always",
			},
			CertbotImagePullSecrets: []corev1.LocalObjectReference{},
			// Certificate for root, e.g. example.com, or wildcard for all subdomains, e.g. *.example.com
			Domain: certbot.Domain{
				Domain:     "example.com",
				Subdomains: []string{"*.example.com"},
			},
		},
		KuardInput: kuard.Input{
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
		},
		IngressInput: ingress.Input{
			Name: "ingress",
			Rules: []netv1.IngressRule{
				ingress.CreatePrefixIngressRule(ingress.IngressRule{
					Host: "chart-example.local",
					Paths: []ingress.IngressRulePath{
						{Path: "/", ServiceName: "kuard", ServicePort: 8080},
					},
				}),
			},
		},
	}
}

func RenderTemplates(input ChartInput, outdir string) (err error) {
	inputCopy := input
	err = helpaUtils.ApplyDefaults(&inputCopy, ChartDefaults())
	if err != nil {
		return err
	}

	var certbotSpecs []runtime.Object
	if inputCopy.CertbotEnabled {
		certbotSpecs, _, err = certbot.Component.Render(inputCopy.CertbotInput)
		if err != nil {
			return err
		}
	}

	kuardSpecs, _, err := kuard.Component.Render(inputCopy.KuardInput)
	if err != nil {
		return err
	}

	ingressSpecs, _, err := ingress.Component.Render(inputCopy.IngressInput)
	if err != nil {
		return err
	}

	outfiles := map[string][]runtime.Object{
		"certbot": certbotSpecs,
		"kuard":   kuardSpecs,
		"ingress": ingressSpecs,
	}

	err = serializers.HelmChartSerializer(outfiles, outdir)
	if err != nil {
		return err
	}

	return nil
}
