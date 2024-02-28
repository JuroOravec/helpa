# Helm chart example

This example shows a full-fledged Helm chart project for a small cluster.

The cluster defines an ingress (`ingress`), a demo web app (`kuard`), and
configures TLS with batch jobs that request certificates using Certbot (`certbot`).

- `src` - This directory includes the component files.

- `templates` - This is the OUTPUT for the Helpa components, serialized as YAML.
  As such the generated files may be used as part of Helm chart right away.

## Component inputs - Emulating `values.yaml` with `src.go`

### Centralizing all inputs

Navigating to e.g. [kuard.go](./src/kuard/kuard.go) or [ingress.go](./src/ingress/ingress.go),
you will notice that each component defines its own inputs.

You may want to centralize all component inputs in a single place, to double
as a config file (as Helm's `values.yaml`).

This is what we achive in `src.go` by defining a single struct that joins all
inputs together.

```go
type ChartInput struct {
	CertbotInput   certbot.Input
	CertbotEnabled bool
	KuardInput     kuard.Input
	IngressInput   ingress.Input
}
```

### Defining defaults

In this example, we decided to define the defaults on the level of the chart.
In `src.go`, you will see the `ChartDefaults` function, which does that.

To APPLY the defaults, use the `ApplyDefaults` utility:

```go
import (
  helpaUtils "github.com/jurooravec/helpa/pkg/utils"
)

func RenderTemplates(input ChartInput) (err error) {
	inputCopy := input
	err = helpaUtils.ApplyDefaults(&inputCopy, ChartDefaults())
	if err != nil {
		return err
	}
  // ...
}
```

## Serializing the rendered components into `templates` dir

Currently, Helpa doesn't make assumptions about how to write to the `templates`
directory. See `src.go` for an example of how to do it.

Overall, it involves 3 steps:

1. Render the components, and extract data structs.
2. Decide which data structs should be rendered to what files.
3. Write the data to the files using `HelmChartSerializer`.

```go
import (
  serializers "github.com/jurooravec/helpa/pkg/serializers"
)

func RenderTemplates(input ChartInput, outdir string) (err error) {
  // ...

  // 1. Render components
	ingressSpecs, _, err := ingress.Component.Render(inputCopy.IngressInput)
	if err != nil {
		return err
	}

  // 2. Arrange which resources should be written to which file
	outfiles := map[string][]runtime.Object{
		"certbot": certbotSpecs,
		"kuard":   kuardSpecs,
		"ingress": ingressSpecs,
	}

  // 3. Use HelmChartSerializer to write Helm templates.
	err = serializers.HelmChartSerializer(outfiles, outdir)
	if err != nil {
		return err
	}
}
```
