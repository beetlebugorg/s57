---
sidebar_position: 2
---

# Installation

## Requirements

- Go 1.21 or later
- Access to S-57 ENC files (.000, .001, etc.)

## Install the Package

```bash
go get github.com/beetlebugorg/s57
```

## Verify Installation

Create a simple test program:

```go
package main

import (
    "fmt"
    "github.com/beetlebugorg/s57/pkg/s57"
)

func main() {
    parser := s57.NewParser()
    fmt.Printf("S-57 Parser version: %T\n", parser)
}
```

Run it:

```bash
go run main.go
```

You should see:
```
S-57 Parser version: *s57.Parser
```

## Dependencies

The parser automatically installs these dependencies:

- `github.com/beetlebugorg/iso8211/pkg/iso8211` - ISO 8211 file format parser
- `github.com/dhconnelly/rtreego` - R-tree for spatial indexing

## Getting S-57 Test Data

For development and testing, you can obtain S-57 ENC files from:

- **NOAA** - US charts available at [NOAA ENC Direct](https://www.charts.noaa.gov/ENCs/ENCs.shtml)
- **UKHO** - UK Admiralty charts (commercial)
- **Primar** - International chart distributor (commercial)

Many regions provide free ENC data for non-commercial use.

## Next Steps

- [Quick Start Guide](examples.md#quick-start) - Your first S-57 parsing program
- [API Reference](api-reference.md) - Complete API documentation
