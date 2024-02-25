package component

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	template "text/template"

	reflections "github.com/oleiade/reflections"
	dynamicstruct "github.com/ompluscator/dynamic-struct"
	templateEngine "k8s.io/helm/pkg/engine"
	yaml "sigs.k8s.io/yaml"

	preprocess "ceiondis/helpa/lib/component/preprocess"
)

// Component definition
type Def[TInput any, TContext any] struct {
	Name     string
	Template string
	Setup    func(TInput) TContext
	Options  Options
}

func (i Def[TInput, TContext]) Copy() Def[TInput, TContext] {
	// NOTE: Should be sufficient according to https://stackoverflow.com/questions/51635766
	copy := i
	options := i.Options
	copy.Options = options
	return copy
}

// Component options
type Options struct {
	// By default, any errors are returned as result tuple. If you want to panic
	// on errors and don't want to handle errors every time, set this to `true`.
	PanicOnError bool
	// By default, the templates have leading/trailing empty lines shaven, and
	// indentation is normalized. See more in the `lib/component/preprocess` package.
	//
	// Use this option to define custom preprocessing, or disable the default one.
	PreprocessTemplate func(tmpl string) (string, error)
	// By default, templates are assumed to be YAML, and unmarshalled with yaml.Unmarshall.
	//
	// Use this option to if you want to modify the rendered template before unmarshalling it,
	// or if you want to use different data types like JSON, TOML, etc.
	Unmarshal func(rendered string, container any) error
	// err = yaml.Unmarshal([]byte(content), &instance)
}

type Component[TType any, TInput any] struct {
	Render func(input TInput) (instance TType, content string, err error)
}

func isFunc(v any) bool {
	return reflect.TypeOf(v).Kind() == reflect.Func
}

func preprocessTemplate(tmpl string) (string, error) {
	tmpl, err := preprocess.TrimTemplate(tmpl)
	if err != nil {
		return tmpl, err
	}
	tmpl = preprocess.Unindent(tmpl)
	return tmpl, nil
}

func unmarshall(rendered string, container any) error {
	jsondata, err := yaml.YAMLToJSON([]byte(rendered))
	if err != nil {
		return err
	}
	return json.Unmarshal(jsondata, container)
}

// Process the fields in Context.
//
// If a field is a function, it will be made available as template function.
// If it's a non-func, we will expose it as a template variable.
//
// To do the latter, though, we need to create a new Struct with only non-func
// fields. So we build it dynamically.
func parseContext(
	compName string,
	context any,
) (template.FuncMap, any, error) {
	funcMap := template.FuncMap{}

	structBuilder := dynamicstruct.NewStruct()
	structItems, err := reflections.Items(context)
	if err != nil {
		return funcMap, nil, fmt.Errorf("failed to process context in %q: %s", compName, err)
	}

	varMap := map[string]any{}
	for key, val := range structItems {
		// Pass functions to the engine's FuncMap, so users may call them as
		// `{{ MyFunc arg1 arg2 }}`
		if isFunc(val) {
			funcMap[key] = val
			continue
		}

		// NOTE: AddField infers correct type from the variable that's given.
		structBuilder = structBuilder.AddField(key, val, "")
		varMap[key] = val
	}

	// See https://github.com/Ompluscator/dynamic-struct#add-new-struct
	dataStructInst := structBuilder.Build().New()

	// The above only created an empty struct, but we still need to populate it
	for key, val := range varMap {
		err = reflections.SetField(dataStructInst, key, val)
		if err != nil {
			return funcMap, dataStructInst, fmt.Errorf("failed to create data struct in %q: %s", compName, err)
		}
	}

	return funcMap, dataStructInst, nil
}

func doRender[TType any, TInput any, TContext any](
	comp Def[TInput, TContext],
	input TInput,
) (instance TType, content string, err error) {
	context := comp.Setup(input)

	funcMap, dataStructInst, err := parseContext(comp.Name, context)
	if err != nil {
		return instance, "", err
	}

	// Using the Engine struct from Helm package ensures that we use all the same
	// functions as they do (with a few exceptions).
	engine := templateEngine.New()

	// Set user-defined functions. These may override the defaults, but this should NOT
	// happen, as the defaults are defined in lowercase. Since our custom fields
	// come from public fields, they SHOULD be all PascalCase.
	for key, val := range funcMap {
		engine.FuncMap[key] = val
	}

	tmpl := template.New(comp.Name)
	tmpl.Funcs(engine.FuncMap)

	// This section is based on Helm's code
	if engine.Strict {
		tmpl.Option("missingkey=error")
	} else {
		// Not that zero will attempt to add default values for types it knows,
		// but will still emit <no value> for others. We mitigate that later.
		tmpl.Option("missingkey=zero")
	}

	_, err = tmpl.Parse(comp.Template)
	if err != nil {
		return instance, "", fmt.Errorf("parse error in %q: %s", comp.Name, err)
	}

	// Do the actual rendering
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, dataStructInst)
	if err != nil {
		err = fmt.Errorf("render error in %q: %s", comp.Name, err)
		return instance, "", err
	}

	content = strings.Replace(buf.String(), "<no value>", "", -1)

	// Lastly, unmarshal the generated structured data to ensure
	// that they are valid.
	err = comp.Options.Unmarshal(content, &instance)
	if err != nil {
		err = fmt.Errorf("render error in %q: %s", comp.Name, err)
		return instance, "", err
	}

	return instance, content, nil
}

func CreateComponent[
	TType any,
	TInput any,
	TContext any,
](comp Def[TInput, TContext]) Component[TType, TInput] {
	comp = comp.Copy()

	// Set defaults
	if comp.Options.PreprocessTemplate == nil {
		comp.Options.PreprocessTemplate = preprocessTemplate
	}
	if comp.Options.Unmarshal == nil {
		comp.Options.Unmarshal = unmarshall
	}

	// Resulting function is wrapped in a Struct so it's easier to type,
	// so we can use:
	// `Component[TType, TInput].Render`
	//
	// Instead of manually typing:
	// `func(input TInput) (instance TType, content string, err error)`
	return Component[TType, TInput]{
		Render: func(input TInput) (instance TType, content string, err error) {
			defer func() {
				if !comp.Options.PanicOnError {
					if r := recover(); r != nil {
						err = fmt.Errorf("failed rendering component %q: %v", comp.Name, r)
					}
				}
			}()

			tmpl, err := comp.Options.PreprocessTemplate(comp.Template)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instance, content, err
				}
			}
			comp.Template = tmpl

			instance, content, err = doRender[TType, TInput, TContext](comp, input)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instance, content, err
				}
			}

			return instance, content, nil
		},
	}
}

// Same as `CreateComponent`, except that the `Template` field of
// `CreateComponentFromFile` is a file path to a template file that will
// be loaded.
func CreateComponentFromFile[
	TType any,
	TInput any,
	TContext any,
](comp Def[TInput, TContext]) (Component[TType, TInput], error) {
	dat, err := os.ReadFile(comp.Template)
	if err != nil {
		err = fmt.Errorf("error reading file in %q: %s", comp.Name, err)
		return Component[TType, TInput]{}, err
	}

	compCopy := comp.Copy()
	compCopy.Template = string(dat)

	return CreateComponent[TType, TInput](compCopy), nil
}
