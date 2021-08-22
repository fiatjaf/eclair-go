eclair-go
=========

An API wrapper for [Eclair](https://acinq.github.io/eclair/) that returns [gjson](https://github.com/tidwall/gjson) results.

Read the [documentation](https://pkg.go.dev/github.com/fiatjaf/eclair-go).

Quick Start
-----------

```go
package main

import (
  "log"
  "github.com/fiatjaf/eclair-go"
)

func main() {
  ln := eclair.Client{Host: "http://localhost:8080", Password: "satoshi21"}
  res, _ := ln.Call("getinfo", nil)
  log.Print(res.Get("nodeId").String())
}
```
