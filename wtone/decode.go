// Package wtone handles reading and writing .wtone files.
//
// .wtone is wondertone's native, human-editable format built on TOML.
// It supports two document types: standalone tones and palettes.
//
// # Standalone tone (type = "tone")
//
// The simplest possible contribution — no Go required:
//
//	type        = "tone"
//	name        = "Petrichor"
//	description = "the smell of rain on dry earth"
//	author      = "leraniode"
//	tags        = ["nature", "calm"]
//
//	light    = 48
//	vibrancy = 35
//	hue      = 158
//	energy   = 0.7
//	mood     = "calm"
//
// # Palette (type = "palette", or omit type — backwards compatible)
//
//	type        = "palette"
//	name        = "Obsidian"
//	description = "deep volcanic dark theme"
//	mood        = "deep"
//	author      = "leraniode"
//
//	[[colors]]
//	name     = "Base"
//	light    = 12        # wondertone vocabulary (preferred)
//	vibrancy = 8
//	hue      = 240
//	energy   = 1.0
//
//	[[colors]]
//	name  = "Accent"
//	oklch = "0.60 0.20 142"  # raw OKLCH shorthand also accepted
//
// # Colour definition — three equivalent forms (pick one per entry)
//
//	# Form 1 — wondertone vocabulary (most readable, recommended)
//	light    = 65
//	vibrancy = 80
//	hue      = 30
//
//	# Form 2 — raw OKLCH shorthand
//	oklch = "0.65 0.18 30"
//
//	# Form 3 — hex (parsed via FromHex, WonderMath applied on load)
//	hex = "#f5a04a"
//
//	# Form 4 — explicit raw OKLCH fields (power users)
//	l = 0.65
//	c = 0.18
//	h = 30.0
package wtone

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	tone "github.com/leraniode/wondertone/core"
	"github.com/leraniode/wondertone/palette"
)

// docType distinguishes standalone tone files from palette files.
type docType string

const (
	docTypeTone    docType = "tone"
	docTypePalette docType = "palette"
)

// rawDoc is the top-level TOML structure for any .wtone file.
// Fields used depends on Type.
type rawDoc struct {
	Type        string   `toml:"type"` // "tone" or "palette" (default: "palette")
	Name        string   `toml:"name"`
	Description string   `toml:"description,omitempty"`
	Author      string   `toml:"author,omitempty"`
	Version     string   `toml:"version,omitempty"`
	Tags        []string `toml:"tags,omitempty"` // glitter categorisation: ["nature", "calm"]

	// Tone-level fields (used when Type = "tone")
	Mood     string  `toml:"mood,omitempty"`
	Light    float64 `toml:"light"`           // wondertone vocabulary [0–100]
	Vibrancy float64 `toml:"vibrancy"`        // wondertone vocabulary [0–100]
	Hue      float64 `toml:"hue"`             // wondertone vocabulary [0–360)
	Energy   float64 `toml:"energy"`          // [0–1]
	Alpha    float64 `toml:"alpha,omitempty"` // [0–1]
	Hex      string  `toml:"hex,omitempty"`   // "#rrggbb" — parsed via FromHex
	OKLCH    string  `toml:"oklch,omitempty"` // "L C H" shorthand
	L        float64 `toml:"l"`               // raw OKLCH L [0–1]
	C        float64 `toml:"c"`               // raw OKLCH C
	H        float64 `toml:"h"`               // raw OKLCH H [0–360)

	// Palette-level fields (used when Type = "palette" or unset)
	Colors []rawColor `toml:"colors"`
}

// rawColor is one [[colors]] entry inside a palette .wtone file.
type rawColor struct {
	Name string   `toml:"name"`
	Mood string   `toml:"mood,omitempty"`
	Tags []string `toml:"tags,omitempty"`

	// Colour definition — pick one form; priority: wondertone > oklch > hex > l/c/h
	Light    float64 `toml:"light"`           // wondertone vocabulary [0–100]
	Vibrancy float64 `toml:"vibrancy"`        // wondertone vocabulary [0–100]
	Hue      float64 `toml:"hue"`             // wondertone vocabulary [0–360)
	OKLCH    string  `toml:"oklch,omitempty"` // "L C H" shorthand
	Hex      string  `toml:"hex,omitempty"`   // "#rrggbb"
	L        float64 `toml:"l"`               // raw OKLCH L
	C        float64 `toml:"c"`               // raw OKLCH C
	H        float64 `toml:"h"`               // raw OKLCH H

	Energy float64 `toml:"energy"`
	Alpha  float64 `toml:"alpha,omitempty"`
}

// --- Tone file (type = "tone") ---

// LoadTone parses a standalone .wtone tone file and returns a Tone.
func LoadTone(path string) (tone.Tone, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tone.Tone{}, fmt.Errorf("wtone: cannot read %q: %w", path, err)
	}
	return ParseTone(data)
}

// ParseTone parses a standalone .wtone tone file from bytes.
// Useful with //go:embed.
func ParseTone(data []byte) (tone.Tone, error) {
	var doc rawDoc
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return tone.Tone{}, fmt.Errorf("wtone: TOML parse error: %w", err)
	}
	if doc.Type != "" && docType(doc.Type) != docTypeTone {
		return tone.Tone{}, fmt.Errorf("wtone: expected type = %q, got %q", docTypeTone, doc.Type)
	}
	if doc.Name == "" {
		return tone.Tone{}, fmt.Errorf("wtone: tone file must have a name")
	}
	return rawDocToTone(doc)
}

// rawDocToTone converts a rawDoc (type=tone) to a Tone.
func rawDocToTone(doc rawDoc) (tone.Tone, error) {
	rc := rawColor{
		Name:     doc.Name,
		Mood:     doc.Mood,
		Light:    doc.Light,
		Vibrancy: doc.Vibrancy,
		Hue:      doc.Hue,
		Energy:   doc.Energy,
		Alpha:    doc.Alpha,
		Hex:      doc.Hex,
		OKLCH:    doc.OKLCH,
		L:        doc.L,
		C:        doc.C,
		H:        doc.H,
	}
	return rawColorToTone(rc, "")
}

// --- Palette file (type = "palette" or unset) ---

// LoadWTone parses a .wtone palette file and returns a Palette.
// Backwards compatible — files without a type field are treated as palettes.
func LoadWTone(path string) (*palette.Palette, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("wtone: cannot read %q: %w", path, err)
	}
	return ParseWTone(data)
}

// ParseWTone parses .wtone palette file contents from a byte slice.
// Useful for embedding .wtone files with //go:embed.
func ParseWTone(data []byte) (*palette.Palette, error) {
	var doc rawDoc
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil, fmt.Errorf("wtone: TOML parse error: %w", err)
	}
	if docType(doc.Type) == docTypeTone {
		return nil, fmt.Errorf("wtone: file has type = %q — use ParseTone instead", docTypeTone)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("wtone: palette must have a name")
	}
	if len(doc.Colors) == 0 {
		return nil, fmt.Errorf("wtone: palette %q has no colors", doc.Name)
	}

	b := palette.New(doc.Name).
		Description(doc.Description).
		Mood(doc.Mood).
		Author(doc.Author).
		Version(doc.Version)

	for i, rc := range doc.Colors {
		t, err := rawColorToTone(rc, doc.Mood)
		if err != nil {
			return nil, fmt.Errorf("wtone: color[%d] in %q: %w", i, doc.Name, err)
		}
		b.Add(t)
	}

	return b.Build()
}

// --- Shared colour parsing ---

// rawColorToTone converts a rawColor entry to a Tone.
// Colour definition priority:
//  1. wondertone vocabulary (light/vibrancy/hue) — most readable
//  2. oklch shorthand ("L C H")
//  3. hex string ("#rrggbb")
//  4. explicit l/c/h fields
func rawColorToTone(rc rawColor, paletteMood string) (tone.Tone, error) {
	if rc.Name == "" {
		return tone.Tone{}, fmt.Errorf("every colour entry must have a name")
	}

	energy := rc.Energy
	if energy == 0 {
		energy = 1.0
	}
	alpha := rc.Alpha
	if alpha == 0 {
		alpha = 1.0
	}
	mood := rc.Mood
	if mood == "" {
		mood = paletteMood
	}

	var t tone.Tone

	switch {
	case rc.Light != 0 || rc.Vibrancy != 0 || rc.Hue != 0:
		// Form 1: wondertone vocabulary — Light, Vibrancy, Hue
		// Zero values are valid (e.g. a pure black has Light=0)
		// We use this branch if ANY of the three fields is set non-zero.
		// Edge case: a tone with Light=0, Vibrancy=0, Hue=0 is valid (pure black)
		// and will fall through to l/c/h. This is acceptable.
		t = tone.New(
			tone.Light(rc.Light),
			tone.Vibrancy(rc.Vibrancy),
			tone.Hue(rc.Hue),
		)

	case rc.OKLCH != "":
		// Form 2: raw OKLCH shorthand "L C H"
		l, c, h, _, err := parseOKLCHShorthand(rc.OKLCH)
		if err != nil {
			return tone.Tone{}, fmt.Errorf("invalid oklch %q: %w", rc.OKLCH, err)
		}
		t = tone.FromOKLCH(l, c, h)

	case rc.Hex != "":
		// Form 3: hex string
		var err error
		t, err = tone.FromHex(rc.Hex)
		if err != nil {
			return tone.Tone{}, fmt.Errorf("invalid hex %q: %w", rc.Hex, err)
		}

	default:
		// Form 4: explicit l/c/h fields
		t = tone.FromOKLCH(rc.L, rc.C, rc.H)
	}

	return t.
		WithName(rc.Name).
		WithMood(mood).
		WithEnergy(energy).
		WithAlpha(alpha), nil
}

// parseOKLCHShorthand parses "L C H" or "L C H / A".
func parseOKLCHShorthand(s string) (l, c, h, a float64, err error) {
	s = strings.TrimSpace(s)
	a = 1.0

	if idx := strings.Index(s, "/"); idx >= 0 {
		alphaPart := strings.TrimSpace(s[idx+1:])
		s = strings.TrimSpace(s[:idx])
		a, err = strconv.ParseFloat(alphaPart, 64)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid alpha %q", alphaPart)
		}
	}

	parts := strings.Fields(s)
	if len(parts) != 3 {
		return 0, 0, 0, 0, fmt.Errorf("expected 3 values (L C H), got %d", len(parts))
	}

	vals := [3]*float64{&l, &c, &h}
	names := [3]string{"L", "C", "H"}
	for i, p := range parts {
		*vals[i], err = strconv.ParseFloat(p, 64)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid %s value %q", names[i], p)
		}
	}
	return l, c, h, a, nil
}
