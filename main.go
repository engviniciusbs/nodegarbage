package main

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/ai"
	"github.com/engviniciusbs/nodegarbage/internal/app"
	"github.com/engviniciusbs/nodegarbage/internal/cleaner"
	"github.com/engviniciusbs/nodegarbage/internal/elevation"
	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/scanner"
	"github.com/engviniciusbs/nodegarbage/internal/secrets"
	"github.com/engviniciusbs/nodegarbage/internal/store"
	"github.com/engviniciusbs/nodegarbage/internal/volumes"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	xwindows "golang.org/x/sys/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is overridden at release time via -ldflags "-X main.version=<tag>".
//lint:ignore U1000 set at release time via -ldflags
var version = "dev"

// wailsEmitter emite eventos ao frontend; o contexto é definido no OnStartup.
type wailsEmitter struct {
	ctx atomic.Pointer[context.Context]
}

func (e *wailsEmitter) Emit(event string, data any) {
	if c := e.ctx.Load(); c != nil {
		wailsruntime.EventsEmit(*c, event, data)
	}
}

func dataDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	return filepath.Join(base, "NodeGarbage")
}

func listNames(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

// isWindows10 reports whether the OS predates Windows 11 (build < 22000).
func isWindows10() bool {
	return xwindows.RtlGetVersion().BuildNumber < 22000
}

func main() {
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		println("Error:", err.Error())
		return
	}
	st, err := store.Open(filepath.Join(dir, "nodegarbage.db"))
	if err != nil {
		println("Error:", err.Error())
		return
	}
	defer st.Close()

	rs, err := rules.LoadBuiltin()
	if err != nil {
		println("Error:", err.Error())
		return
	}
	userRulesDir := filepath.Join(dir, "rules.d")
	userRules, problems := rules.LoadUserDir(userRulesDir)
	rs = append(rs, userRules...)

	// Always protect every possible drive letter, independent of volumes.List.
	roots := make([]string, 0, 26)
	for c := 'A'; c <= 'Z'; c++ {
		roots = append(roots, string(c)+`:\`)
	}
	prot := rules.NewProtector(os.Getenv, roots, nil)
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
	cl := rules.NewClassifier(rs, prot, os.Getenv, exists, listNames)

	em := &wailsEmitter{}
	var a *app.App
	planner := cleaner.NewPlanner(prot, cleaner.OSDeps(),
		func(v string) bool { return a != nil && a.IsScanned(v) }, time.Now)

	a = app.New(app.Deps{
		Store:   st,
		Volumes: volumes.List,
		NewScanner: func(_ string, workers int) scanner.Scanner {
			return &scanner.WalkScanner{FS: scanner.OSFS(), Workers: workers}
		},
		Rules: rs, RuleProblems: problems, UserRulesDir: userRulesDir, Protector: prot, Classifier: cl,
		Planner: planner, Deleter: cleaner.OSDeleter(),
		NewProvider: func(p ai.Profile, key string) (ai.Provider, error) { return ai.NewProvider(p, key, nil) },
		SecretGet:   secrets.Get, SecretSet: secrets.Set, SecretDelete: secrets.Delete,
		Emitter: em, Now: time.Now,
		Elevated: elevation.IsElevated,
		RestartElevated: func() error {
			if err := elevation.RestartElevated(os.Args[1:]); err != nil {
				return err
			}
			if c := em.ctx.Load(); c != nil {
				wailsruntime.Quit(*c)
			}
			return nil
		},
	})

	// Mica needs Windows 11 (build >= 22000). On Windows 10 fall back to a solid
	// #F0F0F0 window (no backdrop, opaque background).
	backdrop, bgAlpha := windows.Mica, uint8(0)
	if isWindows10() {
		backdrop, bgAlpha = windows.None, 255
	}

	err = wails.Run(&options.App{
		Title:     "NodeGarbage",
		Width:     1024,
		Height:    768,
		MinWidth:  1024,
		MinHeight: 680,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 240, G: 240, B: 240, A: bgAlpha},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         backdrop,
		},
		OnStartup: func(ctx context.Context) {
			em.ctx.Store(&ctx)
			app.Startup(ctx, a)
		},
		Bind: []any{
			a,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
