package wtone_test

// wtone_test.go covers palette .wtone file features:
// vocabulary forms in [[colors]], mixed definition forms, mood
// inheritance, and serialisation with vocabulary fields.
//
// Standalone tone file tests, LoadAny, and Validate are in tone_test.go.

import (
	"strings"
	"testing"

	tone "github.com/leraniode/wondertone/core"
	"github.com/leraniode/wondertone/palette"
	"github.com/leraniode/wondertone/wtone"
	"github.com/leraniode/x/wtone/testutil"
)

// --- Palette parsing ---

func TestParseWToneWondertoneVocabulary(t *testing.T) {
	src := []byte(`
type        = "palette"
name        = "Forest"
description = "deep forest tones"
mood        = "calm"
author      = "leraniode"

[[colors]]
name     = "Moss"
light    = 42
vibrancy = 55
hue      = 138
energy   = 0.8

[[colors]]
name     = "Bark"
light    = 28
vibrancy = 20
hue      = 32
`)
	p, err := wtone.ParseWTone(src)
	testutil.NoError(t, err)
	testutil.Equal(t, "Forest", p.Name())
	testutil.Equal(t, 2, p.Len())

	moss, ok := p.Get("Moss")
	testutil.True(t, ok, "Moss should exist")
	testutil.InDelta(t, 42.0, moss.Light(), 0.2)
	testutil.InDelta(t, 0.8, moss.Energy(), 1e-3)
}

func TestParseWToneMixedForms(t *testing.T) {
	src := []byte(`
type = "palette"
name = "Mixed"

[[colors]]
name     = "ByVocab"
light    = 65
vibrancy = 80
hue      = 30

[[colors]]
name  = "ByOKLCH"
oklch = "0.55 0.18 200"

[[colors]]
name = "ByHex"
hex  = "#4ecb71"

[[colors]]
name = "ByRaw"
l    = 0.60
c    = 0.14
h    = 142.0
`)
	p, err := wtone.ParseWTone(src)
	testutil.NoError(t, err)
	testutil.Equal(t, 4, p.Len())

	for _, name := range []string{"ByVocab", "ByOKLCH", "ByHex", "ByRaw"} {
		_, ok := p.Get(name)
		testutil.True(t, ok, "%s should parse", name)
	}
}

func TestParseWToneNoTypeDefaultsToPalette(t *testing.T) {
	// Backwards compat — files without type field parse as palettes
	src := []byte(`
name = "Legacy"
[[colors]]
name  = "Old"
oklch = "0.65 0.18 142"
`)
	p, err := wtone.ParseWTone(src)
	testutil.NoError(t, err)
	testutil.Equal(t, "Legacy", p.Name())
}

func TestParseWToneRejectsToneType(t *testing.T) {
	src := []byte(`
type  = "tone"
name  = "Wrongly sent to ParseWTone"
light = 50
`)
	_, err := wtone.ParseWTone(src)
	testutil.Error(t, err)
}

func TestParseWToneMoodInheritance(t *testing.T) {
	src := []byte(`
type = "palette"
name = "Moody"
mood = "serene"

[[colors]]
name     = "Inherits"
light    = 70
vibrancy = 30
hue      = 200

[[colors]]
name     = "Overrides"
light    = 50
vibrancy = 80
hue      = 10
mood     = "urgent"
`)
	p, err := wtone.ParseWTone(src)
	testutil.NoError(t, err)

	inherits, _ := p.Get("Inherits")
	overrides, _ := p.Get("Overrides")
	testutil.Equal(t, "serene", inherits.Mood())
	testutil.Equal(t, "urgent", overrides.Mood())
}

// --- Palette serialisation ---

func TestMarshalWToneUsesVocabularyFields(t *testing.T) {
	p, _ := palette.New("Test").
		Add(tone.New(tone.Light(60), tone.Vibrancy(75), tone.Hue(142), tone.Named("Leaf"))).
		Build()

	data, err := wtone.MarshalWTone(p)
	testutil.NoError(t, err)

	s := string(data)
	testutil.True(t, strings.Contains(s, "light"), "should use light field")
	testutil.True(t, strings.Contains(s, "vibrancy"), "should use vibrancy field")
	testutil.True(t, strings.Contains(s, "hue"), "should use hue field")
	testutil.True(t, strings.Contains(s, `type = "palette"`), "should have type field")
	// Should NOT contain raw l/c/h (those are for power users; serialiser uses vocab)
	testutil.True(t, !strings.Contains(s, "\nl = "), "should not use raw l field")
}

func TestMarshalWToneRoundTrip(t *testing.T) {
	original, _ := palette.New("Leraniode").
		Mood("focused").
		Author("leraniode").
		Description("the original tones").
		Add(tone.New(tone.Light(68), tone.Vibrancy(72), tone.Hue(142), tone.Named("Unix"), tone.Moody("focused"))).
		Add(tone.New(tone.Light(45), tone.Vibrancy(60), tone.Hue(285), tone.Named("Starlight"), tone.Moody("mystical"))).
		Build()

	data, err := wtone.MarshalWTone(original)
	testutil.NoError(t, err)

	recovered, err := wtone.ParseWTone(data)
	testutil.NoError(t, err)
	testutil.Equal(t, "Leraniode", recovered.Name())
	testutil.Equal(t, "leraniode", recovered.Author())
	testutil.Equal(t, 2, recovered.Len())

	unix, ok := recovered.Get("Unix")
	testutil.True(t, ok)
	testutil.Equal(t, "focused", unix.Mood())
	// Values should survive round-trip within rounding tolerance
	testutil.InDelta(t, 68.0, unix.Light(), 0.2)
	testutil.InDelta(t, 72.0, unix.Vibrancy(), 0.2)
	testutil.InDelta(t, 142.0, unix.Hue(), 0.2)
}
