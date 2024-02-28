package certbot

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestCertbotTemplateRendersEmpty(t *testing.T) {
	_, _, err := Component.Render(Input{})
	if err != nil {
		t.Error(err)
	}
}

func TestCertbotTemplateRenders(t *testing.T) {
	_, _, err := Component.Render(Input{
		RunImmediately:      true,
		CertbotNamespace:    "certbot",
		CertbotCronSchedule: "20 3 * * */6", // Every 6th day-of-week at 03:20
		CertbotCmdArgs:      "certonly",
		CertbotContactEmail: "",
		TlsSecretName:       "certbot-tls-secret",
		TlsSecretNamespaces: []string{"default"},
		CertbotContainer: corev1.Container{
			Image:           "certbot/certbot",
			ImagePullPolicy: "Always",
		},
		CertbotImagePullSecrets: []corev1.LocalObjectReference{},
		// Certificate for root, e.g. example.com, or wildcard for all subdomains, e.g. *.example.com
		Domain: Domain{
			Domain:     "example.com",
			Subdomains: []string{"*.example.com"},
		},
	})
	if err != nil {
		t.Error(err)
	}
}
