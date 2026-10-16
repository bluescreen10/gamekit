// Command widget_gallery shows every component of the ui package, with
// controls that change their states, so that a native backend can be examined
// by hand.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/bluescreen10/gamekit/ui"
	_ "github.com/bluescreen10/gamekit/ui/default"
)

func main() {
	runtime.LockOSThread()
	startPage := flag.String("page", "buttons", "ID of the first page to show, such as dialogs or input")
	smoke := flag.Bool("smoke", false, "build every page, print the events and unavailable components, and exit")
	flag.Parse()

	backend, err := ui.New(nil)
	if err != nil {
		panic(err)
	}
	window, err := backend.CreateWindow(ui.WindowOptions{
		Title:       "GameKit widget gallery",
		Size:        ui.Size{Width: 1120, Height: 780},
		MinimumSize: ui.Size{Width: 760, Height: 520},
		Resizable:   true,
		Theme:       ui.ThemeSystem,
	})
	if err != nil {
		panic(err)
	}
	defer window.Close()

	g := newGallery(backend, window)
	g.printsEvents = *smoke
	if err := g.build(*startPage); err != nil {
		panic(err)
	}
	if *smoke {
		g.showEveryPage()
		fmt.Printf("%d components are not available on this backend\n", g.unavailableCount)
		if g.unavailableCount > 0 {
			window.Close()
			os.Exit(1)
		}
		return
	}
	g.run()
}

// showEveryPage shows each page in turn and processes its events, so that
// every component is created and laid out.
func (g *gallery) showEveryPage() {
	for _, group := range g.groups {
		for _, entry := range group.pages {
			g.showPage(entry.id)
			for range 10 {
				g.window.PollEvents()
				g.runTimers(time.Now())
				time.Sleep(time.Second / 60)
			}
		}
	}
}

// run processes events and timers until the user closes the main window.
func (g *gallery) run() {
	for !g.window.ShouldClose() {
		g.window.PollEvents()
		g.runTimers(time.Now())
		time.Sleep(time.Second / 120)
	}
}
