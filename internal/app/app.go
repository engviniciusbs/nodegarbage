// Package app expõe a camada de aplicação ligada ao frontend via Wails.
package app

import (
	"context"
	"sync"
	"time"
)

// App é o tipo vinculado ao frontend.
type App struct {
	ctx  context.Context
	deps Deps

	mu       sync.RWMutex
	sessions map[string]*session

	scanMu     sync.Mutex
	scanning   bool
	scanCancel context.CancelFunc
	scanWG     sync.WaitGroup
}

// New cria um App com as dependências informadas.
func New(d Deps) *App {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &App{deps: d, sessions: map[string]*session{}}
}

// Startup é chamado quando o app inicia; guarda o contexto do runtime Wails e
// carrega os snapshots salvos, reclassificando-os.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.loadSnapshots()
}

func (a *App) baseCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) emit(event string, data any) {
	if a.deps.Emitter != nil {
		a.deps.Emitter.Emit(event, data)
	}
}

// IsElevated informa se o processo roda como administrador.
func (a *App) IsElevated() bool {
	if a.deps.Elevated == nil {
		return false
	}
	return a.deps.Elevated()
}

// RestartElevated reinicia o app com privilégios elevados.
func (a *App) RestartElevated() error {
	if a.deps.RestartElevated == nil {
		return errNotSupported
	}
	return a.deps.RestartElevated()
}
