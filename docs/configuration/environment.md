---
layout: default
title: Environment Variables
parent: Configuration
nav_order: 1
---

# Environment Variables

goopt can read configuration from environment variables using `SetEnvNameConverter`.

## Usage

```go
parser.SetEnvNameConverter(func(s string) string {
    return "APP_" + strings.ToUpper(s)
})
```

[Rest of environment variables documentation] 