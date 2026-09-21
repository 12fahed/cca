---
title: Installation
description: Build cca from source, or download a prebuilt binary.
sidebar:
  order: 2
---

`cca` ships as a single static binary with no runtime dependencies. There is
nothing to install alongside it and nothing to configure before first use.

## Build from source

This is the primary path, and the one to use if you have Go available.

**Requires Go 1.27 or newer.**

```sh
git clone https://github.com/12fahed/cca.git
cd cca
make build
```

That produces `./cca` in the repository root. Put it somewhere on your `PATH`:

```sh
# macOS and Linux
sudo install -m 0755 cca /usr/local/bin/cca
```

On Windows, copy `cca.exe` to a directory on your `PATH`.

Verify it:

```sh
cca version
```

```
cca       v0.1.0
commit    1964cd1
built     2026-09-22T09:14:03Z
go        go1.27.1
platform  linux/amd64
license   GPL-3.0, no warranty — https://github.com/12fahed/cca/blob/main/LICENSE
```

## Download a binary

Prebuilt archives are attached to each
[release](https://github.com/12fahed/cca/releases), with a `checksums.txt`
alongside them.

| Platform | Architecture |
| --- | --- |
| macOS | `arm64`, `amd64` |
| Linux | `amd64`, `arm64` |
| Windows | `amd64` |

Download, verify, extract, install:

```sh
curl -LO https://github.com/12fahed/cca/releases/latest/download/cca_0.1.0_linux_amd64.tar.gz
curl -LO https://github.com/12fahed/cca/releases/latest/download/checksums.txt
sha256sum --check --ignore-missing checksums.txt
tar -xzf cca_0.1.0_linux_amd64.tar.gz
sudo install -m 0755 cca /usr/local/bin/cca
```

## Build every platform at once

```sh
make build-all
```

Writes all five targets to `dist/`, each built with `CGO_ENABLED=0`.

## Why not `go install`?

:::note
The module path is `cca` rather than a domain-qualified path such as
`github.com/12fahed/cca`, so `go install cca/cmd/cca@latest` cannot be resolved
by the Go toolchain. Clone and build instead.
:::

## Uninstalling

`cca` writes nothing outside its own configuration directory, so removing it is
two steps:

```sh
sudo rm /usr/local/bin/cca
rm -rf ~/.config/cca        # only if you created a config file
```

Your Claude Code transcripts are untouched by both.
