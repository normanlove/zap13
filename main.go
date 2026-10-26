package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

//go:embed ui
var uiFiles embed.FS

const winW, winH = 960, 680

// the webview gets a single html string, so inline the css and js
func page() string {
	html, _ := uiFiles.ReadFile("ui/index.html")
	css, _ := uiFiles.ReadFile("ui/style.css")
	js, _ := uiFiles.ReadFile("ui/app.js")
	return strings.NewReplacer(
		`<link rel="stylesheet" href="style.css">`, "<style>\n"+string(css)+"</style>",
		`<script src="app.js"></script>`, "<script>\n"+string(js)+"</script>",
	).Replace(string(html))
}

func main() {
	if alreadyRunning() {
		return
	}

	a, err := openATK()
	if err != nil {
		messageBox("Couldn't open the ASUS ACPI driver ("+err.Error()+").\n\n"+
			"Make sure \"ASUS System Control Interface\" is installed. It comes with "+
			"MyASUS / Armoury Crate, or you can get it from the ASUS support page for your model.",
			windows.MB_ICONERROR)
		return
	}
	au := newAuto(a)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(os.Getenv("LOCALAPPDATA"), "zap13", "WebView2"),
		WindowOptions: webview2.WindowOptions{
			Title: "zap13", Width: winW, Height: winH, Center: true, IconId: 1,
		},
	})
	if w == nil {
		messageBox("zap13 needs the Microsoft Edge WebView2 Runtime (it comes with Windows 11).\n\n"+
			"You can get it from https://developer.microsoft.com/microsoft-edge/webview2/",
			windows.MB_ICONERROR)
		return
	}
	defer w.Destroy()
	removeTitleBar(w, winW, winH)
	hwnd := uintptr(w.Window())

	// don't push anything into the page until it has loaded and asked for its initial state
	var ready atomic.Bool
	eval := func(js string) {
		if ready.Load() {
			w.Dispatch(func() { w.Eval(js) })
		}
	}
	au.onStatus = func(st status) {
		b, _ := json.Marshal(st)
		eval("onStatus(" + string(b) + ")")
		if st.Percent != nil {
			title := fmt.Sprintf("zap13 · %d%%", *st.Percent)
			w.Dispatch(func() { w.SetTitle(title) })
		}
	}
	au.onLog = func(line string) {
		b, _ := json.Marshal(line)
		eval("onLog(" + string(b) + ")")
	}

	w.Bind("getInitial", func() map[string]any {
		ready.Store(true)
		au.mu.Lock()
		defer au.mu.Unlock()
		return map[string]any{
			"sliders":  loadSliders(),
			"defaults": usbcLimits,
			"battery":  batteryLimits,
			"accent":   accentColor(),
			"status":   au.last,
			"log":      au.lastLog,
		}
	})
	w.Bind("applyLimits", func(spl, sppt, fppt int) {
		l := limits{spl, sppt, fppt}.clamp()
		saveSliders(l)
		au.setManual(l)
	})
	w.Bind("backToAuto", au.backToAuto)
	w.Bind("minimise", func() { procShowWindow.Call(hwnd, swMinimize) })
	w.Bind("closeApp", func() { procPostMessage.Call(hwnd, wmClose, 0, 0) })
	w.Bind("startDrag", func() { startDrag(hwnd) })

	go au.run()

	// follow accent colour changes from Settings
	go func() {
		last := accentColor()
		for range time.Tick(3 * time.Second) {
			if c := accentColor(); c != last {
				last = c
				eval(fmt.Sprintf("setAccent(%q)", c))
			}
		}
	}()

	w.SetHtml(page())
	w.Run()
}
