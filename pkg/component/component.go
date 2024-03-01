package component

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	template "text/template"

	eris "github.com/rotisserie/eris"
	helmfile "github.com/helmfile/helmfile/pkg/tmpl"
	reflections "github.com/oleiade/reflections"
	dynamicstruct "github.com/ompluscator/dynamic-struct"
	templateEngine "k8s.io/helm/pkg/engine"
	yaml "sigs.k8s.io/yaml"

	functions "github.com/jurooravec/helpa/pkg/functions"
	preprocess "github.com/jurooravec/helpa/pkg/preprocess"
)

var (
	ErrComponentRenderResultMismatch = eris.New("number of instances extracted from the rendered template does not match the number of declared instances in `GetInstances`")
)

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
	Render  func(input TInput, context TContext, content string) (TType, error)
	Options Options[TInput]
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
	GetInstances func(input TInput, context TContext) ([]TType, error)
	Render       func(input TInput, context TContext, contentParts []string) ([]TType, error)
	Options      Options[TInput]
}

func (i DefMulti[TType, TInput, TContext]) Copy() DefMulti[TType, TInput, TContext] {
	// NOTE: Should be sufficient according to https://stackoverflow.com/questions/51635766
	copy := i
	options := i.Options
	copy.Options = options
	return copy
}

// Component options
type Options[TInput any] struct {
	// By default, any errors are returned as result tuple. If you want to panic
	// on errors and don't want to handle errors every time, set this to `true`.
	PanicOnError bool
	// By default, the templates have leading/trailing empty lines shaven, and
	// indentation is normalized. See more in the `lib/component/preprocess` package.
	//
	// Use this option to define custom preprocessing, or disable the default one.
	PreprocessTemplate func(tmpl string, options Options[TInput]) (string, error)
	// By default, templates are assumed to be YAML, and unmarshalled with yaml.Unmarshall.
	//
	// Use this option to if you want to modify the rendered template before unmarshalling it,
	// or if you want to use different data types like JSON, TOML, etc.
	Unmarshal func(rendered string, container any, options Options[TInput]) error
	// If the document contains lines that contain this separator and nothing else,
	// then the document will be split at these points, and evaluated as a list of
	// smaller documents.
	//
	// Default: `---`
	//
	// See https://yaml.org/spec/1.2.2/#22-structures
	MultiDocSeparator string
	// Optionally replace tabs with spaces.
	//
	// NOTE: This is required if you're using tabs and generating YAML files. Because
	// YAML cannot process tabs.
	TabSize *int
	// Check integrity of textual templates at component creation.
	//
	// If frontloading is enabled, we will make a dummy call to the `component.Render`
	// method at component creation, to ensure that everything works correctly,
	// especially the unmarshalling of a textual template.
	//
	// Frontloading should be OFF in production, and ON for development and testing.
	FrontloadEnabled bool
	// Configure the input for the frontloading call.
	FrontloadInput TInput
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

func genCustomFuncMap() template.FuncMap {
	return template.FuncMap{
		"indentRest": functions.IndentRest,
		"yamlToJson": functions.YamlToJson,
		"jsonToYaml": functions.JsonToYaml,
	}
}

func defaultPreprocessor[TInput any](tmpl string, opts Options[TInput]) (string, error) {
	tmpl, err := preprocess.TrimTemplate(tmpl)
	if err != nil {
		return tmpl, eris.Wrap(err, "failed to trim whitespace from template")
	}

	if opts.TabSize != nil {
		tmpl = strings.ReplaceAll(tmpl, "\t", strings.Repeat(" ", *opts.TabSize))
	}

	tmpl = preprocess.Unindent(tmpl)
	return tmpl, nil
}

func defaultUnmarshaller[TInput any](rendered string, container any, opts Options[TInput]) error {
	jsondata, err := yaml.YAMLToJSON([]byte(rendered))
	if err != nil {
		return eris.Wrap(err, "failed to convert rendered template from YAML to JSON")
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
		return funcMap, nil, eris.Wrapf(err, "failed to process context in %q", compName)
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
			return funcMap, dataStructInst, eris.Wrapf(err, "failed to create data struct in %q", compName)
		}
	}

	return funcMap, dataStructInst, nil
}

func Render[TContext any](
	templateName string,
	templateStr string,
	context TContext,
) (content string, err error) {
	funcMap, dataStructInst, err := parseContext(templateName, context)
	if err != nil {
		return content, eris.Wrapf(err, "failed to process context in component %q", templateName)
	}

	// "Namespace" all the variables from user's component under the "Helpa" key
	// so they are accessed as:
	// {{ .Helpa.MyValue }}
	data := map[string]any{}
	data["Helpa"] = dataStructInst

	// Using the Engine struct from Helm package ensures that we use all the same
	// functions as they do (with a few exceptions).
	// See https://helm.sh/docs/chart_template_guide/function_list/
	engine := templateEngine.New()
	for key, val := range engine.FuncMap {
		funcMap[key] = val
	}

	// Similarly we use generate FuncMap for Helmfile's functions
	// See https://helmfile.readthedocs.io/en/latest/templating_funcs/#env
	// and https://github.com/helmfile/helmfile/blob/main/pkg/tmpl/context_funcs.go
	helmfileCtx := helmfile.Context{}
	helmfileFuncMap := helmfileCtx.CreateFuncMap()
	for key, val := range helmfileFuncMap {
		funcMap[key] = val
	}

	// Set our own custom functions
	customFuncs := genCustomFuncMap()
	for key, val := range customFuncs {
		funcMap[key] = val
	}

	tmpl := template.New(templateName)
	tmpl.Funcs(funcMap)

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
		return content, eris.Wrapf(err, "parse error in %q", templateName)
	}

	// Do the actual rendering
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		err = eris.Wrapf(err, "render error in %q", templateName)
		return content, err
	}

	content = strings.Replace(buf.String(), "<no value>", "", -1)

	return content, nil
}

func doUnmarshalOne[TType any, TInput any](
	templateName string,
	content string,
	options Options[TInput],
) (out TType, err error) {
	err = options.Unmarshal(content, &out, options)
	if err != nil {
		err = eris.Wrapf(err, "render error in %q", templateName)
		return out, err
	}

	return out, nil
}

func doUnmarshalMulti[TType any, TInput any](
	templateName string,
	contentParts []string,
	options Options[TInput],
	instances []TType,
) (out []TType, err error) {
	// Lastly, unmarshal the generated structured data to ensure
	// that they are valid.
	for index, doc := range contentParts {
		// NOTE: We MUST make a copy of the instance, because the `instances` serve as blueprint.
		// So we must be careful here not to accidentally change state of the `instances` array.
		instance := instances[index]
		err = options.Unmarshal(doc, &instance, options)
		if err != nil {
			err = eris.Wrapf(err, "render error in %q", templateName)
			return out, err
		}
		out = append(out, instance)
	}

	return out, nil
}

// Adds a way for users to access helm variables via go templates `{{ }}` without
// having those commands lost when we "pre-render" templates.
//
// To achieve that, user has to use `{{! ... }}` instead of plain `{{ ... }}`.
//
// Behind the scences, we replace the `{{! }}` with identifiers that we can then
// match back after the template has been matched.
func escapeHelmTemplateActions(tmpl string) (string, map[string]string) {
	replacementMap := map[string]string{}

	re := regexp.MustCompile(`{{![^}]*}}`)
	tmpl = re.ReplaceAllStringFunc(tmpl, func(match string) string {
		// E.g. `__helpa__slot_1`
		key := fmt.Sprintf("__helpa__slot_%v", len(replacementMap))
		match = strings.Replace(match, "{{!", "{{", 1)
		replacementMap[key] = match
		return key
	})

	return tmpl, replacementMap
}

func unescapeHelmTemplateActions(tmpl string, replMap map[string]string) string {
	re := regexp.MustCompile(`__helpa__slot_\d+`)
	tmpl = re.ReplaceAllStringFunc(tmpl, func(match string) string {
		return replMap[match]
	})
	return tmpl
}

func doPrepareComponentInput[TInput any](
	templateName string,
	templateStr string,
	templateIsFile bool,
	options *Options[TInput],
) (outTemplateStr string, replacementMap map[string]string, err error) {
	outTemplateStr = templateStr

	// Set defaults
	if options.PreprocessTemplate == nil {
		options.PreprocessTemplate = defaultPreprocessor
	}
	if options.Unmarshal == nil {
		options.Unmarshal = defaultUnmarshaller
	}
	if options.MultiDocSeparator == "" {
		options.MultiDocSeparator = "---"
	}

	// Load the template from file
	if templateIsFile {
		dat, err := os.ReadFile(outTemplateStr)
		if err != nil {
			err = eris.Wrapf(err, "error reading file in %q", templateName)
			return outTemplateStr, replacementMap, err
		}
		outTemplateStr = string(dat)
	}

	// Normalize the template
	outTemplateStr, err = options.PreprocessTemplate(outTemplateStr, *options)
	if err != nil {
		return outTemplateStr, replacementMap, eris.Wrapf(err, "failed to preprocess template in %q", templateName)
	}

	// Add a way for users to access helm variables via go templates `{{ }}` without
	// having those commands lost when we "pre-render" templates.
	outTemplateStr, replacementMap = escapeHelmTemplateActions(outTemplateStr)

	return outTemplateStr, replacementMap, nil
}

func CreateComponent[
	TType any,
	TInput any,
	TContext any,
](comp Def[TType, TInput, TContext]) (Component[TType, TInput], error) {
	comp = comp.Copy()

	if comp.Setup == nil {
		comp.Setup = func(t TInput) (context TContext, err error) { return context, err }
	}

	tmpl, replMap, err := doPrepareComponentInput(comp.Name, comp.Template, comp.TemplateIsFile, &comp.Options)
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

			content, err = Render[TContext](comp.Name, comp.Template, context)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instance, content, err
				}
			}

			// Put back the bits that we've removed previously so that they get rendered by Helm
			content = unescapeHelmTemplateActions(content, replMap)

			if comp.Render != nil {
				instance, err = comp.Render(input, context, content)
			} else {
				// Unmarshal the generated structured data to ensure that they are valid.
				instance, err = doUnmarshalOne[TType](comp.Name, content, comp.Options)
			}
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

	// If frontloading is enabled, we will make a dummy call to the `component.Render`
	// method at component creation, to ensure that everything works correctly,
	// especially the unmarshalling of a textual template.
	if comp.Options.FrontloadEnabled {
		_, _, err = component.Render(comp.Options.FrontloadInput)
	}
	if err != nil {
		if comp.Options.PanicOnError {
			panic(err)
		} else {
			return component, err
		}
	}

	return component, nil
}

func CreateComponentMulti[
	TType any,
	TInput any,
	TContext any,
](comp DefMulti[TType, TInput, TContext]) (ComponentMulti[TType, TInput], error) {
	comp = comp.Copy()

	if comp.Setup == nil {
		comp.Setup = func(t TInput) (context TContext, err error) { return context, err }
	}

	tmpl, replMap, err := doPrepareComponentInput(comp.Name, comp.Template, comp.TemplateIsFile, &comp.Options)
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

			content, err := Render(comp.Name, comp.Template, context)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instances, contentParts, err
				}
			}

			// Put back the bits that we've removed previously so that they get rendered by Helm
			content = unescapeHelmTemplateActions(content, replMap)

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
			instances, err = comp.GetInstances(input, context)
			if err != nil {
				if comp.Options.PanicOnError {
					panic(err)
				} else {
					return instances, contentParts, err
				}
			}

			if len(instances) != len(contentParts) {
				err = eris.Wrapf(ErrComponentRenderResultMismatch, "found %v documents in the template, but there is %v instances to unmarshal the data to. These must match. Review the component's `GetInstances` method and the template", len(contentParts), len(instances))
				return instances, contentParts, err
			}

			if comp.Render != nil {
				instances, err = comp.Render(input, context, contentParts)
			} else {
				// Unmarshal the generated structured data to ensure that they are valid.
				instances, err = doUnmarshalMulti(comp.Name, contentParts, comp.Options, instances)
			}
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

	// If frontloading is enabled, we will make a dummy call to the `component.Render`
	// method at component creation, to ensure that everything works correctly,
	// especially the unmarshalling of a textual template.
	if comp.Options.FrontloadEnabled {
		_, _, err = component.Render(comp.Options.FrontloadInput)
	}
	if err != nil {
		if comp.Options.PanicOnError {
			panic(err)
		} else {
			return component, err
		}
	}

	return component, nil
}
