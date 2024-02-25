package main

import (
	"log"

	runtime "k8s.io/apimachinery/pkg/runtime"
	serializers "github.com/jurooravec/helpa/pkg/serializers"
	appsv1 "k8s.io/api/apps/v1"

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

	// We expect to find 2 deployments in the rendered document
	deployments := []appsv1.Deployment{{}, {}}
	_, err = helm.HelmComponent.RenderMulti(helm.Input{Number: 2}, &deployments)
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

	// In the next, we will export the rendered yaml document(s) into a `templates`
	// directory of a Helm chart.

	// Below, we define which resources should be written to what files
	
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
