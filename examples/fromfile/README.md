# Component from file example

This example is the same as [the Basic example](./../basic/README.md).
But in this one, we move the template out into it's own file.

Notice that when we use template defined in a seprate file, we must set
`TemplateIsFile` to `true`.

Moreover, the path specified in `Template` must be relative to the
working directory (the directory where you execute the Go binary).
We asume that the binary is executed in the same directory as where
we define `go.mod`.
