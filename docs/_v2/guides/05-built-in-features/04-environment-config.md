---
layout: default
title: Environment & External Configuration
parent: Built-in Features
nav_order: 4
---

# Environment & External Configuration

Beyond command-line flags, `goopt` supports loading configuration from two additional sources: environment variables and external configuration maps. This allows for flexible and powerful configuration management.

## Configuration Precedence

`goopt` resolves values in a clear, fixed order. Sources with a higher number override those with a lower number.

1.  **Default Values:** Set via `default:"..."` or `WithDefaultValue()`. (Lowest priority)
2.  **Environment Variables:** Loaded if a name converter is set.
3.  **External Configuration:** Loaded from the map passed to `ParseWithDefaults()`.
4.  **Command-Line Flags:** Provided directly by the user. (Highest priority)

---

## Environment Variables

By default, `goopt` does not read from the environment. To enable this, you must provide a `NameConversionFunc` using `SetEnvNameConverter`. This function tells `goopt` how to map an environment variable name (like `MYAPP_SERVER_PORT`) to a flag name (like `server.port`).

```go
import "github.com/iancoleman/strcase"

func main() {
    parser := goopt.NewParser()

    // Example: Convert "MYAPP_SERVER_PORT" to "serverPort"
    parser.SetEnvNameConverter(strcase.ToLowerCamel)

    // Example: Convert "MYAPP_SERVER_PORT" to "server-port" (kebab-case)
    parser.SetEnvNameConverter(goopt.ToKebabCase)
    
    // Now, when parser.Parse() is called, goopt will check for environment
    // variables that match your flag names after conversion.
}
```

With a converter set, an environment variable like `MYAPP_HOST=db.example.com` would automatically provide the value for a `--host` flag if it wasn't set on the command line.

---

## External Configuration (`ParseWithDefaults`)

The `ParseWithDefaults` function allows you to load configuration from any source—such as a JSON file, a YAML file, or a remote configuration service—by passing in a `map[string]string`.

This map acts as a source of default values that have a higher precedence than built-in defaults and environment variables, but a lower precedence than explicit command-line flags.

### Example: Loading from a JSON File

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
    "github.com/napalu/goopt/v2"
)

// loadConfigFromFile reads a JSON file and returns it as a map.
func loadConfigFromFile(path string) (map[string]string, error) {
    bytes, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    
    var result map[string]interface{}
    if err := json.Unmarshal(bytes, &result); err != nil {
        return nil, err
    }

    // Convert map[string]interface{} to map[string]string
    configMap := make(map[string]string)
    for key, value := range result {
        configMap[key] = fmt.Sprint(value)
    }
    return configMap, nil
}

func main() {
    parser := goopt.NewParser()
    // ... define flags ...

    // Try to load config from "config.json"
    defaults, err := loadConfigFromFile("config.json")
    if err != nil {
        // No config file, just parse regular args
        parser.Parse(os.Args)
    } else {
        // Config file found, parse with it as a source of defaults
        parser.ParseWithDefaults(defaults, os.Args)
    }
    
    // ... handle parsing results ...
}
```
In this example, if `config.json` contains `{"host": "prod.db.server"}`, the value for the `--host` flag will be `prod.db.server` unless the user provides a different value on the command line (e.g., `./myapp --host staging.db.server`).