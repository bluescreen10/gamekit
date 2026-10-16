package main

import (
	"image"
	"image/color"
	"math"
)

// sampleSVG is a small vector drawing for the SVG view.
const sampleSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="120" viewBox="0 0 160 120">
  <defs>
    <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="#4f8ef7"/>
      <stop offset="1" stop-color="#a8d0ff"/>
    </linearGradient>
  </defs>
  <rect width="160" height="120" rx="12" fill="url(#sky)"/>
  <circle cx="122" cy="32" r="14" fill="#ffd34d"/>
  <path d="M0 96 L44 52 L76 84 L104 60 L160 104 L160 120 L0 120 Z" fill="#2f6b3a"/>
  <path d="M0 108 L56 76 L96 100 L160 88 L160 120 L0 120 Z" fill="#3f8f4d"/>
</svg>`

// sampleRTF is a short RTF document for the rich text view.
const sampleRTF = `{\rtf1\ansi\deff0{\fonttbl{\f0 Helvetica;}}{\colortbl;\red220\green60\blue60;}` +
	`\f0\fs28 {\b Rich text} supports {\i italic}, {\ul underlined}, {\strike struck}, ` +
	`and {\cf1 colored} text.\par\fs22 Turn on editing and use the platform shortcuts for bold and italic.}`

// sampleMarkdown is the initial document of the Markdown demo.
const sampleMarkdown = `# Markdown

GameKit renders **CommonMark** with the native text system.

- Lists, *emphasis*, and ` + "`inline code`" + `
- [Links](https://github.com/bluescreen10/gamekit) that report activation

> A quotation block.

` + "```" + `
func main() {
    println("code block")
}
` + "```"

// sampleHTML is the initial page of the web view.
const sampleHTML = `<!doctype html>
<html><head><meta charset="utf-8"><style>
  body { font: 14px -apple-system, system-ui, sans-serif; margin: 16px; color: #222; background: #f6f7f9; }
  @media (prefers-color-scheme: dark) { body { color: #eee; background: #1e1f22; } }
  button { font: inherit; padding: 4px 12px; }
</style></head><body>
  <h2>Embedded web view</h2>
  <p>This page is loaded from an HTML string. <a href="https://example.com/">This link</a> navigates.</p>
  <button onclick="document.getElementById('count').textContent = ++window.clicks">Run script</button>
  Clicks: <span id="count">0</span>
  <script>window.clicks = 0;</script>
</body></html>`

// gradientImage returns a checkered gradient image of the given size.
func gradientImage(width, height int) image.Image {
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			base := uint8(45)
			if ((x/24)+(y/24))%2 == 1 {
				base = 75
			}
			result.SetRGBA(x, y, color.RGBA{
				R: uint8(40 + x*180/width),
				G: uint8(80 + y*140/height),
				B: base + 100,
				A: 255,
			})
		}
	}
	return result
}

// pixelArtImage returns a small image that shows the difference between
// smooth and nearest-neighbor interpolation.
func pixelArtImage() image.Image {
	rows := []string{
		"..XXXX..",
		".X....X.",
		"X.X..X.X",
		"X......X",
		"X.X..X.X",
		"X..XX..X",
		".X....X.",
		"..XXXX..",
	}
	result := image.NewRGBA(image.Rect(0, 0, len(rows[0]), len(rows)))
	for y, row := range rows {
		for x, cell := range row {
			pixel := color.RGBA{R: 0xff, G: 0xd3, B: 0x4d, A: 0xff}
			if cell == 'X' {
				pixel = color.RGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xff}
			}
			result.SetRGBA(x, y, pixel)
		}
	}
	return result
}

// swatchImage returns a square image of one hue, for thumbnails.
func swatchImage(size int, hue float64) image.Image {
	result := image.NewRGBA(image.Rect(0, 0, size, size))
	center := float64(size) / 2
	for y := range size {
		for x := range size {
			distance := math.Hypot(float64(x)-center, float64(y)-center) / center
			lightness := 0.65 - 0.3*min(1, distance)
			result.SetRGBA(x, y, hslColor(hue, 0.7, lightness))
		}
	}
	return result
}

// spinnerFrame returns one frame of a rotating-dot animation.
func spinnerFrame(size, frame, frameCount int) image.Image {
	result := image.NewRGBA(image.Rect(0, 0, size, size))
	angle := 2 * math.Pi * float64(frame) / float64(frameCount)
	center := float64(size) / 2
	dotX := center + math.Cos(angle)*center*0.6
	dotY := center + math.Sin(angle)*center*0.6
	for y := range size {
		for x := range size {
			distance := math.Hypot(float64(x)-dotX, float64(y)-dotY)
			pixel := hslColor(float64(frame)/float64(frameCount), 0.6, 0.85)
			if distance < center*0.25 {
				pixel = hslColor(float64(frame)/float64(frameCount), 0.8, 0.45)
			}
			result.SetRGBA(x, y, pixel)
		}
	}
	return result
}

// hslColor converts a hue, saturation, and lightness, each from 0 to 1, to
// an opaque color.
func hslColor(hue, saturation, lightness float64) color.RGBA {
	chroma := (1 - math.Abs(2*lightness-1)) * saturation
	sector := math.Mod(hue*6, 6)
	second := chroma * (1 - math.Abs(math.Mod(sector, 2)-1))
	var r, g, b float64
	switch int(sector) {
	case 0:
		r, g = chroma, second
	case 1:
		r, g = second, chroma
	case 2:
		g, b = chroma, second
	case 3:
		g, b = second, chroma
	case 4:
		r, b = second, chroma
	default:
		r, b = chroma, second
	}
	offset := lightness - chroma/2
	return color.RGBA{
		R: uint8((r + offset) * 255),
		G: uint8((g + offset) * 255),
		B: uint8((b + offset) * 255),
		A: 255,
	}
}
