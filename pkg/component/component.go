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

	preprocess "github.com/jurooravec/helpa/pkg/preprocess"
)

// Component definition
type Def[TType any, TInput any, TContext any] struct {
	Name     string
	Template string
	// If true, the `Template` is evaluated as a path to a template file.
	//
	// If false, `Template` is assumed to be the template itself.
	TemplateIsFile bool
	Setup          func(TInput) TContext
	MakeInstances  func(TInput) (TType, error)
	Options        Options
}

func (i Def[TType, TInput, TContext]) Copy() Def[TType, TInput, TContext] {
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
	// If the document contains lines that contain this separator and nothing else,
	// then the document will be split at these points, and evaluated as a list of
	// smaller documents.
	//
	// Default: `---`
	//
	// See https://yaml.org/spec/1.2.2/#22-structures
	MultiDocSeparator string
}

type Component[TType any, TInput any] struct {
	Render func(input TInput) (instance TType, content string, err error)
}
type ComponentMulti[TType any, TInput any] struct {
	Render func(input TInput) (instances []TType, contents []string, err error)
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
	dec := json.NewDecoder(bytes.NewReader(jsondata))
	dec.DisallowUnknownFields()
	return dec.Decode(container)
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
	comp Def[TType, TInput, TContext],
	input TInput,
) (content string, err error) {
	context := comp.Setup(input)

	funcMap, dataStructInst, err := parseContext(comp.Name, context)
	if err != nil {
		return content, err
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
		return content, fmt.Errorf("parse error in %q: %s", comp.Name, err)
	}

	// Do the actual rendering
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, dataStructInst)
	if err != nil {
		err = fmt.Errorf("render error in %q: %s", comp.Name, err)
		return content, err
	}

	content = strings.Replace(buf.String(), "<no value>", "", -1)

	return content, nil
}

func doUnmarshalOne[TType any, TInput any, TContext any](
	comp Def[TType, TInput, TContext],
	content string,
) (out TType, err error) {
	err = comp.Options.Unmarshal(content, &out)
	if err != nil {
		err = fmt.Errorf("render error in %q: %s", comp.Name, err)
		return out, err
	}

	return out, nil
}

func doUnmarshalMulti[TType any, TInput any, TContext any](
	comp Def[[]TType, TInput, TContext],
	content string,
	out []TType,
) (contentParts []string, err error) {
	// In Helm files, it's common to use `---` to define multiple independent
	// resources. To support that, we try to split the rendered file into an array
	// of docs.
	//
	// NOTE: In such case, the `TType` instance that the user provided should
	// itself be an Array/Slice.
	contentParts = strings.Split(content, comp.Options.MultiDocSeparator)

	// log.Printf("OUT: %v", outUnpacked)

	// Lastly, unmarshal the generated structured data to ensure
	// that they are valid.
	for index, doc := range contentParts {
		err = comp.Options.Unmarshal(doc, &out[index])

		if err != nil {
			err = fmt.Errorf("render error in %q: %s", comp.Name, err)
			return contentParts, err
		}
	}

	return contentParts, nil
}

func doPrepareComponentInput[TType any, TInput any, TContext any](
	comp Def[TType, TInput, TContext],
) (Def[TType, TInput, TContext], error) {
	comp = comp.Copy()

	// Set defaults
	if comp.Options.PreprocessTemplate == nil {
		comp.Options.PreprocessTemplate = preprocessTemplate
	}
	if comp.Options.Unmarshal == nil {
		comp.Options.Unmarshal = unmarshall
	}
	if comp.Options.MultiDocSeparator == "" {
		comp.Options.MultiDocSeparator = "---"
	}

	// Load the template from file
	if comp.TemplateIsFile {
		dat, err := os.ReadFile(comp.Template)
		if err != nil {
			err = fmt.Errorf("error reading file: %s in %q", err, comp.Name)
			return comp, err
		}
		comp.Template = string(dat)
	}

	// Normalize the template
	tmpl, err := comp.Options.PreprocessTemplate(comp.Template)
	if err != nil {
		return Def[TType, TInput, TContext]{}, err
	}
	comp.Template = tmpl

	return comp, nil
}

func CreateComponent[
	TType any,
	TInput any,
	TContext any,
](comp Def[TType, TInput, TContext]) (Component[TType, TInput], error) {
	comp, err := doPrepareComponentInput[TType, TInput, TContext](comp)
	if err != nil {
		if comp.Options.PanicOnError {
			panic(err)
		} else {
			return Component[TType, TInput]{}, err
		}
	}

	// Resulting function is wrapped in a Struct so it's easier to type,
	// so we can use:
	// `Component[TType, TInput].Render`
	//
	// Instead of manually typing:
	// `func(input TInput) (instance TType, content string, err error)`
	component := Component[TType, TInput]{
		Render: func(input TInput) (instance TType, content string, err error) {
			defer func() {
				if !comp.Options.PanicOnError {
					if r := recover(); r != nil {
						err = fmt.Errorf("failed rendering component %q: %v", comp.Name, r)
					}
				}
			}()

			content, err = doRender[TType, TInput, TContext](comp, input)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instance, content, err
				}
			}

			// Unmarshal the generated structured data to ensure that they are valid.
			instance, err = doUnmarshalOne[TType, TInput, TContext](comp, content)
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

	return component, nil
}

func CreateComponentMulti[
	TType any,
	TInput any,
	TContext any,
](comp Def[[]TType, TInput, TContext]) (ComponentMulti[TType, TInput], error) {
	comp, err := doPrepareComponentInput[[]TType, TInput, TContext](comp)
	if err != nil {
		if comp.Options.PanicOnError {
			panic(err)
		} else {
			return ComponentMulti[TType, TInput]{}, err
		}
	}

	// Resulting function is wrapped in a Struct so it's easier to type,
	// so we can use:
	// `ComponentMulti[TType, TInput].Render`
	//
	// Instead of manually typing:
	// `func(input TInput) (instance TType, []contents string, err error)`
	component := ComponentMulti[TType, TInput]{
		Render: func(input TInput) (instances []TType, contents []string, err error) {
			defer func() {
				if !comp.Options.PanicOnError {
					if r := recover(); r != nil {
						err = fmt.Errorf("failed rendering component %q: %v", comp.Name, r)
					}
				}
			}()

			instances, err = comp.MakeInstances(input)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instances, contents, err
				}
			}

			content, err := doRender[[]TType, TInput, TContext](comp, input)

			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instances, contents, err
				}
			}

			// Unmarshal the generated structured data to ensure that they are valid.
			contents, err = doUnmarshalMulti[TType, TInput, TContext](comp, content, instances)

			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instances, contents, err
				}
			}

			return instances, contents, nil
		},
	}

	return component, nil
}
