package kuard

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestKuardTemplateRendersEmpty(t *testing.T) {
	_, _, err := Component.Render(Input{})
	if err != nil {
		t.Error(err)
	}
}

func TestKuardTemplateRenders(t *testing.T) {
	_, _, err := Component.Render(Input{
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
	})
	if err != nil {
		t.Error(err)
	}
}
