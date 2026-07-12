package testkit

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-rod/rod"
)

type Artifacts struct {
	Actual string
	Golden string
	t      *testing.T
}

func NewArtifacts(t *testing.T) *Artifacts {
	t.Helper()

	name := safeName(t.Name())
	artifacts := &Artifacts{
		Actual: filepath.Join(".context", "artifacts", name),
		Golden: filepath.Join("testdata", "golden", name),
		t:      t,
	}
	if err := os.RemoveAll(artifacts.Actual); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifacts.Actual, 0o755); err != nil {
		t.Fatal(err)
	}
	return artifacts
}

func (a *Artifacts) ActualPath(name string) string {
	a.t.Helper()
	return filepath.Join(a.Actual, name)
}

func (a *Artifacts) GoldenPath(name string) string {
	a.t.Helper()
	return filepath.Join(a.Golden, name)
}

func (a *Artifacts) Screenshot(page *rod.Page, name string) {
	a.t.Helper()

	page.MustWaitStable()
	out, err := page.Screenshot(false, nil)
	if err != nil {
		a.t.Fatal(err)
	}
	if err := os.WriteFile(a.ActualPath(name), out, 0o644); err != nil {
		a.t.Fatal(err)
	}
}

func (a *Artifacts) MatchScreenshot(page *rod.Page, name string) {
	a.t.Helper()

	page.MustWaitStable()
	out, err := page.Screenshot(false, nil)
	if err != nil {
		a.t.Fatal(err)
	}
	if err := os.WriteFile(a.ActualPath(name), out, 0o644); err != nil {
		a.t.Fatal(err)
	}

	golden := a.GoldenPath(name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			a.t.Fatal(err)
		}
		if err := os.WriteFile(golden, out, 0o644); err != nil {
			a.t.Fatal(err)
		}
		return
	}

	expected, err := os.ReadFile(golden)
	if err != nil {
		a.t.Fatalf("read golden %s: %v", golden, err)
	}
	if !bytes.Equal(expected, out) && !similarPNG(expected, out, 0.001) {
		a.t.Fatalf("screenshot %s differs from golden %s", a.ActualPath(name), golden)
	}
}

func (a *Artifacts) MatchFile(name string, out []byte) {
	a.t.Helper()

	if err := os.WriteFile(a.ActualPath(name), out, 0o644); err != nil {
		a.t.Fatal(err)
	}

	golden := a.GoldenPath(name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			a.t.Fatal(err)
		}
		if err := os.WriteFile(golden, out, 0o644); err != nil {
			a.t.Fatal(err)
		}
		return
	}

	expected, err := os.ReadFile(golden)
	if err != nil {
		a.t.Fatalf("read golden %s: %v", golden, err)
	}
	if !bytes.Equal(expected, out) {
		a.t.Fatalf("artifact %s differs from golden %s", a.ActualPath(name), golden)
	}
}

func similarPNG(expected []byte, actual []byte, tolerance float64) bool {
	left, _, err := image.Decode(bytes.NewReader(expected))
	if err != nil {
		return false
	}
	right, _, err := image.Decode(bytes.NewReader(actual))
	if err != nil {
		return false
	}
	if left.Bounds() != right.Bounds() {
		return false
	}

	var changed int
	bounds := left.Bounds()
	total := bounds.Dx() * bounds.Dy()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if colorDistance(left.At(x, y), right.At(x, y)) > 2 {
				changed++
			}
		}
	}
	return float64(changed)/float64(total) <= tolerance
}

func colorDistance(left color.Color, right color.Color) int {
	lr, lg, lb, la := left.RGBA()
	rr, rg, rb, ra := right.RGBA()
	return abs8(lr, rr) + abs8(lg, rg) + abs8(lb, rb) + abs8(la, ra)
}

func abs8(left uint32, right uint32) int {
	if left > right {
		return int((left - right) >> 8)
	}
	return int((right - left) >> 8)
}

func safeName(name string) string {
	name = strings.ToLower(name)
	name = strings.NewReplacer("/", "-", " ", "-", "_", "-").Replace(name)
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, name)
	name = strings.Trim(name, "-")
	if name == "" {
		return "test"
	}
	return name
}
