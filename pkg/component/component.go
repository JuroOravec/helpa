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

type IDef interface {
}

// Component definition
type Def[TType any, TInput any, TContext any] struct {
	Name     string
	Template string
	// If true, the `Template` is evaluated as a path to a template file.
	//
	// If false, `Template` is assumed to be the template itself.
	TemplateIsFile bool
	// Function that transforms input to context. Functions defined on the context
	// will be made available as template functions. Other context fields will b
	// available as template variables.
	Setup   func(TInput) (TContext, error)
	Options Options
}

func (i Def[TType, TInput, TContext]) Copy() Def[TType, TInput, TContext] {
	// NOTE: Should be sufficient according to https://stackoverflow.com/questions/51635766
	copy := i
	options := i.Options
	copy.Options = options
	return copy
}

// Component definition
type DefMulti[TType any, TInput any, TContext any] struct {
	Name     string
	Template string
	// If true, the `Template` is evaluated as a path to a template file.
	//
	// If false, `Template` is assumed to be the template itself.
	TemplateIsFile bool
	// Function that transforms input to context. Functions defined on the context
	// will be made available as template functions. Other context fields will b
	// available as template variables.
	Setup func(TInput) (TContext, error)
	// When we use ComponentMulti, the component does not know what data types to instantiate
	// for each element in the array/slice. Thus, we need to specify them ourselves here.
	//
	// The component reports error if the size of the Array/Slice does not match
	// the number of instances extracted from the template.
	Instances []TType
	Options   Options
}

func (i DefMulti[TType, TInput, TContext]) Copy() DefMulti[TType, TInput, TContext] {
	// NOTE: Should be sufficient according to https://stackoverflow.com/questions/51635766
	copy := i
	options := i.Options
	instances := i.Instances
	copy.Options = options
	copy.Instances = instances
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

func doRender[TContext any](
	templateName string,
	templateStr string,
	context TContext,
) (content string, err error) {
	funcMap, dataStructInst, err := parseContext(templateName, context)
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

	tmpl := template.New(templateName)
	tmpl.Funcs(engine.FuncMap)

	// This section is based on Helm's code
	if engine.Strict {
		tmpl.Option("missingkey=error")
	} else {
		// Not that zero will attempt to add default values for types it knows,
		// but will still emit <no value> for others. We mitigate that later.
		tmpl.Option("missingkey=zero")
	}

	_, err = tmpl.Parse(templateStr)
	if err != nil {
		return content, fmt.Errorf("parse error in %q: %s", templateName, err)
	}

	// Do the actual rendering
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, dataStructInst)
	if err != nil {
		err = fmt.Errorf("render error in %q: %s", templateName, err)
		return content, err
	}

	content = strings.Replace(buf.String(), "<no value>", "", -1)

	return content, nil
}

func doUnmarshalOne[TType any](
	templateName string,
	content string,
	options Options,
) (out TType, err error) {
	err = options.Unmarshal(content, &out)
	if err != nil {
		err = fmt.Errorf("render error in %q: %s", templateName, err)
		return out, err
	}

	return out, nil
}

func doUnmarshalMulti[TType any](
	templateName string,
	contentParts []string,
	options Options,
	instances []TType,
) (out []TType, err error) {
	// Lastly, unmarshal the generated structured data to ensure
	// that they are valid.
	for index, doc := range contentParts {
		// NOTE: We MUST make a copy of the instance, because the `instances` serve as blueprint.
		// So we must be careful here not to accidentally change state of the `instances` array.
		instance := instances[index]
		err = options.Unmarshal(doc, &instance)
		if err != nil {
			err = fmt.Errorf("render error in %q: %s", templateName, err)
			return out, err
		}
		out = append(out, instance)
	}

	return out, nil
}

func doPrepareComponentInput(
	templateName string,
	templateStr string,
	templateIsFile bool,
	options *Options,
) (string, error) {
	// Set defaults
	if options.PreprocessTemplate == nil {
		options.PreprocessTemplate = preprocessTemplate
	}
	if options.Unmarshal == nil {
		options.Unmarshal = unmarshall
	}
	if options.MultiDocSeparator == "" {
		options.MultiDocSeparator = "---"
	}

	// Load the template from file
	if templateIsFile {
		dat, err := os.ReadFile(templateStr)
		if err != nil {
			err = fmt.Errorf("error reading file: %s in %q", err, templateName)
			return templateStr, err
		}
		templateStr = string(dat)
	}

	// Normalize the template
	templateStr, err := options.PreprocessTemplate(templateStr)
	if err != nil {
		return templateStr, err
	}

	return templateStr, nil
}

func CreateComponent[
	TType any,
	TInput any,
	TContext any,
](comp Def[TType, TInput, TContext]) (Component[TType, TInput], error) {
	comp = comp.Copy()
	tmpl, err := doPrepareComponentInput(comp.Name, comp.Template, comp.TemplateIsFile, &comp.Options)
	if err != nil {
		if comp.Options.PanicOnError {
			panic(err)
		} else {
			return Component[TType, TInput]{}, err
		}
	}
	comp.Template = tmpl

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

			context, err := comp.Setup(input)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instance, content, err
				}
			}

			content, err = doRender[TContext](comp.Name, comp.Template, context)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instance, content, err
				}
			}

			// Unmarshal the generated structured data to ensure that they are valid.
			instance, err = doUnmarshalOne[TType](comp.Name, content, comp.Options)
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
](comp DefMulti[TType, TInput, TContext]) (ComponentMulti[TType, TInput], error) {
	comp = comp.Copy()
	tmpl, err := doPrepareComponentInput(comp.Name, comp.Template, comp.TemplateIsFile, &comp.Options)
	if err != nil {
		if comp.Options.PanicOnError {
			panic(err)
		} else {
			return ComponentMulti[TType, TInput]{}, err
		}
	}
	comp.Template = tmpl

	// Resulting function is wrapped in a Struct so it's easier to type,
	// so we can use:
	// `ComponentMulti[TType, TInput].Render`
	//
	// Instead of manually typing:
	// `func(input TInput) (instance TType, []contentParts string, err error)`
	component := ComponentMulti[TType, TInput]{
		Render: func(input TInput) (instances []TType, contentParts []string, err error) {
			defer func() {
				if !comp.Options.PanicOnError {
					if r := recover(); r != nil {
						err = fmt.Errorf("failed rendering component %q: %v", comp.Name, r)
					}
				}
			}()

			context, err := comp.Setup(input)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instances, contentParts, err
				}
			}

			content, err := doRender[TContext](comp.Name, comp.Template, context)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instances, contentParts, err
				}
			}

			// In Helm files, it's common to use `---` to define multiple independent
			// resources. To support that, we try to split the rendered file into an array
			// of docs.
			//
			// NOTE: In such case, the `TType` instance that the user provided should
			// itself be an Array/Slice.
			contentParts = strings.Split(content, comp.Options.MultiDocSeparator)

			// Allow the author of the component to specify exact instances that should be populated
			// with the extracted data. This way, they can specify an interface for the instances' type,
			// and then create homogenous array of specific length (assuming all elements implement
			// the interface).
			//
			// But if author didn't specify this array,
			if len(comp.Instances) != len(contentParts) {
				err = fmt.Errorf("found %v documents in the template, but there is %v instances to unmarshal the data to. These must match. Review the component's `Instances` field and its template", len(contentParts), len(comp.Instances))
				return instances, contentParts, err
			}

			// Unmarshal the generated structured data to ensure that they are valid.
			instances, err = doUnmarshalMulti[TType](comp.Name, contentParts, comp.Options, comp.Instances)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instances, contentParts, err
				}
			}

			return instances, contentParts, nil
		},
	}

	return component, nil
}
