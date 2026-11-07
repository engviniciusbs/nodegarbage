package app

import (
	"context"
	"errors"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/cleaner"
	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/store"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

type PlanItemDTO struct {
	Path     string   `json:"path"`
	Size     int64    `json:"size"`
	Verdict  string   `json:"verdict"`
	Source   string   `json:"source"`
	RuleID   string   `json:"ruleId"`
	Action   string   `json:"action"`
	Warnings []string `json:"warnings"`
}

type RefusalDTO struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type PlanDTO struct {
	ID         string        `json:"id"`
	Mode       string        `json:"mode"`
	Token      string        `json:"token"`
	Items      []PlanItemDTO `json:"items"`
	Refused    []RefusalDTO  `json:"refused"`
	TotalBytes int64         `json:"totalBytes"`
	ExpiresAt  time.Time     `json:"expiresAt"`
}

type ItemResultDTO struct {
	Path         string   `json:"path"`
	Status       string   `json:"status"`
	Reason       string   `json:"reason"`
	Freed        int64    `json:"freed"`
	SkippedFiles []string `json:"skippedFiles"`
	SkippedCount int      `json:"skippedCount"`
	Detail       string   `json:"detail"`
}

type CleanProgressDTO struct {
	Index  int           `json:"index"`
	Total  int           `json:"total"`
	Result ItemResultDTO `json:"result"`
}

type CleanDoneDTO struct {
	Freed   int64           `json:"freed"`
	Results []ItemResultDTO `json:"results"`
}

type CleanupDTO struct {
	Path   string    `json:"path"`
	Size   int64     `json:"size"`
	Mode   string    `json:"mode"`
	Source string    `json:"source"`
	RuleID string    `json:"ruleId"`
	Status string    `json:"status"`
	Reason string    `json:"reason"`
	Freed  int64     `json:"freed"`
	At     time.Time `json:"at"`
}

type HistoryDTO struct {
	TotalFreed int64        `json:"totalFreed"`
	Items      []CleanupDTO `json:"items"`
}

// locate finds the node of path in the sessions. Size, file count and
// classification of a plan request always come from here, never from the UI.
func (a *App) locate(path string) (*session, tree.NodeID, bool) {
	for _, s := range a.allSessions() {
		if id, ok := s.tree.Find(path); ok && !s.tree.IsRemoved(id) {
			return s, id, true
		}
	}
	return nil, tree.NoNode, false
}

// PlanClean valida os caminhos e devolve um plano com token de uso único.
// Não apaga nada.
func (a *App) PlanClean(paths []string, mode string) (PlanDTO, error) {
	if a.deps.Planner == nil {
		return PlanDTO{}, errNotSupported
	}
	var refused []RefusalDTO
	var reqs []cleaner.Request
	files := map[string]uint32{}
	for _, p := range paths {
		s, id, ok := a.locate(p)
		if !ok {
			refused = append(refused, RefusalDTO{Path: p, Reason: cleaner.ReasonNotScanned})
			continue
		}
		path := s.tree.Path(id)
		reqs = append(reqs, cleaner.Request{Path: path, Size: s.tree.Nodes[id].TotalSize, Class: a.effective(s, id)})
		files[path] = s.tree.Nodes[id].TotalFiles
	}
	plan, err := a.deps.Planner.Plan(reqs, cleaner.Mode(mode), func(p string) uint32 { return files[p] })
	if err != nil {
		return PlanDTO{}, err
	}
	out := PlanDTO{ID: plan.ID, Mode: string(plan.Mode), Token: plan.Token, TotalBytes: plan.TotalBytes,
		ExpiresAt: plan.ExpiresAt, Items: []PlanItemDTO{}, Refused: refused}
	if out.Refused == nil {
		out.Refused = []RefusalDTO{}
	}
	for _, it := range plan.Items {
		src := string(it.Class.Source)
		if src == "" {
			src = string(rules.SourceUnknown)
		}
		w := it.Warnings
		if w == nil {
			w = []string{}
		}
		out.Items = append(out.Items, PlanItemDTO{Path: it.Path, Size: it.Size, Verdict: verdictStr(it.Class.Verdict),
			Source: src, RuleID: it.Class.RuleID, Action: string(it.Class.Action), Warnings: w})
	}
	for _, r := range plan.Refused {
		out.Refused = append(out.Refused, RefusalDTO{Path: r.Path, Reason: r.Reason})
	}
	return out, nil
}

func resultDTO(r cleaner.ItemResult) ItemResultDTO {
	sf := r.SkippedFiles
	if sf == nil {
		sf = []string{}
	}
	return ItemResultDTO{Path: r.Path, Status: string(r.Status), Reason: r.Reason, Freed: r.Freed,
		SkippedFiles: sf, SkippedCount: r.SkippedCount, Detail: r.Detail}
}

// ExecuteClean é o ÚNICO caminho que apaga arquivos: resgata o plano com o
// token (uso único) e só então chama cleaner.Execute. Assíncrono: emite
// clean:progress e clean:done.
func (a *App) ExecuteClean(planID, token string) error {
	if a.deps.Planner == nil || a.deps.Deleter == nil {
		return errNotSupported
	}
	a.cleanMu.Lock()
	if a.cleaning {
		a.cleanMu.Unlock()
		return errors.New("já existe uma limpeza em andamento")
	}
	a.cleaning = true // reserved before Redeem, which consumes the plan
	a.cleanMu.Unlock()
	release := func() {
		a.cleanMu.Lock()
		a.cleaning, a.cleanCancel = false, nil
		a.cleanMu.Unlock()
	}
	plan, err := a.deps.Planner.Redeem(planID, token)
	if err != nil {
		release()
		return err
	}
	ctx, cancel := context.WithCancel(a.baseCtx())
	a.cleanMu.Lock()
	a.cleanCancel = cancel
	a.cleanMu.Unlock()
	a.cleanWG.Add(1)

	go func() {
		defer func() {
			cancel()
			release()
			a.cleanWG.Done()
		}()
		a.runClean(ctx, plan)
	}()
	return nil
}

func (a *App) runClean(ctx context.Context, plan cleaner.Plan) {
	total := len(plan.Items)
	th := NewThrottle(progressInterval, nil)
	onItem := func(i int, r cleaner.ItemResult) {
		it := plan.Items[i]
		a.recordCleanup(plan, it, r)
		if th.Allow() || i == total-1 {
			a.emit(EvCleanProgress, CleanProgressDTO{Index: i, Total: total, Result: resultDTO(r)})
		}
	}
	results := cleaner.Execute(ctx, plan, a.deps.Deleter, a.deps.Planner.Recheck, onItem)
	a.applyCleanResults(plan, results)

	done := CleanDoneDTO{Results: make([]ItemResultDTO, 0, len(results))}
	for _, r := range results {
		done.Freed += r.Freed
		done.Results = append(done.Results, resultDTO(r))
	}
	a.emit(EvCleanDone, done)
}

func (a *App) recordCleanup(plan cleaner.Plan, it cleaner.PlanItem, r cleaner.ItemResult) {
	if a.deps.Store == nil {
		return
	}
	src := string(it.Class.Source)
	if src == "" {
		src = string(rules.SourceUnknown)
	}
	// History is written even when the app is shutting down.
	_ = a.deps.Store.AddCleanup(context.Background(), store.CleanupRecord{
		Path: it.Path, Size: it.Size, Mode: string(plan.Mode), Source: src, RuleID: it.Class.RuleID,
		Status: string(r.Status), Reason: r.Reason, Freed: r.Freed, At: a.deps.Now(),
	})
}

// applyCleanResults removes the fully deleted items from the sessions
// (copy-on-write, one tree clone per session) and updates ancestor totals.
// Partially deleted items (some files were locked) stay in the tree.
func (a *App) applyCleanResults(plan cleaner.Plan, results []cleaner.ItemResult) {
	for _, k := range a.sessionKeys() {
		a.updateSession(k, func(old *session) *session {
			var t *tree.Tree
			for i, r := range results {
				if r.Status != cleaner.StatusOK || r.SkippedCount > 0 {
					continue
				}
				cur := old.tree
				if t != nil {
					cur = t
				}
				id, ok := cur.Find(plan.Items[i].Path)
				if !ok {
					continue
				}
				if t == nil {
					t = old.tree.Clone()
				}
				if plan.Items[i].Class.Action == rules.ActionDeleteContents {
					t.RemoveContents(id)
				} else {
					t.Remove(id)
				}
			}
			if t == nil {
				return nil
			}
			n := *old
			n.tree = t
			return &n
		})
	}
}

// CancelClean cancela a limpeza em andamento; os itens restantes são pulados.
func (a *App) CancelClean() {
	a.cleanMu.Lock()
	defer a.cleanMu.Unlock()
	if a.cleanCancel != nil {
		a.cleanCancel()
	}
}

// waitClean bloqueia até a limpeza terminar (usado em testes).
func (a *App) waitClean() { a.cleanWG.Wait() }

// GetHistory devolve o histórico de limpezas (mais recentes primeiro).
func (a *App) GetHistory(limit int) (HistoryDTO, error) {
	if a.deps.Store == nil {
		return HistoryDTO{}, errNotSupported
	}
	ctx := context.Background()
	recs, err := a.deps.Store.ListCleanups(ctx, limit)
	if err != nil {
		return HistoryDTO{}, err
	}
	total, err := a.deps.Store.TotalFreed(ctx)
	if err != nil {
		return HistoryDTO{}, err
	}
	out := HistoryDTO{TotalFreed: total, Items: make([]CleanupDTO, 0, len(recs))}
	for _, r := range recs {
		out.Items = append(out.Items, CleanupDTO{Path: r.Path, Size: r.Size, Mode: r.Mode, Source: r.Source,
			RuleID: r.RuleID, Status: r.Status, Reason: r.Reason, Freed: r.Freed, At: r.At})
	}
	return out, nil
}
