// Package main provides a utility to extract icons from Bose-branded TrueType fonts.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type IconMapping struct {
	Hex       string `json:"hex"`
	GlyphName string `json:"glyph_name"`
	File      string `json:"file"`
	SVGFile   string `json:"svg_file,omitempty"`
}

func main() {
	fontPath := flag.String("font", "/path/to/bose.ttf", "Path to the TTF font file")
	outputDir := flag.String("output", "extracted_icons", "Output directory for icons")
	imgSize := flag.Int("size", 256, "Size of the PNG icons")

	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	data, err := os.ReadFile(*fontPath)
	if err != nil {
		log.Fatalf("Failed to read font file: %v", err)
	}

	f, err := sfnt.Parse(data)
	if err != nil {
		log.Fatalf("Failed to parse font: %v", err)
	}

	var (
		buffer     sfnt.Buffer
		glyphIndex sfnt.GlyphIndex
		glyphName  string
		segments   sfnt.Segments
		pngFile    *os.File
	)

	unitsPerEm := f.UnitsPerEm()
	ppem := fixed.Int26_6(unitsPerEm) << 6

	m, err := f.Metrics(&buffer, ppem, font.HintingNone)
	if err != nil {
		log.Fatalf("Failed to get metrics: %v", err)
	}

	mapping := make(map[rune]IconMapping)

	// Iterate through common ranges
	ranges := []struct{ start, end rune }{
		{0x20, 0x7E},     // Basic Latin
		{0xA0, 0xFF},     // Latin-1 Supplement
		{0xE000, 0xF8FF}, // Private Use Area
	}

	for _, rg := range ranges {
		for r := rg.start; r <= rg.end; r++ {
			glyphIndex, err = f.GlyphIndex(&buffer, r)
			if err != nil || glyphIndex == 0 {
				continue
			}

			glyphName, err = f.GlyphName(&buffer, glyphIndex)
			if err != nil {
				glyphName = fmt.Sprintf("uni%04X", r)
			}

			segments, err = f.LoadGlyph(&buffer, glyphIndex, ppem, nil)
			if err != nil {
				fmt.Printf("Failed to load glyph 0x%04X: %v\n", r, err)
				continue
			}

			if len(segments) == 0 {
				continue
			}

			charHex := fmt.Sprintf("%04X", r)
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

			if err = os.WriteFile(filepath.Join(*outputDir, svgFilename), []byte(svgContent), 0644); err != nil {
				fmt.Printf("Failed to write SVG 0x%s: %v\n", charHex, err)
			}

			// 2. Render PNG
			img := renderGlyphToPNG(segments, int(unitsPerEm), int(m.Ascent), int(m.Descent), *imgSize)

			pngFile, err = os.Create(filepath.Join(*outputDir, pngFilename))
			if err == nil {
				if err = png.Encode(pngFile, img); err != nil {
					fmt.Printf("Failed to encode PNG 0x%s: %v\n", charHex, err)
				}

				pngFile.Close()
			} else {
				fmt.Printf("Failed to create PNG file 0x%s: %v\n", charHex, err)
			}

			mapping[r] = IconMapping{
				Hex:       fmt.Sprintf("0x%s", charHex),
				GlyphName: glyphName,
				File:      pngFilename,
				SVGFile:   svgFilename,
			}
		}
	}

	// Save mapping.json
	mappingList := make(map[string]IconMapping)

	var keys []int

	for r, m := range mapping {
		mappingList[fmt.Sprintf("%d", r)] = m
		keys = append(keys, int(r))
	}

	sort.Ints(keys)

	jsonData, err := json.MarshalIndent(mappingList, "", "    ")
	if err != nil {
		log.Fatalf("Failed to marshal mapping: %v", err)
	}

	_ = os.WriteFile(filepath.Join(*outputDir, "mapping.json"), jsonData, 0644)

	// Save mapping.md
	mdFile, _ := os.Create(filepath.Join(*outputDir, "mapping.md"))
	fmt.Fprintln(mdFile, "# Bose Icons Mapping")
	fmt.Fprintln(mdFile, "")
	fmt.Fprintln(mdFile, "| Char Code | Glyph Name | PNG | SVG |")
	fmt.Fprintln(mdFile, "| --- | --- | --- | --- |")

	for _, k := range keys {
		m := mapping[rune(k)]
		fmt.Fprintf(mdFile, "| %s | %s | ![%s](%s) | [SVG](%s) |\n", m.Hex, m.GlyphName, m.GlyphName, m.File, m.SVGFile)
	}

	mdFile.Close()

	fmt.Printf("Extracted %d icons to %s\n", len(mapping), *outputDir)
}

func segmentsToSVGPath(segments sfnt.Segments) string {
	var path string

	for _, seg := range segments {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			path += fmt.Sprintf("M%g %g ", float64(seg.Args[0].X)/64.0, -float64(seg.Args[0].Y)/64.0)
		case sfnt.SegmentOpLineTo:
			path += fmt.Sprintf("L%g %g ", float64(seg.Args[0].X)/64.0, -float64(seg.Args[0].Y)/64.0)
		case sfnt.SegmentOpQuadTo:
			path += fmt.Sprintf("Q%g %g %g %g ", float64(seg.Args[0].X)/64.0, -float64(seg.Args[0].Y)/64.0, float64(seg.Args[1].X)/64.0, -float64(seg.Args[1].Y)/64.0)
		case sfnt.SegmentOpCubeTo:
			path += fmt.Sprintf("C%g %g %g %g %g %g ", float64(seg.Args[0].X)/64.0, -float64(seg.Args[0].Y)/64.0, float64(seg.Args[1].X)/64.0, -float64(seg.Args[1].Y)/64.0, float64(seg.Args[2].X)/64.0, -float64(seg.Args[2].Y)/64.0)
		}
	}

	return path
}

func renderGlyphToPNG(segments sfnt.Segments, _, _, _, imgSize int) image.Image {
	rgba := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))
	draw.Draw(rgba, rgba.Bounds(), image.Transparent, image.Point{}, draw.Src)

	// Calculate glyph bounds
	var xmin, ymin, xmax, ymax float64

	initialized := false

	for _, seg := range segments {
		for _, arg := range seg.Args {
			x, y := float64(arg.X)/64.0, float64(arg.Y)/64.0
			if !initialized {
				xmin, xmax = x, x
				ymin, ymax = y, y
				initialized = true
			} else {
				if x < xmin {
					xmin = x
				}

				if x > xmax {
					xmax = x
				}

				if y < ymin {
					ymin = y
				}

				if y > ymax {
					ymax = y
				}
			}
		}
	}

	w := xmax - xmin
	h := ymax - ymin

	// If no width/height, return empty image
	if w <= 0 || h <= 0 {
		return rgba
	}

	// Calculate scale to fit in imgSize with padding
	padding := 20.0
	available := float64(imgSize) - 2*padding

	scale := available / w
	if h*scale > available {
		scale = available / h
	}

	// Center the glyph
	// X: center of image (imgSize/2) - (center of glyph (xmin+xmax)/2) * scale
	offsetX := float64(imgSize)/2.0 - (xmin+xmax)/2.0*scale
	// Y: center of image (imgSize/2) - (center of glyph (ymin+ymax)/2) * scale
	offsetY := float64(imgSize)/2.0 - (ymin+ymax)/2.0*scale

	scanner := rasterx.NewScannerGV(imgSize, imgSize, rgba, rgba.Bounds())
	filler := rasterx.NewFiller(imgSize, imgSize, scanner)
	filler.SetColor(color.Black)

	for _, seg := range segments {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			filler.Start(fixedP(
				offsetX+float64(seg.Args[0].X)/64.0*scale,
				offsetY+float64(seg.Args[0].Y)/64.0*scale,
			))
		case sfnt.SegmentOpLineTo:
			filler.Line(fixedP(
				offsetX+float64(seg.Args[0].X)/64.0*scale,
				offsetY+float64(seg.Args[0].Y)/64.0*scale,
			))
		case sfnt.SegmentOpQuadTo:
			filler.QuadBezier(
				fixedP(
					offsetX+float64(seg.Args[0].X)/64.0*scale,
					offsetY+float64(seg.Args[0].Y)/64.0*scale,
				),
				fixedP(
					offsetX+float64(seg.Args[1].X)/64.0*scale,
					offsetY+float64(seg.Args[1].Y)/64.0*scale,
				),
			)
		case sfnt.SegmentOpCubeTo:
			filler.CubeBezier(
				fixedP(
					offsetX+float64(seg.Args[0].X)/64.0*scale,
					offsetY+float64(seg.Args[0].Y)/64.0*scale,
				),
				fixedP(
					offsetX+float64(seg.Args[1].X)/64.0*scale,
					offsetY+float64(seg.Args[1].Y)/64.0*scale,
				),
				fixedP(
					offsetX+float64(seg.Args[2].X)/64.0*scale,
					offsetY+float64(seg.Args[2].Y)/64.0*scale,
				),
			)
		}
	}

	filler.Stop(true)
	filler.Draw()

	return rgba
}

func fixedP(x, y float64) fixed.Point26_6 {
	return fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
}
