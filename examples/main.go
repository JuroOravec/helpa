package main

import (
	"log"

	serializers "github.com/jurooravec/helpa/pkg/serializers"
	runtime "k8s.io/apimachinery/pkg/runtime"

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

	// Render Kubernetes Deployment definitions from the template. The definitions
	// are automatically validated as they are unmarshalled and made available as
	// the `deployments` variable.
	deployments, _, err := helm.HelmComponent.Render(helm.Input{Number: 2})
	if err != nil {
		log.Panicf("Error: %v", err)
	}

	for _, deploy := range deployments {
		log.Print(deploy.Kind)
		log.Print(deploy.APIVersion)
		log.Print(deploy.Name)
		log.Print(deploy.Spec.Template.Spec.Containers[0].Image)
		log.Print(deploy.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		// Outputs:
		// Deployment
		// apps/v1
		// kuard
		// gcr.io/kuar-demo/kuard-amd64:1
		// 8080
	}

	checkError := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	// Next, we will export the rendered yaml document(s) into a `templates`
	// directory of a Helm chart, so we can then further work with the templates
	// via Helm.
	//
	// Below, we define which resources should be written to what files.

	// We can either use helpers:
	groups, err := serializers.K8sGroupResourcesBy([]runtime.Object{&deployments[0], &deployments[1]}, "kind")
	checkError(err)

	// Or define the files manually:
	manualGroups := map[string][]runtime.Object{}
	manualGroups["ingress"] = []runtime.Object{&deployments[0]}
	manualGroups["certbot"] = []runtime.Object{&deployments[1]}

	// And once the map is ready, we pass it to HelmChartSerializer, which
	// will do the rest for us.
	err = serializers.HelmChartSerializer(groups, "./templates")
	checkError(err)
}
