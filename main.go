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
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

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
	userRules, problems := rules.LoadUserDir(filepath.Join(dir, "rules.d"))
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
		Store:      st,
		Volumes:    volumes.List,
		NewScanner: func(string) scanner.Scanner { return &scanner.WalkScanner{FS: scanner.OSFS()} },
		Rules:      rs, RuleProblems: problems, Protector: prot, Classifier: cl,
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

	err = wails.Run(&options.App{
		Title:     "NodeGarbage",
		Width:     1024,
		Height:    768,
		MinWidth:  1024,
		MinHeight: 680,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			em.ctx.Store(&ctx)
			a.Startup(ctx)
		},
		Bind: []any{
			a,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
