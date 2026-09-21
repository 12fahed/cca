---
title: Configuration
description: Optional settings, where they live, and which layer wins.
sidebar:
  order: 5
---

`cca` works with no configuration at all. A config file only exists to change a
default you keep overriding by hand.

## Where the file goes

| Platform | Path |
| --- | --- |
| Linux, macOS | `~/.config/cca/config.json` |
| Linux, macOS (XDG set) | `$XDG_CONFIG_HOME/cca/config.json` |
| Windows | `%APPDATA%\cca\config.json` |

`cca config` prints the exact path it is looking at, whether or not the file
exists.

## Every setting

```json
{
  "water_ml_per_1k_tokens": 0.30,
  "claude_dir": "",
  "pricing": "",
  "no_sidechains": false,
  "ascii": false,
  "no_color": false
}
```

Every key is optional. Omit what you do not want to change.

| Key | Type | Default | Effect |
| --- | --- | --- | --- |
| `water_ml_per_1k_tokens` | number | `0.30` | The water constant. A **placeholder**, not a measurement |
| `claude_dir` | string | `~/.claude` | Where transcripts live |
| `pricing` | string | embedded table | Path to a replacement rate table |
| `no_sidechains` | bool | `false` | Exclude sub-agent usage |
| `ascii` | bool | `false` | Plain ASCII instead of `·`, `≈`, `─` |
| `no_color` | bool | `false` | Never style output |

:::note[JSON has no comments]
The file supports a `_comment` key, which `cca` ignores, so you can document
your own choices in place — particularly useful for explaining why you changed
the water constant.
:::

## Precedence

**Flag beats config file beats default.** Always.

A boolean set to `false` in the file is a real setting, not an absent one, and
a flag passed explicitly beats it either way.

## Seeing what won

```sh
cca config
```

```
  Claude Code analyzer · resolved configuration

  Setting                 value                from
  claude_dir              /home/fahed/.claude  default
  water_ml_per_1k_tokens  0.30 mL / 1k tokens  default
  no_sidechains           false                default
  ascii                   false                default
  no_color                false                default

  Files
  File     path                                                                                                                       status
  config   /tmp/claude-1001/-home-fahed-Fahed-Personal-cca/51f4fc3e-1393-4a0a-8076-b8ada1d8bc9e/scratchpad/emptycfg/cca/config.json   not present
  pricing  /tmp/claude-1001/-home-fahed-Fahed-Personal-cca/51f4fc3e-1393-4a0a-8076-b8ada1d8bc9e/scratchpad/emptycfg/cca/pricing.json  embedded default

  ─ The water rate is a rough placeholder, not a measured value.
    Set water_ml_per_1k_tokens in the config file to substitute your own; see README.
  ─ No config file yet. Create one at the path above to change these defaults;
    every key is optional and any flag overrides it.
```

The `from` column is the point of this view. When output surprises you, this
tells you which layer is responsible.

## Overriding rates

Drop a `pricing.json` next to `config.json`, or pass `--pricing PATH`. It
**replaces** the embedded table entirely rather than merging into it, so a
price change is a whole-file edit and never a partial one.

See [Rate table](/cca/reference/pricing/).

## Changing the water constant

```sh
cca --water-ml-per-1k 0.12
```

or, permanently:

```json
{ "water_ml_per_1k_tokens": 0.12 }
```

The printed assumption updates to match, so the output never claims a figure it
did not use. See [The water estimate](/cca/internals/water/).
