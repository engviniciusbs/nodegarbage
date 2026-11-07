package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/ai"
	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/scanner"
	"github.com/engviniciusbs/nodegarbage/internal/store"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
)

const (
	topFilesN        = 1000
	progressInterval = 100 * time.Millisecond
)

// session é o estado em memória de um volume varrido.
type session struct {
	root       string
	tree       *tree.Tree
	classes    *rules.Classifications
	aiOpinions map[tree.NodeID]ai.Resolved // replaced, never mutated, once published
	scannedAt  time.Time
}

func volKey(v string) string {
	return strings.ToLower(strings.TrimRight(strings.ReplaceAll(v, "/", `\`), `\`))
}

func (a *App) classify(t *tree.Tree) *rules.Classifications {
	if a.deps.Classifier == nil {
		return &rules.Classifications{}
	}
	return a.deps.Classifier.Classify(t, a.deps.Now())
}

func (a *App) setSession(root string, t *tree.Tree, at time.Time) {
	s := &session{root: root, tree: t, classes: a.classify(t), aiOpinions: map[tree.NodeID]ai.Resolved{}, scannedAt: at}
	a.mu.Lock()
	a.sessions[volKey(root)] = s
	a.mu.Unlock()
}

func (a *App) session(volume string) (*session, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s := a.sessions[volKey(volume)]
	if s == nil {
		return nil, errNotScanned
	}
	return s, nil
}

// IsScanned informa se o volume tem sessão (varrido ou carregado do disco).
func (a *App) IsScanned(volume string) bool {
	_, err := a.session(volume)
	return err == nil
}

func (a *App) loadSnapshots() {
	if a.deps.Store == nil {
		return
	}
	ctx := a.baseCtx()
	latest, err := a.deps.Store.LatestScans(ctx)
	if err != nil {
		return
	}
	for vol := range latest {
		sn, err := a.deps.Store.LoadSnapshot(ctx, vol)
		if err != nil || sn == nil || sn.Tree == nil {
			continue
		}
		a.setSession(vol, sn.Tree, sn.ScannedAt)
	}
}

// StartScan varre os volumes em sequência, de forma assíncrona.
func (a *App) StartScan(roots []string) error {
	if len(roots) == 0 {
		return errors.New("nenhum volume informado")
	}
	if a.deps.NewScanner == nil {
		return errNotSupported
	}
	a.scanMu.Lock()
	if a.scanning {
		a.scanMu.Unlock()
		return errors.New("já existe uma varredura em andamento")
	}
	ctx, cancel := context.WithCancel(a.baseCtx())
	a.scanning, a.scanCancel = true, cancel
	a.scanWG.Add(1)
	a.scanMu.Unlock()

	rs := append([]string(nil), roots...)
	go func() {
		defer func() {
			cancel()
			a.scanMu.Lock()
			a.scanning, a.scanCancel = false, nil
			a.scanMu.Unlock()
			a.scanWG.Done()
		}()
		for _, r := range rs {
			if !a.scanOne(ctx, r) {
				return
			}
		}
	}()
	return nil
}

// CancelScan cancela a varredura em andamento (se houver).
func (a *App) CancelScan() {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	if a.scanCancel != nil {
		a.scanCancel()
	}
}

// waitScan bloqueia até a varredura terminar (usado em testes).
func (a *App) waitScan() { a.scanWG.Wait() }

// scanOne varre um volume; retorna false se deve parar (cancelado).
func (a *App) scanOne(ctx context.Context, root string) bool {
	start := time.Now()
	b := tree.NewBuilder(root, topFilesN)
	prog := &scanner.Progress{}
	th := NewThrottle(progressInterval, nil)
	emitProgress := func() {
		s := prog.Snapshot()
		var bps float64
		if e := time.Since(start).Seconds(); e > 0 {
			bps = float64(s.Bytes) / e
		}
		a.emit(EvScanProgress, ProgressDTO{Volume: root, Files: s.Files, Dirs: s.Dirs, Bytes: s.Bytes,
			Denied: s.Denied, Current: s.Current, BytesPerSec: bps})
	}

	stop := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		tk := time.NewTicker(progressInterval)
		defer tk.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tk.C:
				if th.Allow() {
					emitProgress()
				}
			}
		}
	}()

	err := a.deps.NewScanner(root, a.loadSettings().Workers).Scan(ctx, root, b, prog)
	close(stop)
	<-finished
	emitProgress()

	done := ScanDoneDTO{Volume: root, DurationMs: time.Since(start).Milliseconds(), DeniedSample: []string{}}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		if ctx.Err() != nil {
			done.Error = "canceled"
		} else {
			done.Error = err.Error()
		}
		a.emit(EvScanDone, done)
		return ctx.Err() == nil
	}

	t := b.Finish()
	at := a.deps.Now()
	if a.deps.Store != nil {
		sn := store.Snapshot{Volume: root, ScannedAt: at, Engine: scanner.EngineWalk,
			Duration: time.Since(start), Tree: t}
		if err := a.deps.Store.SaveSnapshot(ctx, sn); err != nil {
			done.Error = err.Error()
			a.emit(EvScanDone, done)
			return ctx.Err() == nil
		}
	}
	a.setSession(root, t, at)
	done.DeniedCount = t.DeniedCount
	if len(t.Denied) > 0 {
		n := len(t.Denied)
		if n > 100 {
			n = 100
		}
		done.DeniedSample = append([]string(nil), t.Denied[:n]...)
	}
	a.emit(EvScanDone, done)
	return true
}

// updateSession replaces the session of a volume copy-on-write: fn gets the
// current (immutable) session and returns the new one, or nil for no change.
// fn runs without the lock; the swap only commits if nobody replaced the
// session meanwhile, otherwise fn is run again on the new one. Readers
// therefore always see a complete session.
func (a *App) updateSession(key string, fn func(old *session) *session) {
	for {
		a.mu.RLock()
		old := a.sessions[key]
		a.mu.RUnlock()
		if old == nil {
			return
		}
		next := fn(old)
		if next == nil {
			return
		}
		a.mu.Lock()
		if a.sessions[key] == old {
			a.sessions[key] = next
			a.mu.Unlock()
			return
		}
		a.mu.Unlock()
	}
}

// sessionKeys lists the keys of all sessions, sorted.
func (a *App) sessionKeys() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	keys := make([]string, 0, len(a.sessions))
	for k := range a.sessions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// reclassifyAll recomputes the classification of every session (after the
// protected paths changed).
func (a *App) reclassifyAll() {
	for _, k := range a.sessionKeys() {
		a.updateSession(k, func(old *session) *session {
			n := *old
			n.classes = a.classify(old.tree)
			return &n
		})
	}
}

// effective is the classification of a node merged with the AI opinion, if any.
func (a *App) effective(s *session, id tree.NodeID) rules.Classification {
	c := s.classes.Get(s.tree, id)
	if op, ok := s.aiOpinions[id]; ok {
		c = ai.Merge(c, op)
	}
	return c
}

// allSessions returns the sessions sorted by volume key.
func (a *App) allSessions() []*session {
	keys := a.sessionKeys()
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]*session, 0, len(keys))
	for _, k := range keys {
		if s := a.sessions[k]; s != nil {
			out = append(out, s)
		}
	}
	return out
}
