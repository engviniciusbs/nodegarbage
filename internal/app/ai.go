package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/engviniciusbs/nodegarbage/internal/ai"
	"github.com/engviniciusbs/nodegarbage/internal/rules"
	"github.com/engviniciusbs/nodegarbage/internal/tree"
	"github.com/engviniciusbs/nodegarbage/internal/volumes"
)

const (
	profilesKey    = "ai.profiles"
	testAITimeout  = 60 * time.Second
	maxProfileText = 300
)

type AIProfileDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
	HasKey  bool   `json:"hasKey"`
}

type PayloadPreviewDTO struct {
	JSON     string `json:"json"`
	Tokens   int    `json:"tokens"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Cached   bool   `json:"cached"`
}

type AIItemDTO struct {
	Path       string  `json:"path"`
	Verdict    string  `json:"verdict"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

type AIReportDTO struct {
	Markdown  string      `json:"markdown"`
	Items     []AIItemDTO `json:"items"`
	CreatedAt time.Time   `json:"createdAt"`
}

// AIDoneDTO é o payload do evento ai:done.
type AIDoneDTO struct {
	Report *AIReportDTO `json:"report"`
	Error  string       `json:"error"`
}

// storedProfile is a profile as persisted (never holds the API key).
type storedProfile struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
}

func (p storedProfile) profile() ai.Profile {
	return ai.Profile{ID: p.ID, Name: p.Name, Kind: ai.Kind(p.Kind), BaseURL: p.BaseURL, Model: p.Model}
}

func newProfileID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *App) loadProfiles() ([]storedProfile, error) {
	if a.deps.Store == nil {
		return nil, errNotSupported
	}
	b, ok, err := a.deps.Store.GetSetting(context.Background(), profilesKey)
	if err != nil {
		return nil, err
	}
	var out []storedProfile
	if ok && len(b) > 0 {
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (a *App) storeProfiles(ps []storedProfile) error {
	if ps == nil {
		ps = []storedProfile{}
	}
	b, err := json.Marshal(ps)
	if err != nil {
		return err
	}
	return a.deps.Store.SetSetting(context.Background(), profilesKey, b)
}

func (a *App) hasKey(id string) bool {
	if a.deps.SecretGet == nil {
		return false
	}
	_, err := a.deps.SecretGet(id)
	return err == nil
}

func (a *App) profileDTO(p storedProfile) AIProfileDTO {
	return AIProfileDTO{ID: p.ID, Name: p.Name, Kind: p.Kind, BaseURL: p.BaseURL, Model: p.Model, HasKey: a.hasKey(p.ID)}
}

func (a *App) findProfile(id string) (storedProfile, error) {
	ps, err := a.loadProfiles()
	if err != nil {
		return storedProfile{}, err
	}
	for _, p := range ps {
		if p.ID == id {
			return p, nil
		}
	}
	return storedProfile{}, errors.New("perfil de IA não encontrado")
}

// ListAIProfiles lista os perfis de IA (sem chaves).
func (a *App) ListAIProfiles() ([]AIProfileDTO, error) {
	ps, err := a.loadProfiles()
	if err != nil {
		return nil, err
	}
	out := make([]AIProfileDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, a.profileDTO(p))
	}
	return out, nil
}

func validateProfile(p AIProfileDTO) (storedProfile, error) {
	sp := storedProfile{ID: p.ID, Name: strings.TrimSpace(p.Name), Kind: p.Kind,
		BaseURL: strings.TrimSpace(p.BaseURL), Model: strings.TrimSpace(p.Model)}
	switch ai.Kind(sp.Kind) {
	case ai.KindOpenAICompat, ai.KindAnthropic:
	default:
		return sp, errors.New("tipo de provedor inválido")
	}
	if sp.Name == "" || sp.Model == "" {
		return sp, errors.New("nome e modelo são obrigatórios")
	}
	if len(sp.Name) > maxProfileText || len(sp.BaseURL) > maxProfileText || len(sp.Model) > maxProfileText {
		return sp, errors.New("texto muito longo")
	}
	return sp, nil
}

// SaveAIProfile cria (ID vazio) ou atualiza um perfil. apiKey vazia mantém a
// chave atual; caso contrário vai para o Credential Manager.
func (a *App) SaveAIProfile(p AIProfileDTO, apiKey string) (AIProfileDTO, error) {
	sp, err := validateProfile(p)
	if err != nil {
		return AIProfileDTO{}, err
	}
	a.profMu.Lock()
	defer a.profMu.Unlock()
	ps, err := a.loadProfiles()
	if err != nil {
		return AIProfileDTO{}, err
	}
	idx := -1
	if sp.ID == "" {
		if sp.ID, err = newProfileID(); err != nil {
			return AIProfileDTO{}, err
		}
	} else {
		for i := range ps {
			if ps[i].ID == sp.ID {
				idx = i
			}
		}
		if idx < 0 {
			return AIProfileDTO{}, errors.New("perfil de IA não encontrado")
		}
	}
	if apiKey != "" {
		if a.deps.SecretSet == nil {
			return AIProfileDTO{}, errNotSupported
		}
		if err := a.deps.SecretSet(sp.ID, apiKey); err != nil {
			return AIProfileDTO{}, errors.New("não foi possível guardar a chave de API")
		}
	}
	if idx >= 0 {
		ps[idx] = sp
	} else {
		ps = append(ps, sp)
	}
	if err := a.storeProfiles(ps); err != nil {
		if idx < 0 && apiKey != "" && a.deps.SecretDelete != nil {
			_ = a.deps.SecretDelete(sp.ID)
		}
		return AIProfileDTO{}, err
	}
	return a.profileDTO(sp), nil
}

// DeleteAIProfile remove o perfil e a sua chave.
func (a *App) DeleteAIProfile(id string) error {
	a.profMu.Lock()
	defer a.profMu.Unlock()
	ps, err := a.loadProfiles()
	if err != nil {
		return err
	}
	out := ps[:0:0]
	for _, p := range ps {
		if p.ID != id {
			out = append(out, p)
		}
	}
	if len(out) == len(ps) {
		return errors.New("perfil de IA não encontrado")
	}
	if err := a.storeProfiles(out); err != nil {
		return err
	}
	if a.deps.SecretDelete != nil {
		_ = a.deps.SecretDelete(id) // a missing key is fine
	}
	return nil
}

// friendlyAIError maps provider errors to user-facing PT-BR messages.
func friendlyAIError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ai.ErrAuth):
		return errors.New("Chave de API inválida")
	case errors.Is(err, ai.ErrModelNotFound):
		return errors.New("Modelo não encontrado")
	case errors.Is(err, ai.ErrRateLimited):
		return errors.New("Limite de requisições atingido")
	case errors.Is(err, ai.ErrTimeout):
		return errors.New("Tempo esgotado")
	case errors.Is(err, ai.ErrNetwork):
		return errors.New("Falha de rede")
	case errors.Is(err, ai.ErrInvalidResponse):
		return errors.New("Resposta inválida do provedor")
	case errors.Is(err, context.Canceled):
		return errors.New("Análise cancelada")
	}
	return err
}

func (a *App) newProvider(p storedProfile) (ai.Provider, error) {
	if a.deps.NewProvider == nil {
		return nil, errNotSupported
	}
	key := ""
	if a.deps.SecretGet != nil {
		key, _ = a.deps.SecretGet(p.ID) // no key is valid for local endpoints
	}
	return a.deps.NewProvider(p.profile(), key)
}

// TestAIProfile envia um payload mínimo ao provedor e devolve um erro amigável.
func (a *App) TestAIProfile(id string) error {
	p, err := a.findProfile(id)
	if err != nil {
		return err
	}
	prov, err := a.newProvider(p)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.baseCtx(), testAITimeout)
	defer cancel()
	tiny := ai.Payload{Classified: []ai.Item{{
		Path: `C:\example`, Size: 1 << 20, Files: 1, RuleID: "example", Verdict: rules.Safe,
	}}}
	_, err = prov.Analyze(ctx, tiny)
	return friendlyAIError(err)
}

// aiBuild is everything RunAI and PreviewAIPayload derive from the sessions.
type aiBuild struct {
	payload ai.Payload
	json    []byte
	pm      ai.PathMap
}

// buildAI is the single place that builds the payload sent to a provider.
func (a *App) buildAI() (aiBuild, error) {
	sessions := a.allSessions()
	if len(sessions) == 0 {
		return aiBuild{}, errors.New("nenhum volume varrido")
	}
	vols := map[string]volumes.Volume{}
	if a.deps.Volumes != nil {
		if vs, err := a.deps.Volumes(); err == nil {
			for _, v := range vs {
				vols[volKey(v.Root)] = v
			}
		}
	}
	scans := make([]ai.ScanData, 0, len(sessions))
	trees := make([]*tree.Tree, 0, len(sessions))
	for _, s := range sessions {
		trees = append(trees, s.tree)
		v, ok := vols[volKey(s.root)]
		if !ok {
			v = volumes.Volume{Root: s.root}
		}
		scans = append(scans, ai.ScanData{Volume: v, Tree: s.tree, Classes: s.classes})
	}
	st := a.loadSettings()
	// Every user name we can know of: USERNAME, the USERPROFILE leaf (may
	// differ, e.g. "jdoe.CORP") and each profile folder under X:\Users.
	names := append([]string{os.Getenv("USERNAME"), pathLeaf(os.Getenv("USERPROFILE"))},
		ai.UserNamesFromTrees(trees...)...)
	anon := ai.NewUserAnonymizer(names, st.RedactTerms)
	payload, pm := ai.BuildPayload(scans, anon, ai.PayloadOptions{MinUnknownBytes: st.AIMinBytes})
	b, err := payload.Compact()
	if err != nil {
		return aiBuild{}, err
	}
	return aiBuild{payload: payload, json: b, pm: pm}, nil
}

// pathLeaf is the last component of a Windows path ("" for "").
func pathLeaf(p string) string {
	p = strings.TrimRight(strings.ReplaceAll(p, "/", `\`), `\`)
	if i := strings.LastIndex(p, `\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

func (a *App) cachedAnalysis(key string) (ai.Analysis, bool) {
	if a.deps.Store == nil {
		return ai.Analysis{}, false
	}
	b, ok, err := a.deps.Store.GetAIReport(context.Background(), key)
	if err != nil || !ok {
		return ai.Analysis{}, false
	}
	var an ai.Analysis
	if json.Unmarshal(b, &an) != nil {
		return ai.Analysis{}, false
	}
	return an, true
}

// PreviewAIPayload devolve exatamente o que RunAI enviaria.
func (a *App) PreviewAIPayload(profileID string) (PayloadPreviewDTO, error) {
	p, err := a.findProfile(profileID)
	if err != nil {
		return PayloadPreviewDTO{}, err
	}
	b, err := a.buildAI()
	if err != nil {
		return PayloadPreviewDTO{}, err
	}
	_, cached := a.cachedAnalysis(ai.CacheKey(b.json, ai.Kind(p.Kind), p.Model))
	return PayloadPreviewDTO{JSON: string(b.json), Tokens: ai.EstimateTokens(b.json),
		Provider: p.Kind, Model: p.Model, Cached: cached}, nil
}

// RunAI analisa as varreduras com o perfil; assíncrono → evento ai:done.
func (a *App) RunAI(profileID string, force bool) error {
	p, err := a.findProfile(profileID)
	if err != nil {
		return err
	}
	prov, err := a.newProvider(p)
	if err != nil {
		return err
	}
	b, err := a.buildAI()
	if err != nil {
		return err
	}
	a.aiMu.Lock()
	if a.aiRunning {
		a.aiMu.Unlock()
		return errors.New("já existe uma análise em andamento")
	}
	ctx, cancel := context.WithCancel(a.baseCtx())
	a.aiRunning, a.aiCancel = true, cancel
	a.aiWG.Add(1)
	a.aiMu.Unlock()

	go func() {
		defer func() {
			cancel()
			a.aiMu.Lock()
			a.aiRunning, a.aiCancel = false, nil
			a.aiMu.Unlock()
			a.aiWG.Done()
		}()
		rep, err := a.analyze(ctx, p, prov, b, force)
		if err != nil {
			a.emit(EvAIDone, AIDoneDTO{Error: friendlyAIError(err).Error()})
			return
		}
		a.emit(EvAIDone, AIDoneDTO{Report: rep})
	}()
	return nil
}

func (a *App) analyze(ctx context.Context, p storedProfile, prov ai.Provider, b aiBuild, force bool) (*AIReportDTO, error) {
	key := ai.CacheKey(b.json, ai.Kind(p.Kind), p.Model)
	an, hit := ai.Analysis{}, false
	if !force {
		an, hit = a.cachedAnalysis(key)
	}
	if !hit {
		var err error
		if an, err = prov.Analyze(ctx, b.payload); err != nil {
			return nil, err
		}
		if a.deps.Store != nil {
			if raw, err := json.Marshal(an); err == nil {
				_ = a.deps.Store.PutAIReport(context.Background(), key, p.Kind, p.Model, raw)
			}
		}
	}
	res := ai.Resolve(an, b.pm)
	a.applyOpinions(res)
	rep := &AIReportDTO{Markdown: an.ReportMarkdown, Items: make([]AIItemDTO, 0, len(res)), CreatedAt: a.deps.Now()}
	for _, r := range res {
		rep.Items = append(rep.Items, AIItemDTO{Path: r.RealPath, Verdict: string(r.Verdict), Reason: r.Reason, Confidence: r.Confidence})
	}
	a.aiMu.Lock()
	a.lastReport = rep
	a.aiMu.Unlock()
	return rep, nil
}

// applyOpinions replaces the AI opinions of every session (copy-on-write).
func (a *App) applyOpinions(res []ai.Resolved) {
	for _, k := range a.sessionKeys() {
		a.updateSession(k, func(old *session) *session {
			m := map[tree.NodeID]ai.Resolved{}
			for _, r := range res {
				if id, ok := old.tree.Find(r.RealPath); ok && !old.tree.IsRemoved(id) {
					m[id] = r
				}
			}
			n := *old
			n.aiOpinions = m
			return &n
		})
	}
}

// CancelAI cancela a análise em andamento.
func (a *App) CancelAI() {
	a.aiMu.Lock()
	defer a.aiMu.Unlock()
	if a.aiCancel != nil {
		a.aiCancel()
	}
}

// waitAI bloqueia até a análise terminar (usado em testes).
func (a *App) waitAI() { a.aiWG.Wait() }

// GetAIReport devolve o último relatório da sessão (nil se ainda não houve).
func (a *App) GetAIReport() (*AIReportDTO, error) {
	a.aiMu.Lock()
	defer a.aiMu.Unlock()
	return a.lastReport, nil
}
