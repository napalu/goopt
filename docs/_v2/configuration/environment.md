---
layout: default
title: Environment Variables
parent: Configuration
nav_order: 1
---

# Environment Variables

By default, goopt will not consider environment variables when parsing. In order to enable this, you need to set the `SetEnvNameConverter` function which will convert the environment variable name to the flag name.
Unless explicitly specified, goopt assumes that flag names are in local camel case format. Setting the `SetEnvNameConverter` will do the following:
- Convert an environment variable name to using the output of the `SetEnvNameConverter` function 
- Checks the configuration for the flag name and if it exists, use the value from the environment variable.

Variables are evaluated in the following order (from highest precedence to lowest):
1. command line flags
2. flag defaults from external sources (such as JSON, YAML, etc.) set via the map supplied to the `ParseWithDefaults` function
3. **environment variables**
4. defaults set via the `WithDefaultValue` function or via struct tag annotations

## Usage

```go
package main

import "github.com/napalu/goopt/v2"

func main() {
	parser := goopt.NewParser()
	parser.SetEnvNameConverter(func(s string) string {
		// DefaultFlagNameConverter is the default implementation and converts ENV var names to lowerCamelCase
		return goopt.DefaultFlagNameConverter(s) 
	})
	
}
```

