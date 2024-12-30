---
layout: default
title: External Configuration
parent: Configuration
nav_order: 2
---

# External Configuration

Use `ParseWithDefaults` to load configuration from external sources.

Variables are evaluated in the following order (from highest precedence to lowest):
1. command line flags
2. **flag defaults from external sources (such as JSON, YAML, etc.) set via the map supplied to the `ParseWithDefaults` function**
3. environment variables
4. defaults set via the `SetDefaultValue` function or the `WithDefaultValue` function or via struct tag annotations


## Example

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
    "github.com/napalu/goopt"
)

func getDefaults() (map[string]string, error) {
	extension := filepath.Ext(os.Args[0])
	configFile := strings.TrimRight(os.Args[0], extension) + ".config.json"

	if _, err := os.Stat(configFile); err == nil {
		jsonFile, err := os.Open(configFile)
		if err != nil {
			return nil, err
		}
		defer jsonFile.Close()

		jsonBytes, _ := io.ReadAll(jsonFile)
		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		if err != nil {
			return nil, err
		}

		defaults := make(map[string]string, len(result))
		for k, v := range result {
			defaults[k] = fmt.Sprint(v)
		}

		return defaults, nil
	}

	return nil, fmt.Errorf("no config file found")
}

func main() {
    parser := goopt.NewParser()

    var success bool
    if defaults, err := getDefaults(); err == nil {
        // whatever command line flags are not set will be set to the defaults if present
        success = parser.ParseWithDefaults(os.Args, defaults)
    } else {
        success = parser.Parse(os.Args)
    }

    if !success {
        fmt.Println("Failed to parse flags")
    }
}
```

