# Helm template example

This example shows how to use Helpa to generate Helm templates.

- `helm.yaml` is the YAML template - it's the backbone of what we want to define.

- `helm.go` is the Go counterpart - the component. The component defines how to
  prepare inputs for the YAML template, AND defines data structs that will be used
  to unmarshal (deserialize) the data from YAML.

  > NOTE: The data structs serve two functions. They give us a way to interact with
  > the underlying YAML data. But this process also ensures that templates are valid.

- `templates/deployment.yaml` is the generated YAML we obtained by running
  `Component.Render(Input{ ... })`, and then writing the results to new file.
