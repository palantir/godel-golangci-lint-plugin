<p align="right">
<a href="https://autorelease.general.dmz.palantir.tech/palantir/godel-golangci-lint-plugin"><img src="https://img.shields.io/badge/Perform%20an-Autorelease-success.svg" alt="Autorelease"></a>
</p>

# godel-golangci-lint-plugin
`godel-golangci-lint-plugin` is a godel plugin that adds tasks that allow linting godel projects using [`golangci-lint`](https://github.com/golangci/golangci-lint).
It is a successor to the [`okgo`](https://github.com/palantir/okgo) godel plugin, and is meant to replace it in most use
cases.

## Features
The `godel-golangci-lint-plugin` adds the following tasks to `godel`:

* `lint`: runs `golangci-lint` on the project (equivalent of `golangci-lint run`)
    * `lint [linters]`: runs only the specified linters on the project
* `linters`: prints the configured linters
    * `linters --config`: prints the full `golangci-lint` configuration used by the plugin

The `lint` task is also added to the godel `verify` task, and if verify is run with `--apply=true`, then `lint` is run
in a mode that applies its fixes (if supported by the linter).

## Configuration
The `golangci-lint-plugin` is configured using the `godel/config/golangci-lint-plugin.yml` file. The configuration file
is a YAML file that is defined as follows:

```
type PluginConfig struct {
	Linters LintersConfig `yaml:"linters,omitempty"`
}

type LintersConfig struct {
	Enable     []string         `yaml:"enable,omitempty"`
	Disable    []string         `yaml:"disable,omitempty"`
	Settings   yaml.MapSlice    `yaml:"settings,omitempty"`
	Exclusions ExclusionsConfig `yaml:"exclusions,omitempty"`
}

type ExclusionsConfig struct {
	Rules       []RulesConfig `yaml:"rules,omitempty"`
	Paths       []string      `yaml:"paths,omitempty"`
	PathsExcept []string      `yaml:"paths-except,omitempty"`
}

type RulesConfig struct {
	Linters    []string `yaml:"linters,omitempty"`
	Path       string   `yaml:"path,omitempty"`
	PathExcept string   `yaml:"path-except,omitempty"`
	Text       string   `yaml:"text,omitempty"`
	Source     string   `yaml:"source,omitempty"`
}
```

The following is an example of a specific configuration:

```yaml
linters:
  enable:
    - staticcheck
  settings:
    revive:
      rules:
        - name: package-comments
          disabled: true
```

The configuration that is exposed in this file is a strict subset of the configuration that is supported by `golangci-lint`,
as defined at https://golangci-lint.run/docs/configuration/file/. This is intentional: one of the goals of the
`golangci-lint-plugin` is to provide consistency and standardization across projects, and being opinionated about the
configuration that can be specified and only allowing specific aspects to be configured via the plugin configuration
helps achieve this goal.

The configuration specified in this file is merged with base configuration (if specified as an asset). Merging is done
in the following manner:

* Elements in the `enable` and `disable` lists are appended to the corresponding lists in the base configuration
* If `settings` is specified, the value that corresponds to each key in the `settings` map is set as the value for the
  corresponding key in the base configuration (adding the key if it does not already exist)
* If `exclusions` is specified, any elements in the `rules`, `paths`, and `paths-except` lists are appended to the
  corresponding lists in the base configuration

## Design
`golangci-lint-plugin` provides `godel` tasks, reads the plugin configuration from the
`godel/config/golangci-lint-plugin.yml` file, and invokes `golangci-lint` with the appropriate flags, arguments, and
configuration.

The `golangci-lint` executable that is invoked is specified to the plugin as an asset in the form of a TGZ that contains
a single file that is the `golangci-lint` executable (the asset resolver should allow resolving the correct asset for a
given OS/architecture). The plugin requires that exactly 1 `golangci-lint` asset is specified. Allowing the
`golangci-lint` executable to be specified as an asset allow for flexibility by allowing the user to specify the exact
executable that is run. This allows for things like using a `golangci-lint` executable with a specific version or using
a custom build of `golangci-lint` that includes custom linters.

`golangci-lint-plugin` also supports specifying a base configuration that should be used when invoking `golangci-lint`.
The base configuration is specified as an optional asset. If specified, there can only be 1 configuration asset. The
configuration asset is a TGZ that contains a single YAML file that is the base `golangci-lint` configuration.

When `golangci-lint-plugin` invokes `golangci-lint`, it reads the base configuration from the asset (if specified),
adds any "exclude" configuration specified in `godel/config/godel.yml` as exclusions, then merges it with the
user-specified configuration in `godel/config/golangci-lint-plugin.yml` (by applying this configuration on top of the
default configuration in a specific manner), writes the merged configuration to a temporary file, and then invokes
`golangci-lint` with a flag values that instructs it to use this configuration file.

## Debugging issues
The most straightforward way to debug linting issues is to run the `lint` command with the `--debug` flag:
`./godelw lint --debug`. This will do the following:

* Prints the command used to invoke the `golangci-lint` executable (asset), including the path to the executable and the
  flags and arguments passed to it (including to the configuration file)
* Preserves the configuration file that is generated by the plugin
* Runs the `golangci-lint` plugin in verbose mode (`-v`)

With this information, the `golangci-lint` asset can be run directly and provided with any extra flags or arguments
necessary for further debugging (including things like persisting CPU and memory profiles). If the user has the source
code for the asset locally, they can also run the asset with a debugger using the provided configuration and arguments
to further debug issues.
