package config

import (
	"encoding/json"
	"fmt"
	"os"

	"cca/internal/water"
)

// File mirrors config.json. Every field is a pointer so that an absent setting
// is distinguishable from one deliberately set to a zero value: without that,
// "no_sidechains": false would be indistinguishable from not mentioning it, and
// could not override a default.
type File struct {
	WaterMLPer1k *float64 `json:"water_ml_per_1k_tokens"`
	ClaudeDir    *string  `json:"claude_dir"`
	Pricing      *string  `json:"pricing"`
	NoSidechains *bool    `json:"no_sidechains"`
	ASCII        *bool    `json:"ascii"`
	NoColor      *bool    `json:"no_color"`
}

// Overrides carries the flags the user actually passed. A nil field means the
// flag was absent, which is what lets the config file win over a default while
// still losing to an explicit flag.
type Overrides struct {
	WaterMLPer1k *float64
	ClaudeDir    *string
	Pricing      *string
	NoSidechains *bool
	ASCII        *bool
	NoColor      *bool
}

// Source names where a resolved value came from, for `cca config`.
type Source string

const (
	FromDefault Source = "default"
	FromFile    Source = "config file"
	FromFlag    Source = "flag"
)

// Resolved is the effective configuration, plus a record of where each value
// came from so the user can see why cca is behaving as it is.
type Resolved struct {
	WaterMLPer1k float64
	ClaudeDir    string
	PricingPath  string
	NoSidechains bool
	ASCII        bool
	NoColor      bool

	// ConfigPath is where cca looked for config.json, whether or not it existed.
	ConfigPath  string
	ConfigFound bool

	Sources map[string]Source
}

// Setting names used as keys in Resolved.Sources.
const (
	KeyWater        = "water_ml_per_1k_tokens"
	KeyClaudeDir    = "claude_dir"
	KeyPricing      = "pricing"
	KeyNoSidechains = "no_sidechains"
	KeyASCII        = "ascii"
	KeyNoColor      = "no_color"
)

// LoadFile reads config.json from path. A missing file is not an error: running
// without one is the normal case.
func LoadFile(path string) (File, bool, error) {
	if !exists(path) {
		return File{}, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, false, fmt.Errorf("read config: %w", err)
	}
	var f File
	// Unknown keys are ignored so that a _comment field can document the
	// settings for whoever edits the file, as pricing.json does.
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := f.validate(path); err != nil {
		return File{}, false, err
	}
	return f, true, nil
}

func (f File) validate(path string) error {
	if f.WaterMLPer1k != nil && *f.WaterMLPer1k < 0 {
		return fmt.Errorf("%s: %s must not be negative", path, KeyWater)
	}
	return nil
}

// Resolve applies flag over file over default and records the winner for each
// setting.
func Resolve(flags Overrides, file File, fileFound bool, configPath string) (Resolved, error) {
	claudeDir, err := DefaultClaudeDir()
	if err != nil {
		return Resolved{}, fmt.Errorf("locate the Claude directory: %w", err)
	}
	pricingPath, err := PricingPath()
	if err != nil {
		return Resolved{}, err
	}
	// A discovered override only counts when it is actually there; otherwise
	// the embedded table is used and the path is reported as unset.
	if !exists(pricingPath) {
		pricingPath = ""
	}

	r := Resolved{
		WaterMLPer1k: water.DefaultMLPer1kTokens,
		ClaudeDir:    claudeDir,
		PricingPath:  pricingPath,
		ConfigPath:   configPath,
		ConfigFound:  fileFound,
		Sources: map[string]Source{
			KeyWater: FromDefault, KeyClaudeDir: FromDefault, KeyPricing: FromDefault,
			KeyNoSidechains: FromDefault, KeyASCII: FromDefault, KeyNoColor: FromDefault,
		},
	}

	resolveFloat(&r, KeyWater, &r.WaterMLPer1k, file.WaterMLPer1k, flags.WaterMLPer1k)
	resolveString(&r, KeyClaudeDir, &r.ClaudeDir, file.ClaudeDir, flags.ClaudeDir)
	resolveString(&r, KeyPricing, &r.PricingPath, file.Pricing, flags.Pricing)
	resolveBool(&r, KeyNoSidechains, &r.NoSidechains, file.NoSidechains, flags.NoSidechains)
	resolveBool(&r, KeyASCII, &r.ASCII, file.ASCII, flags.ASCII)
	resolveBool(&r, KeyNoColor, &r.NoColor, file.NoColor, flags.NoColor)

	if r.WaterMLPer1k < 0 {
		return Resolved{}, fmt.Errorf("%s must not be negative, got %v", KeyWater, r.WaterMLPer1k)
	}
	return r, nil
}

func resolveFloat(r *Resolved, key string, dst *float64, fromFile, fromFlag *float64) {
	if fromFile != nil {
		*dst, r.Sources[key] = *fromFile, FromFile
	}
	if fromFlag != nil {
		*dst, r.Sources[key] = *fromFlag, FromFlag
	}
}

func resolveString(r *Resolved, key string, dst *string, fromFile, fromFlag *string) {
	if fromFile != nil && *fromFile != "" {
		*dst, r.Sources[key] = *fromFile, FromFile
	}
	if fromFlag != nil && *fromFlag != "" {
		*dst, r.Sources[key] = *fromFlag, FromFlag
	}
}

func resolveBool(r *Resolved, key string, dst *bool, fromFile, fromFlag *bool) {
	if fromFile != nil {
		*dst, r.Sources[key] = *fromFile, FromFile
	}
	if fromFlag != nil {
		*dst, r.Sources[key] = *fromFlag, FromFlag
	}
}

// Sample is a documented config.json a user can copy into place.
//
// JSON has no comment syntax, so the caveat about the water constant lives in a
// _comment key, which the loader ignores. The constant must never be presented
// as a measured value.
const Sample = `{
  "_comment": [
    "cca configuration. Every key is optional; a command-line flag overrides it.",
    "",
    "water_ml_per_1k_tokens is a ROUGH PLACEHOLDER, not a measured value. There is",
    "no reliable public per-token water figure for any model, and published",
    "estimates vary by more than an order of magnitude depending on datacenter",
    "location, cooling design, power mix, and whether the figure counts only",
    "on-site evaporation or also the water used to generate the electricity.",
    "Substitute your own figure if you have a better one."
  ],
  "water_ml_per_1k_tokens": 0.30,
  "claude_dir": "",
  "pricing": "",
  "no_sidechains": false,
  "ascii": false,
  "no_color": false
}
`
