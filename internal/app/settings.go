package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/engviniciusbs/nodegarbage/internal/rules"
)

const (
	settingsKey         = "settings"
	defaultAIMinBytes   = 500 << 20
	maxRedactOrExtraLen = 500
)

type RuleProblemDTO struct {
	File   string `json:"file"`
	RuleID string `json:"ruleId"`
	Error  string `json:"error"`
}

type SettingsDTO struct {
	RedactTerms    []string         `json:"redactTerms"`
	ExtraProtected []string         `json:"extraProtected"`
	AIMinBytes     int64            `json:"aiMinBytes"`
	Workers        int              `json:"workers"`
	UserRulesDir   string           `json:"userRulesDir"`
	RuleProblems   []RuleProblemDTO `json:"ruleProblems"`
}

// storedSettings is the persisted part of SettingsDTO; the rest is derived.
type storedSettings struct {
	RedactTerms    []string `json:"redactTerms"`
	ExtraProtected []string `json:"extraProtected"`
	AIMinBytes     int64    `json:"aiMinBytes"`
	Workers        int      `json:"workers"`
}

func defaultSettings() storedSettings {
	return storedSettings{RedactTerms: []string{}, ExtraProtected: []string{}, AIMinBytes: defaultAIMinBytes}
}

// loadSettings reads the saved settings; missing or unreadable data yields
// the defaults.
func (a *App) loadSettings() storedSettings {
	s := defaultSettings()
	if a.deps.Store == nil {
		return s
	}
	b, ok, err := a.deps.Store.GetSetting(context.Background(), settingsKey)
	if err != nil || !ok {
		return s
	}
	if json.Unmarshal(b, &s) != nil {
		return defaultSettings()
	}
	if s.RedactTerms == nil {
		s.RedactTerms = []string{}
	}
	if s.ExtraProtected == nil {
		s.ExtraProtected = []string{}
	}
	return s
}

// applyExtraProtected pushes the saved extra protected paths to the Protector.
func (a *App) applyExtraProtected() {
	if a.deps.Protector != nil {
		a.deps.Protector.SetExtra(a.loadSettings().ExtraProtected)
	}
}

// GetSettings devolve as configurações (com os campos derivados).
func (a *App) GetSettings() (SettingsDTO, error) {
	s := a.loadSettings()
	out := SettingsDTO{
		RedactTerms: s.RedactTerms, ExtraProtected: s.ExtraProtected,
		AIMinBytes: s.AIMinBytes, Workers: s.Workers,
		UserRulesDir: a.deps.UserRulesDir, RuleProblems: []RuleProblemDTO{},
	}
	for _, p := range a.deps.RuleProblems {
		out.RuleProblems = append(out.RuleProblems, RuleProblemDTO{File: p.File, RuleID: p.RuleID, Error: p.Err})
	}
	return out, nil
}

func cleanList(in []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[strings.ToLower(s)] {
			continue
		}
		if len(s) > maxRedactOrExtraLen {
			return nil, errors.New("valor muito longo")
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	return out, nil
}

// SaveSettings valida e grava as configurações; os caminhos protegidos extras
// valem imediatamente (as sessões são reclassificadas). UserRulesDir e
// RuleProblems são derivados e ignorados.
func (a *App) SaveSettings(s SettingsDTO) error {
	if a.deps.Store == nil {
		return errNotSupported
	}
	if s.AIMinBytes < 0 {
		return errors.New("tamanho mínimo inválido")
	}
	if s.Workers < 0 || s.Workers > 256 {
		return errors.New("número de threads inválido")
	}
	terms, err := cleanList(s.RedactTerms)
	if err != nil {
		return err
	}
	extra, err := cleanList(s.ExtraProtected)
	if err != nil {
		return err
	}
	env := rules.Env(os.Getenv)
	for _, e := range extra {
		valid := false
		if a.deps.Protector != nil {
			valid = a.deps.Protector.ValidExtra(e)
		} else {
			valid = rules.IsAbsoluteEntry(e, env)
		}
		if !valid {
			return fmt.Errorf("caminho protegido inválido: \"%s\". Use um caminho absoluto (C:\\pasta ou \\\\servidor\\compartilhamento\\pasta) e apenas variáveis definidas (ex.: %%USERPROFILE%%)", e)
		}
	}
	st := storedSettings{RedactTerms: terms, ExtraProtected: extra, AIMinBytes: s.AIMinBytes, Workers: s.Workers}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := a.deps.Store.SetSetting(context.Background(), settingsKey, b); err != nil {
		return err
	}
	a.applyExtraProtected()
	a.reclassifyAll()
	return nil
}
