// Package app expõe a camada de aplicação ligada ao frontend via Wails.
package app

import "context"

// Deps agrupa as dependências do App (vazio por enquanto).
type Deps struct{}

// App é o tipo vinculado ao frontend.
type App struct {
	ctx  context.Context
	deps Deps
}

// New cria um App com as dependências informadas.
func New(d Deps) *App {
	return &App{deps: d}
}

// Startup é chamado quando o app inicia; guarda o contexto do runtime Wails.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}