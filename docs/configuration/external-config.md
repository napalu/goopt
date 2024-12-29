---
layout: default
title: External Configuration
parent: Configuration
nav_order: 2
---

# External Configuration

Use `ParseWithDefaults` to load configuration from external sources.

## Example

```go
defaults := Options{
    Output: "/tmp/default.txt",
}
parser.ParseWithDefaults(os.Args, defaults)
```

[Rest of external configuration documentation] 