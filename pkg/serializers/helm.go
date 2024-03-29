package serializers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	eris "github.com/rotisserie/eris"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

var (
	ErrInvalidGroupByKey = eris.New("InvalidGroupByKey")
)

func K8sGroupResourcesByFunc[T runtime.Object](resources []T, groupBy func(T) (string, error)) (map[string][]T, error) {
	groups := make(map[string][]T)

	for index, resource := range resources {
		key, err := groupBy(resource)
		if err != nil {
			return groups, eris.Wrapf(err, "groupBy getter error at index %v", index)
		}
		groups[key] = append(groups[key], resource)
	}

	return groups, nil
}

// Supported `groupBy` values are "namespace" and "kind"
func K8sGroupResourcesBy[T runtime.Object](resources []T, groupBy string) (map[string][]T, error) {
	groups := make(map[string][]T)

	// Group resources based on the groupBy parameter
	for _, resource := range resources {
		var key string
		switch groupBy {
		case "namespace":
			accessor, err := meta.Accessor(resource)
			if err != nil {
				return groups, eris.Wrap(err, "failed getting namespace accessor")
			}
			key = accessor.GetNamespace()
			if key == "" {
				key = "default" // Assign a default namespace if not specified
			}
		case "kind":
			gvk := resource.GetObjectKind().GroupVersionKind()
			key = strings.ToLower(gvk.Kind)
		default:
			return groups, eris.Wrapf(ErrInvalidGroupByKey, "unsupported groupBy parameter: %s", groupBy)
		}

		groups[key] = append(groups[key], resource)
	}

	return groups, nil
}

func writeK8sResourcesToFile(resourceGroups map[string][]runtime.Object, targetDir string) error {
	groups := make(map[string]string)

	// Serialize
	for key, resources := range resourceGroups {
		serialized := []string{}
		for index, resource := range resources {
			yamlBytes, err := yaml.Marshal(resource)
			if err != nil {
				return eris.Wrapf(err, "failed to marshal resource for file %s at index %v", key, index)
			}
			serialized = append(serialized, string(yamlBytes))
		}

		content := strings.Join(serialized, "\n---\n")

		re := regexp.MustCompile(`\n?[ \t]*creationTimestamp: null[ \t]*\n?`)
		content = re.ReplaceAllString(content, "\n")

		groups[key] = content
	}

	timestamp := time.Now().Format(time.RFC3339)
	comment := fmt.Sprintf("# Autogenerated by Helpa HelmChartSerializer on %s", timestamp)

	// Write groups to files
	for groupName, content := range groups {
		content = strings.Join([]string{comment, content}, "\n")

		filename := filepath.Join(targetDir, fmt.Sprintf("%s.yaml", groupName))
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return eris.Wrapf(err, "failed to write resources to file %s", groupName)
		}
	}

	return nil
}

// Given a target directory and a Map of `template name -> list K8s resources`,
// serialize the resources to YAML and write these resources to files in the given
// directory.
//
// The output is intended to be compatible with Helm chart templates.
func HelmChartSerializer(resources map[string][]runtime.Object, targetDir string) error {
	// See https://stackoverflow.com/a/31151508/9788634
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return eris.Wrapf(err, "failed to create directory at %q", targetDir)
	}

	if err := writeK8sResourcesToFile(resources, targetDir); err != nil {
		return eris.Wrapf(err, "failed to write k8s resources to directory %q", targetDir)
	}

	return nil
}
