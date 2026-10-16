package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

func mediaCards(g *gallery) []ui.Widget {
	return []ui.Widget{
		g.card("Image view", "Scaling, alignment, interpolation, and the source scale.", g.imageViewDemo),
		g.card("Icons", "System icons by their neutral names, tints, and an icon from an image.", g.iconsDemo),
		g.card("SVG", "A vector drawing at its intrinsic size and stretched.", g.svgDemo),
		g.card("Animated image", "Eight generated frames that repeat.", g.animatedImageDemo),
		g.card("Video", "Choose a video file to play it with the native controls.", g.videoDemo),
		g.card("Web view", "An HTML string with a script, or a URL.", g.webViewDemo),
	}
}

func (g *gallery) imageViewDemo() ui.Widget {
	view := must(g.window.CreateImage(ui.ImageOptions{Source: gradientImage(192, 128), Scaling: ui.ImageScaleFit}))
	view.SetMinimumSize(ui.Size{Width: 320, Height: 200})
	g.attempt("Alternative text", func() error {
		view.SetAlternativeText("A blue and green checkered gradient")
		return nil
	})

	scalings := []ui.ImageScaling{ui.ImageScaleFit, ui.ImageScaleFill, ui.ImageScaleStretch}
	scaling := g.choice([]string{"Fit", "Fill", "Stretch"}, 0, func(index int) {
		if err := view.SetScaling(scalings[index]); err != nil {
			g.log("Scaling: %v", err)
		}
	})
	anchors := []ui.Anchor{ui.AnchorCenter, ui.AnchorTopLeft, ui.AnchorTop, ui.AnchorTopRight, ui.AnchorLeft,
		ui.AnchorRight, ui.AnchorBottomLeft, ui.AnchorBottom, ui.AnchorBottomRight}
	alignment := g.choice([]string{"Center", "Top left", "Top", "Top right", "Left", "Right", "Bottom left", "Bottom", "Bottom right"}, 0, func(index int) {
		view.SetAlignment(anchors[index])
	})
	sources := g.choice([]string{"Gradient", "Pixel art (8 × 8)"}, 0, func(index int) {
		source := gradientImage(192, 128)
		if index == 1 {
			source = pixelArtImage()
		}
		if err := view.SetImage(source); err != nil {
			g.log("Image: %v", err)
		}
	})
	interpolation := g.choice([]string{"Smooth", "Nearest neighbor"}, 0, func(index int) {
		view.SetInterpolation([]ui.Interpolation{ui.InterpolationSmooth, ui.InterpolationNearest}[index])
	})
	scale := g.choice([]string{"Source scale 1×", "Source scale 2×"}, 0, func(index int) {
		view.SetSourceScale(float64(index + 1))
	})
	controls := g.column(
		g.labeled("Source", sources),
		g.labeled("Scaling", scaling),
		g.labeled("Alignment", alignment),
		g.labeled("Interpolation", interpolation),
		g.labeled("Scale", scale),
	)
	return g.layout(ui.Row(ui.Item(view).Grow(1), ui.Item(controls)).WithGap(16))
}

func (g *gallery) iconsDemo() ui.Widget {
	names := []string{"gear", "search", "add", "remove", "delete", "edit", "info", "warning", "error", "success",
		"help", "home", "user", "refresh", "share", "copy", "folder", "star", "heart", "lock", "notification", "clock"}
	icons := make([]ui.Widget, len(names))
	for i, name := range names {
		icons[i] = g.optional(func() ui.Widget {
			icon := must(g.window.CreateIcon(ui.IconOptions{
				SystemName:        name,
				Size:              ui.Size{Width: 20, Height: 20},
				AccessibilityName: name,
			}))
			icon.SetTooltip(name)
			return icon
		})
	}
	tints := []struct {
		name  string
		color ui.SemanticColor
	}{
		{"Accent", ui.SemanticColorAccent},
		{"Success", ui.SemanticColorSuccess},
		{"Warning", ui.SemanticColorWarning},
		{"Error", ui.SemanticColorError},
		{"Secondary", ui.SemanticColorSecondary},
	}
	tinted := make([]ui.Widget, len(tints))
	for i, tint := range tints {
		tinted[i] = g.optional(func() ui.Widget {
			icon := must(g.window.CreateIcon(ui.IconOptions{
				SystemName: "star",
				Size:       ui.Size{Width: 28, Height: 28},
				Tint:       tint.color,
			}))
			icon.SetTooltip(tint.name)
			return icon
		})
	}
	fromImage := g.optional(func() ui.Widget {
		return must(g.window.CreateIcon(ui.IconOptions{
			Image:             swatchImage(32, 0.8),
			Size:              ui.Size{Width: 32, Height: 32},
			AccessibilityName: "Swatch",
		}))
	})
	return g.column(
		g.rowsOf(icons),
		g.labeled("Tints", tinted...),
		g.labeled("From an image", fromImage),
	)
}

// rowsOf places widgets in rows of twelve.
func (g *gallery) rowsOf(widgets []ui.Widget) ui.Widget {
	const perRow = 12
	rows := []ui.LayoutItem{}
	for start := 0; start < len(widgets); start += perRow {
		items := []ui.LayoutItem{}
		for _, widget := range widgets[start:min(start+perRow, len(widgets))] {
			items = append(items, ui.Item(widget).Align(ui.AlignCenter).MinimumSize(ui.Size{Width: 28, Height: 28}))
		}
		items = append(items, ui.Space())
		rows = append(rows, ui.Item(g.layout(ui.Row(items...).WithGap(10))))
	}
	return g.layout(ui.Column(rows...).WithGap(10))
}

func (g *gallery) svgDemo() ui.Widget {
	natural := must(g.window.CreateSVG(ui.SVGOptions{Source: []byte(sampleSVG), AccessibilityText: "Mountains under a sun"}))
	size := natural.IntrinsicSize()
	natural.SetMinimumSize(size)
	natural.SetMaximumSize(size)
	stretched := must(g.window.CreateSVG(ui.SVGOptions{Source: []byte(sampleSVG), AccessibilityText: "Mountains, stretched"}))
	stretched.SetMinimumSize(ui.Size{Width: 320, Height: 120})
	invalid := g.button("Load invalid SVG", func() {
		g.log("SetSource with invalid data: %v", stretched.SetSource([]byte("not an svg")))
	})
	return g.column(
		g.layout(ui.Row(ui.Item(natural), ui.Item(stretched).Grow(1)).WithGap(16)),
		g.row(g.caption(fmt.Sprintf("Intrinsic size: %d × %d", size.Width, size.Height)), invalid),
	)
}

func (g *gallery) animatedImageDemo() ui.Widget {
	const frameCount = 8
	frames := make([]ui.AnimationFrame, frameCount)
	for i := range frames {
		frames[i] = ui.AnimationFrame{Image: spinnerFrame(64, i, frameCount), Duration: 90 * time.Millisecond}
	}
	animation := must(g.window.CreateAnimatedImage(ui.AnimatedImageOptions{Frames: frames, Repeat: true}))
	fixed(animation, ui.Size{Width: 64, Height: 64})
	play := g.button("Play", func() {
		animation.Play()
		g.log("Animation playing: %t", animation.IsPlaying())
	})
	pause := g.button("Pause", func() {
		animation.Pause()
		g.log("Animation playing: %t", animation.IsPlaying())
	})
	return g.row(animation, play, pause)
}

func (g *gallery) videoDemo() ui.Widget {
	placeholder := g.caption("No video loaded.")
	holder := g.layout(ui.Column(ui.Item(placeholder)))
	position := g.value("0:00 / 0:00")
	var video ui.Video

	choose := g.button("Choose video…", func() {
		path, err := g.window.OpenFile(context.Background(), ui.OpenFileOptions{
			Title:   "Choose a video",
			Filters: []ui.FileFilter{{Name: "Videos", Patterns: []string{"*.mp4", "*.mov", "*.m4v", "*.webm"}}},
		})
		if errors.Is(err, ui.ErrCancelled) {
			g.log("Video selection cancelled")
			return
		}
		if err != nil {
			g.log("Open video: %v", err)
			return
		}
		next, err := g.window.CreateVideo(ui.VideoOptions{Source: path, ShowControls: true, Autoplay: true, Muted: true})
		if err != nil {
			g.log("Video: %v", err)
			return
		}
		next.SetMinimumSize(ui.Size{Height: 240})
		if err := holder.SetSpec(ui.Column(ui.Item(next))); err != nil {
			g.log("Video: %v", err)
			return
		}
		placeholder.SetVisible(false)
		if video != nil {
			video.Close()
		}
		video = next
		g.log("Playing %s", path)
	})
	play := g.button("Play", func() {
		if video != nil {
			video.Play()
		}
	})
	pause := g.button("Pause", func() {
		if video != nil {
			video.Pause()
		}
	})
	restart := g.button("Restart", func() {
		if video != nil {
			g.attempt("Seek", func() error {
				return video.Seek(0)
			})
		}
	})
	g.every(func(time.Time) bool {
		if video != nil && video.IsPlaying() {
			position.SetText(fmt.Sprintf("%s / %s", formatMediaTime(video.Position()), formatMediaTime(video.Duration())))
		}
		return true
	})
	return g.column(holder, g.row(choose, play, pause, restart, position))
}

func formatMediaTime(value time.Duration) string {
	seconds := int(value.Seconds())
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func (g *gallery) webViewDemo() ui.Widget {
	view := must(g.window.CreateWebView(ui.WebViewOptions{HTML: sampleHTML, AllowScripts: true}))
	view.SetMinimumSize(ui.Size{Height: 260})
	address := must(g.window.CreateTextField())
	address.SetText("https://example.com/")
	load := func() {
		if err := view.Load(address.Text()); err != nil {
			g.log("Load: %v", err)
		}
	}
	address.OnSubmit(func(string) {
		load()
	})
	view.OnNavigate(func(url string) {
		address.SetText(url)
		g.log("Web view navigated: %s", url)
	})
	visit := g.button("Go", load)
	home := g.button("Show HTML", func() {
		if err := view.LoadHTML(sampleHTML); err != nil {
			g.log("HTML: %v", err)
		}
	})
	bar := g.layout(ui.Row(ui.Item(address).Grow(1).Align(ui.AlignCenter), ui.Item(visit).Align(ui.AlignCenter), ui.Item(home).Align(ui.AlignCenter)).WithGap(8))
	return g.column(bar, view)
}
