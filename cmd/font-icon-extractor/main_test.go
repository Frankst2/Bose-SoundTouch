package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func TestExtractE115(t *testing.T) {
	fontPath := "testdata/bose_subset.ttf"
	outputDir := "test_output"
	refDir := "testdata/references"
	imgSize := 256
	targetRune := rune(0xE115)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	data, err := os.ReadFile(fontPath)
	if err != nil {
		t.Fatalf("Failed to read font file: %v", err)
	}

	f, err := sfnt.Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse font: %v", err)
	}

	var buffer sfnt.Buffer
	unitsPerEm := f.UnitsPerEm()
	ppem := fixed.Int26_6(unitsPerEm) << 6

	m, err := f.Metrics(&buffer, ppem, font.HintingNone)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	glyphIndex, err := f.GlyphIndex(&buffer, targetRune)
	if err != nil || glyphIndex == 0 {
		t.Fatalf("Failed to find glyph for 0x%X", targetRune)
	}

	segments, err := f.LoadGlyph(&buffer, glyphIndex, ppem, nil)
	if err != nil {
		t.Fatalf("Failed to load glyph 0x%X: %v", targetRune, err)
	}

	charHex := fmt.Sprintf("%04X", targetRune)
	pngFilename := fmt.Sprintf("icon_%s.png", charHex)
	svgFilename := fmt.Sprintf("icon_%s.svg", charHex)

	// 1. Extract SVG
	svgPath := segmentsToSVGPath(segments)
	totalHeight := float64(m.Ascent+m.Descent) / 64.0
	svgContent := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 %g %d %g">
  <g transform="scale(1, -1)">
    <path d="%s" />
  </g>
</svg>`, -float64(m.Ascent)/64.0, int(unitsPerEm), totalHeight, svgPath)

	svgFilePath := filepath.Join(outputDir, svgFilename)
	if err = os.WriteFile(svgFilePath, []byte(svgContent), 0644); err != nil {
		t.Errorf("Failed to write SVG 0x%s: %v", charHex, err)
	}

	// 2. Render PNG
	img := renderGlyphToPNG(segments, int(unitsPerEm), int(m.Ascent), int(m.Descent), imgSize)

	pngFilePath := filepath.Join(outputDir, pngFilename)
	pngFile, err := os.Create(pngFilePath)
	if err == nil {
		if err = png.Encode(pngFile, img); err != nil {
			t.Errorf("Failed to encode PNG 0x%s: %v", charHex, err)
		}
		pngFile.Close()
	} else {
		t.Errorf("Failed to create PNG file 0x%s: %v", charHex, err)
	}

	// 3. Compare with references
	for _, filename := range []string{svgFilename, pngFilename} {
		generated, err := os.ReadFile(filepath.Join(outputDir, filename))
		if err != nil {
			t.Errorf("Failed to read generated file %s: %v", filename, err)
			continue
		}
		reference, err := os.ReadFile(filepath.Join(refDir, filename))
		if err != nil {
			t.Errorf("Failed to read reference file %s: %v", filename, err)
			continue
		}
		if string(generated) != string(reference) {
			t.Errorf("Mismatch in %s: generated does not match reference", filename)
		}
	}
}
