package app

import (
	"errors"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/ai"
	"github.com/engviniciusbs/nodegarbage/internal/cleaner"
	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/scanner"
	"github.com/engviniciusbs/nodegarbage/internal/store"
	"github.com/engviniciusbs/nodegarbage/internal/volumes"
)

var (
	errNotSupported = errors.New("operação não suportada")
	errNotScanned   = errors.New("volume not scanned")
)

// Emitter entrega eventos ao frontend.
type Emitter interface {
	Emit(event string, data any)
}

// Deps agrupa as dependências do App.
type Deps struct {
	Store      *store.Store
	Volumes    func() ([]volumes.Volume, error)
	NewScanner func(volumeRoot string, workers int) scanner.Scanner

	Rules        []rules.Rule
	RuleProblems []rules.LoadProblem
	Protector    *rules.Protector
	Classifier   *rules.Classifier

	UserRulesDir string

	Planner *cleaner.Planner
	Deleter cleaner.Deleter

	NewProvider  func(ai.Profile, string) (ai.Provider, error)
	SecretGet    func(string) (string, error)
	SecretSet    func(string, string) error
	SecretDelete func(string) error

	Emitter         Emitter
	Now             func() time.Time
	Elevated        func() bool
	RestartElevated func() error
}
