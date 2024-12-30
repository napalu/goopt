---
layout: default
title: Configuration
nav_order: 3
has_children: true
---

# Configuration

goopt supports multiple configuration sources with clear precedence:

1. Command-line flags (highest priority)
2. External configuration (via ParseWithDefaults)
4. Environment variables
3. Default values (lowest priority) 