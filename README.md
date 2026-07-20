# X

[![part of leraniode](https://raw.githubusercontent.com/leraniode/.github/main/assets/badges/partofleraniode.svg)](https://github.com/leraniode)
[![license](https://img.shields.io/badge/license-MIT-green)](./LICENSE)
[![CI](https://github.com/leraniode/x/actions/workflows/ci.yml/badge.svg)](https://github.com/leraniode/x/actions/workflows/ci.yml)

> Experimental packages for the Leraniode ecosystem.

Packages here are pre-stable. APIs may break between commits.

---

## Packages

### [`centrix`](./centrix)

[![Go](https://img.shields.io/badge/go-1.22-00ADD8?logo=go)](https://go.dev)
[![Tests](https://img.shields.io/badge/tests-167%20passing-brightgreen)]()

Sparse signal mathematics library. Defines the types, algebra, and field dynamics
for deterministic reasoning and generation systems.

```go
import "github.com/leraniode/x/centrix/core"
import "github.com/leraniode/x/centrix/field"
import "github.com/leraniode/x/centrix/registry"
```

### [`wtone`](./wtone/)

[![Go](https://img.shields.io/badge/go-1.22-00ADD8?logo=go)](https://go.dev)

Wondertone file format and Utility package.

```go
import "github.com/leraniode/x/wtone"
import "github.com/leraniode/x/wtone/testutil"
```

---

## Structure

### Go packages

Each package is an independent Go module with its own `go.mod`. To work across
packages simultaneously, use a local `go.work` file:

```bash
go work init
go work use ./centrix
go work use ./wtone
```

`go.work` is gitignored — it is a local development tool, not part of the repository.

---

## Contributing

Experimental packages are maintained by Leraniode. Ideas, feedback, and
discussion are welcome.

- 💬 [Discussions](https://github.com/leraniode/x/discussions)

---

<p align="left">
Experimental Leraniode · Part of Leraniode
</p>

<img
  align="left"
  src="https://raw.githubusercontent.com/leraniode/.github/main/assets/footer/leraniodeproductbrandimage.png"
  alt="Leraniode"
  width="400"
  style="border-radius: 15px;"
/>
