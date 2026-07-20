package wtone

import (
	"bytes"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	tone "github.com/leraniode/wondertone/core"
	"github.com/leraniode/wondertone/palette"
)

// encTone is the TOML layout for a serialised standalone tone file.
// Kept separate from rawDoc so zero floats don't bleed into output.
type encTone struct {
	Type        string   `toml:"type"`
	Name        string   `toml:"name"`
	Description string   `toml:"description,omitempty"`
	Author      string   `toml:"author,omitempty"`
	Version     string   `toml:"version,omitempty"`
	Tags        []string `toml:"tags,omitempty"`
	Mood        string   `toml:"mood,omitempty"`
	Light       float64  `toml:"light"`
	Vibrancy    float64  `toml:"vibrancy"`
	Hue         float64  `toml:"hue"`
	Energy      float64  `toml:"energy"`
	Alpha       *float64 `toml:"alpha,omitempty"` // nil when 1.0 (default) — omitted
}

// encPalette is the TOML layout for a serialised palette file.
type encPalette struct {
	Type        string        `toml:"type"`
	Name        string        `toml:"name"`
	Description string        `toml:"description,omitempty"`
	Author      string        `toml:"author,omitempty"`
	Version     string        `toml:"version,omitempty"`
	Mood        string        `toml:"mood,omitempty"`
	Colors      []encPaletteColor `toml:"colors"`
}

// encPaletteColor is one [[colors]] entry in a serialised palette.
type encPaletteColor struct {
	Name     string  `toml:"name"`
	Light    float64 `toml:"light"`
	Vibrancy float64 `toml:"vibrancy"`
	Hue      float64 `toml:"hue"`
	Energy   float64  `toml:"energy"`
	Mood     string   `toml:"mood,omitempty"`
	Alpha    *float64 `toml:"alpha,omitempty"`
}

// --- Tone encoding ---

// SaveTone writes a standalone Tone to a .wtone file (type = "tone").
func SaveTone(path string, t tone.Tone, opts ...ToneFileOption) error {
	data, err := MarshalTone(t, opts...)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("wtone: cannot write %q: %w", path, err)
	}
	return nil
}

// MarshalTone serialises a Tone to .wtone tone file format bytes.
func MarshalTone(t tone.Tone, opts ...ToneFileOption) ([]byte, error) {
	cfg := toneFileCfg{}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.version == "" {
		cfg.version = "1.0.0"
	}

	enc := encTone{
		Type:        string(docTypeTone),
		Name:        t.Name(),
		Description: cfg.description,
		Author:      cfg.author,
		Version:     cfg.version,
		Tags:        cfg.tags,
		Mood:        t.Mood(),
		Light:       roundF(t.Light(), 2),
		Vibrancy:    roundF(t.Vibrancy(), 2),
		Hue:         roundF(t.Hue(), 2),
		Energy:      roundF(t.Energy(), 4),
	}
	if t.AlphaValue() < 1.0 {
		a := roundF(t.AlphaValue(), 4)
		enc.Alpha = &a
	}

	var buf bytes.Buffer
	buf.WriteString("# wondertone tone file — https://github.com/leraniode/wondertone\n\n")
	e := toml.NewEncoder(&buf)
	if err := e.Encode(enc); err != nil {
		return nil, fmt.Errorf("wtone: encode error: %w", err)
	}
	return buf.Bytes(), nil
}

// ToneFileOption configures tone file metadata during encoding.
type ToneFileOption func(*toneFileCfg)

type toneFileCfg struct {
	description string
	author      string
	version     string
	tags        []string
}

// WithDescription sets the description field in the tone file.
func WithDescription(s string) ToneFileOption {
	return func(c *toneFileCfg) { c.description = s }
}

// WithAuthor sets the author field in the tone file.
func WithAuthor(s string) ToneFileOption {
	return func(c *toneFileCfg) { c.author = s }
}

// WithVersion sets the version field in the tone file.
func WithVersion(s string) ToneFileOption {
	return func(c *toneFileCfg) { c.version = s }
}

// WithTags sets the tags field — used by glitter for categorisation.
func WithTags(tags ...string) ToneFileOption {
	return func(c *toneFileCfg) { c.tags = tags }
}

// --- Palette encoding ---

// SaveWTone writes a Palette to a .wtone file.
func SaveWTone(path string, p *palette.Palette) error {
	data, err := MarshalWTone(p)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("wtone: cannot write %q: %w", path, err)
	}
	return nil
}

// MarshalWTone serialises a Palette to .wtone palette file format bytes.
// Colours are written using the wondertone vocabulary (light/vibrancy/hue).
func MarshalWTone(p *palette.Palette) ([]byte, error) {
	version := p.Version()
	if version == "" {
		version = "1.0.0"
	}

	ep := encPalette{
		Type:        string(docTypePalette),
		Name:        p.Name(),
		Description: p.Description(),
		Mood:        p.Mood(),
		Author:      p.Author(),
		Version:     version,
	}

	for _, t := range p.All() {
		ec := encPaletteColor{
			Name:     t.Name(),
			Light:    roundF(t.Light(), 2),
			Vibrancy: roundF(t.Vibrancy(), 2),
			Hue:      roundF(t.Hue(), 2),
			Energy:   roundF(t.Energy(), 4),
			Mood:     t.Mood(),
		}
		// Don't repeat mood if inherited from palette
		if ec.Mood == p.Mood() {
			ec.Mood = ""
		}
		if t.AlphaValue() < 1.0 {
			a := roundF(t.AlphaValue(), 4)
			ec.Alpha = &a
		}
		ep.Colors = append(ep.Colors, ec)
	}

	var buf bytes.Buffer
	buf.WriteString("# wondertone palette file — https://github.com/leraniode/wondertone\n\n")
	e := toml.NewEncoder(&buf)
	if err := e.Encode(ep); err != nil {
		return nil, fmt.Errorf("wtone: encode error: %w", err)
	}
	return buf.Bytes(), nil
}

// roundF rounds a float64 to n decimal places.
func roundF(v float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int(v*factor+0.5)) / factor
}
