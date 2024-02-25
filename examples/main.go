package main

import (
	"log"

	runtime "k8s.io/apimachinery/pkg/runtime"
	serializers "github.com/jurooravec/helpa/pkg/serializers"

	basic "helpa/examples/basic"
	fromfile "helpa/examples/fromfile"
	helm "helpa/examples/helm"
)

func main() {
	data, content, err := basic.BasicComponent.Render(basic.Input{Number: 2})
	if err != nil {
		log.Panicf("Error: %v", err)
	}

	log.Print(data.Spec[3])

	log.Print(content)
	// Outputs:
	// my: cool
	// spec:
	//   - Hello
	//   - There
	//   - "2"
	//   - 🐈 I LOVE CATS 🐈

	// Same, but template is taken from the file
	_, content, err = fromfile.FileComponent.Render(fromfile.Input{Number: 2})
	if err != nil {
		log.Panicf("Error: %v", err)
	}

	// Render Kubernetes Deployment definition. The rendered definition
	// The definition is also automatically validated by being unmarshalled
	// into the `deployment` variable.
	deployment, content, err := helm.HelmComponent.Render(helm.Input{Number: 2})
	if err != nil {
		log.Panicf("Error: %v", err)
	}

	log.Print(deployment.Kind)
	log.Print(deployment.APIVersion)
	log.Print(deployment.Name)
	log.Print(deployment.Spec.Template.Spec.Containers[0].Image)
	log.Print(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	// Outputs:
	// Deployment
	// apps/v1
	// kuard
	// gcr.io/kuar-demo/kuard-amd64:1
	// 8080

	checkError := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	groups, err := serializers.K8sGroupResourcesBy([]runtime.Object{&deployment, &deployment}, "kind")
	checkError(err)

	err = serializers.HelmChartSerializer(groups, "./templates")
	checkError(err)
}
