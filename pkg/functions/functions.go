package functions

import (
	"log"
	"strings"

	sprig "github.com/Masterminds/sprig"
	yaml "sigs.k8s.io/yaml"
)

var indentFn func(spaces int, v string) string

func init() {
	if fn, ok := sprig.TxtFuncMap()["indent"].(func(spaces int, v string) string); ok {
		indentFn = fn
	} else {
		log.Panicf("failed to prepare the 'indent' function from Sprig: Not a function")
	}
}

// Same as Sprig's `Indent`, except the first line is NOT indented
func IndentRest(spaces int, v string) string {
	headAndRest := strings.SplitN(v, "\n", 2)

	// Skip if there are no newlines in the text
	if len(headAndRest) <= 1 {
		return v
	}

	return strings.Join([]string{
		headAndRest[0],
		indentFn(spaces, headAndRest[1]),
	}, "\n")
}

func YamlToJson(v string) (string, error) {
	jsondata, err := yaml.YAMLToJSON([]byte(v))
	return string(jsondata), err
}

func JsonToYaml(v string) (string, error) {
	jsondata, err := yaml.JSONToYAML([]byte(v))
	return string(jsondata), err
}
