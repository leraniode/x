# X

[![part of leraniode](https://raw.githubusercontent.com/leraniode/.github/main/assets/badges/partofleraniode.svg)](https://github.com/leraniode)
[![license](https://img.shields.io/badge/license-MIT-green)](./LICENSE)
[![CI](https://github.com/leraniode/x/actions/workflows/ci.yml/badge.svg)](https://github.com/leraniode/x/actions/workflows/ci.yml)

> Low coded and experimental packages for the Leraniode ecosystem 🪛.

Packages here are pre-stable. APIs may break between commits.

---

## Packages

- [`centrix`](./centrix) - Sparse signal mathematics library for Go.
- [`testutil`](./testutil) - Test utilities for leraniode.
- [`wtone`](./wtone) - Wondertone's file format tool.

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

Experimental packages are maintained by [DominionDev](https://github.com/dominionthedev).
If you feel the zeal to hop into leraniode's experimental side, You can open Issues and PRs

---

<p align="left">
Experimental Leraniode · Part of Leraniode
</p>

<img
  align="left"
  src="https://raw.githubusercontent.com/leraniode/.github/main/assets/footer.png"
  alt="Leraniode"
  width="400"
  style="border-radius: 15px;"
/>
