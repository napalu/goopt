# Repeated Flags Example

This example demonstrates the repeated flag pattern support in goopt v2.

## Overview

goopt v2 supports repeated flags (called `chained`values) in two ways which can be mixed if desired. 
As is commonly used in many CLI tools, a flag can be specified multiple times to accumulate values. The "goopt" way would be to 
provide the list using comma or pipe-delimited values in quotes which is more compact.

## Usage Patterns

### Traditional Comma-Separated Approach
```bash
./repeated-flags --include "src,test,docs" --tag "dev,prod"
```

### Repeated Flag Approach
```bash
./repeated-flags -i src -i test -i docs -t dev -t prod
```

### Mixed Approach
You can even mix both patterns:
```bash
./repeated-flags --include src,test --include docs --tag dev --tag prod,staging
```

## How It Works

The key is to use the `Chained` type when defining your flags:

```go
package main

import (
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/types"
)

func main() {
	var includes []string
	p := goopt.NewParser()
	err := p.BindFlag(&includes, "include", goopt.NewArg(
		goopt.WithType(types.Chained),
		goopt.WithShortFlag("i"),
		goopt.WithDescription("Include paths (can be repeated)"),
	))
}
```

or when using struct-tags you can just specify the variable as being a slice:

```go
package main

import (
	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/types"
)

type Test struct {
    Includes string `goopt:"short:i;desc:Include paths (can be repeated)"`
}

func main() {
	test := &Test{}
	p, _ := goopt.NewParserFromStrut(test)
}
```

When a `Chained` type flag is encountered multiple times during parsing, goopt will:
1. Parse the value(s) from each occurrence
2. Append them to the bound slice variable
3. Maintain all values in order

## Benefits

- **Familiar Pattern**: Many popular CLI tools use this repeated pattern (docker, kubectl, etc.)
- **Flexible**: Users can choose between comma-separated or repeated flags
- **Shell-Friendly**: The repeated pattern is easier to use with shell expansion and scripting
- **Compact**: The separated-list pattern is more compact

## Example Output

```bash
$ ./repeated-flags -i src -i test -i docs -t dev -t prod -v
Repeated Flags Example
========================================
Verbose mode: ON

Included paths (3):
  1. src
  2. test
  3. docs

Tags (2):
  1. dev
  2. prod
```
