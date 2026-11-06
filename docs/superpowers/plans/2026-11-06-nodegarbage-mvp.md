# NodeGarbage v0.1 (MVP) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Entregar o NodeGarbage v0.1: app desktop Windows que varre discos, classifica o que ocupa espaço via regras locais (e opcionalmente IA), e apaga somente o que o usuário confirmar.

**Architecture:** Backend Go em pacotes `internal/*` independentes e testáveis sem GUI (volumes → scanner → tree → rules → ai → cleaner → store), unidos por uma fachada `internal/app` exposta ao frontend via bindings e eventos do Wails v2. Frontend Svelte 5 + TypeScript + Tailwind v4 com glassmorphism sobre Mica.

**Tech Stack:** Go 1.27, Wails v2, `golang.org/x/sys/windows`, `modernc.org/sqlite`, `gopkg.in/yaml.v3`, `github.com/danieljoos/wincred`, `github.com/go-ole/go-ole`; Svelte 5, TypeScript, Vite, Tailwind v4, Vitest, @testing-library/svelte, d3-hierarchy, marked, DOMPurify.

**Spec:** `docs/superpowers/specs/2026-11-06-nodegarbage-design.md`

## Global Constraints

- Módulo Go: `github.com/engviniciusbs/nodegarbage` (confirmar com o usuário antes da Task 1).
- Só Windows; arquivos com Win32 levam `//go:build windows`. Sem CGO (`CGO_ENABLED=0` deve compilar).
- Toda operação longa recebe `context.Context` e é cancelável.
- **Só o usuário apaga.** Nenhum caminho de código, exceto `cleaner.Planner.Redeem` → `cleaner.Execute` disparado por `App.ExecuteClean(planID, token)`, pode apagar arquivos. A resposta da IA é dado não confiável.
- Caminhos de sistema de arquivos usam prefixo `\\?\` ao acessar o disco; comparações de caminho são case-insensitive e com separador `\`.
- Dados locais em `%LOCALAPPDATA%\NodeGarbage\` (`nodegarbage.db`, `rules.d\`).
- Chaves de API só no Windows Credential Manager (alvo `NodeGarbage/ai/<profileID>`); nunca em arquivo, SQLite ou log.
- Textos de UI em PT-BR, todos em `frontend/src/lib/strings.ts`. Componentes nunca usam hex direto, só tokens CSS.
- Componentes Svelte com menos de 200 linhas; toda tela tem estados vazio, carregando e erro.
- Eventos de progresso: no máximo 10 por segundo.
- Valores fixos da spec: top-N arquivos = 1000; até 100 caminhos negados guardados; workers padrão = `GOMAXPROCS × 4`; candidato à IA = `unknown` ≥ 500 MB; payload: 200 classificados, 150 desconhecidos, 30 extensões, 50 arquivos, orçamento 30.000 tokens (chars/4); IA: timeout 120 s, 2 retentativas; token de limpeza: 32 bytes, uso único, 5 minutos; projeto inativo = 90 dias.

## Review Focus

1. **Diretório some durante a varredura** (ex.: `%TEMP%` sendo limpo por outro processo): espera-se que seja ignorado em silêncio, sem contar como "acesso negado" e sem abortar. Teste na Task 3.
2. **Item mudou entre a varredura e a limpeza** (pasta já apagada, ou virou junction): espera-se recusa com motivo `not_found`/`protected` no plano, nunca erro genérico. Teste na Task 13.
3. **Caixa e forma do caminho diferentes** (`c:\windows\SYSTEM32`, barra `/`, barra final): espera-se a mesma proteção/regra que a forma canônica. Teste na Task 6.
4. **Modelo devolve JSON dentro de cerca de código** (```` ```json ... ``` ````) ou com texto antes: espera-se extração correta, não `ErrInvalidResponse`. Teste na Task 11.
5. **Disco removido/inacessível no início da varredura** (pendrive retirado): espera-se `ErrRootUnavailable` e estado de erro na UI, sem snapshot salvo. Teste na Task 3 (scanner) e Task 15 (app não salva).

---

## Fase A — Fundação

### Task 1: Scaffold do projeto Wails + higiene do repositório

**Files:**
- Create: `go.mod`, `main.go`, `wails.json`, `frontend/` (template), `.gitignore`, `.editorconfig`, `LICENSE`
- Create: `internal/app/app.go` (stub)

**Interfaces:**
- Produces: `app.New(deps app.Deps) *app.App`, `(*App).Startup(ctx context.Context)` (stub vazio por ora); `main.go` faz bind de `*app.App`.

- [ ] **Step 1: Instalar o Wails CLI e conferir o ambiente**

Run: `go install github.com/wailsapp/wails/v2/cmd/wails@latest && wails doctor`
Expected: `wails doctor` sem itens "Missing" obrigatórios (WebView2 e npm presentes).

- [ ] **Step 2: Gerar o template svelte-ts num diretório temporário e mover para a raiz**

Run: `wails init -n nodegarbage -t svelte-ts -d <scratch>/ng` e copiar o conteúdo para a raiz do repo (sem sobrescrever `docs/`). Ajustar `go.mod` para o módulo dos Global Constraints e `go 1.27`.

- [ ] **Step 3: Mover o `App` do template para `internal/app/app.go`**

`package app`; `type Deps struct{}` (vazio por ora); `func New(d Deps) *App`; `func (a *App) Startup(ctx context.Context)` guarda o ctx. `main.go` usa `OnStartup: a.Startup`, `Bind: []any{a}`, `MinWidth: 1024`, `MinHeight: 680`, título "NodeGarbage". Remover o `Greet` do template.

- [ ] **Step 4: Higiene do repo**

`.gitignore` com `build/bin/`, `frontend/node_modules/`, `frontend/dist/`, `frontend/wailsjs/` (gerado). `LICENSE` MIT (ano 2026, titular do repo). `.editorconfig` (utf-8, LF, indent: tabs em `.go`, 2 espaços no resto).

- [ ] **Step 5: Verificar build**

Run: `wails build -clean`
Expected: gera `build/bin/nodegarbage.exe`; ao abrir mostra a janela do template.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "chore: scaffold Wails + Svelte TS project"
```

---

## Fase B — Núcleo do backend

### Task 2: `tree` — árvore agregada, top-N e histograma

**Files:**
- Create: `internal/tree/tree.go`, `internal/tree/builder.go`, `internal/tree/topn.go`
- Test: `internal/tree/tree_test.go`

**Interfaces:**
- Produces:
```go
type NodeID int32
const NoNode NodeID = -1
type Flags uint8
const (FlagLink Flags = 1 << iota; FlagCloud; FlagDenied)
type Node struct {
    Name string; Parent NodeID; Children []NodeID
    OwnSize, TotalSize int64       // bytes
    OwnFiles, TotalFiles uint32
    OwnLatest, TreeLatest int64    // unix seconds; TreeLatest = max na sub-árvore
    Flags Flags
}
type FileEntry struct { Name string; Size int64; ModTime int64 }
type FileInfo struct { Path string; Size int64; ModTime int64 }
type ExtStat struct { Ext string; Count uint64; Bytes int64 } // Ext minúscula com ponto; "" = sem extensão
type Tree struct {
    Root NodeID; Nodes []Node
    TopFiles []FileInfo     // desc por Size, len ≤ topN
    Extensions []ExtStat    // desc por Bytes
    Denied []string; DeniedCount int
}
func (t *Tree) Path(id NodeID) string                 // caminho absoluto com "\"
func (t *Tree) SortedChildren(id NodeID) []NodeID     // desc por TotalSize
func (t *Tree) Find(path string) (NodeID, bool)       // case-insensitive
type Builder struct{ /* ... */ }
func NewBuilder(root string, topN int) *Builder
func (b *Builder) AddDir(parent NodeID, name string, flags Flags) NodeID // thread-safe; raiz: AddDir(NoNode, root, 0)
func (b *Builder) SetOwnStats(id NodeID, size int64, files uint32, latest int64) // chamado uma vez por dir, pelo worker dono
func (b *Builder) AddDenied(path string)              // guarda até 100, conta todos
func (b *Builder) NewWorker() *Worker                 // acumulador por goroutine (top-N + extensões)
func (w *Worker) AddFile(dirPath string, f FileEntry)
func (b *Builder) Finish() *Tree                      // mescla workers + passada pós-ordem
```

- [ ] **Step 1: Escrever os testes**

`TestFinishAggregatesPostOrder`: raiz `C:\` → `a` (own 100 B, 2 arquivos, latest 10) → `a\b` (own 50 B, 1 arquivo, latest 30). Após `Finish`: `a.TotalSize == 150`, `a.TotalFiles == 3`, `a.TreeLatest == 30`, raiz `TotalSize == 150`.
`TestTopFilesMergedAcrossWorkers`: 2 workers, `topN = 3`, arquivos de tamanhos 5,1,9 e 7,3 → `TopFiles` tamanhos `[9,7,5]` e `Path` = `dirPath + "\" + Name`.
`TestExtensionsCaseInsensitive`: `A.LOG` (10) e `b.log` (5) e `Makefile` (1) → `{".log",2,15}` primeiro e `{"",1,1}`.
`TestDeniedCappedAt100`: 150 chamadas → `len(Denied) == 100`, `DeniedCount == 150`.
`TestPathAndFind`: `Path` de `a\b` == `C:\a\b`; `Find("c:\A\B")` acha o mesmo id.
`TestAddDirConcurrent`: 50 goroutines × 100 `AddDir` sob `-race` → 5001 nós, sem race.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/tree/ -race`
Expected: FAIL (tipos não definidos).

- [ ] **Step 3: Implementar**

Arena `[]Node` protegida por `sync.Mutex` só em `AddDir`; `SetOwnStats` escreve campos do nó sem lock (cada dir tem um único worker dono; `Finish` roda após todos terminarem). Top-N por worker com `container/heap` (min-heap) e mescla no `Finish`. Pós-ordem iterativa (pilha) para não estourar a stack em árvores profundas.

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/tree/ -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tree && git commit -m "feat(tree): aggregated directory tree with top-N and extension stats"
```

### Task 3: `scanner` — WalkScanner concorrente

**Files:**
- Create: `internal/scanner/scanner.go`, `internal/scanner/fs.go`, `internal/scanner/fs_windows.go`, `internal/scanner/walk.go`
- Test: `internal/scanner/walk_test.go`, `internal/scanner/walk_windows_test.go`, `internal/scanner/bench_test.go`

**Interfaces:**
- Consumes: `tree.Builder`, `tree.FileEntry`, `tree.Flags` (Task 2).
- Produces:
```go
var ErrRootUnavailable = errors.New("scanner: root unavailable")
type DirEntry struct { Name string; IsDir bool; Size int64; ModTime int64; Attrs uint32 }
type FS interface { ReadDir(path string) ([]DirEntry, error) }
func OSFS() FS
type Progress struct { /* atomics */ }
type ProgressSnapshot struct { Files, Dirs, Bytes, Denied int64; Current string }
func (p *Progress) Snapshot() ProgressSnapshot
type Scanner interface { Scan(ctx context.Context, root string, b *tree.Builder, p *Progress) error }
type WalkScanner struct { FS FS; Workers int } // Workers 0 → GOMAXPROCS*4
const EngineWalk = "walk"
```
Atributos usados (de `golang.org/x/sys/windows`): `FILE_ATTRIBUTE_REPARSE_POINT` → `FlagLink` (não descer, tamanho 0); `FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS` (0x400000), `FILE_ATTRIBUTE_RECALL_ON_OPEN` (0x40000), `FILE_ATTRIBUTE_OFFLINE` → arquivo conta 0 bytes; dir com esses atributos recebe `FlagCloud`.

- [ ] **Step 1: Escrever testes com FS falso (`walk_test.go`)**

`fakeFS` em memória (mapa caminho → entradas, mapa caminho → erro).
`TestWalkSumsTree`: 3 níveis, 6 arquivos → `TotalSize` e `TotalFiles` da raiz batem com a soma.
`TestReparsePointNotFollowed`: dir com `Attrs` de reparse → nó com `FlagLink`, `ReadDir` nunca chamado para ele (o fake falha o teste se for).
`TestCloudFilesCountZero`: arquivo 1 GB com `RECALL_ON_DATA_ACCESS` → `TotalSize` 0 para ele.
`TestAccessDeniedContinues`: `ReadDir` de `C:\x` retorna `windows.ERROR_ACCESS_DENIED` → nó `FlagDenied`, `DeniedCount == 1`, irmãos varridos, `Scan` retorna nil.
`TestVanishedDirIgnored` (Review Focus 1): `ReadDir` retorna `ERROR_PATH_NOT_FOUND`/`fs.ErrNotExist` → nenhum erro, `DeniedCount == 0`.
`TestRootUnavailable` (Review Focus 5): `ReadDir` da raiz falha → `errors.Is(err, ErrRootUnavailable)`.
`TestCancel`: ctx cancelado após o primeiro `ReadDir` → retorna `context.Canceled`, sem goroutines vazando (contar via `runtime.NumGoroutine` com tolerância).
`TestProgressCounts`: `Snapshot()` final com `Files`, `Dirs`, `Bytes` corretos.

- [ ] **Step 2: Escrever testes reais no Windows (`walk_windows_test.go`)**

`TestOSFSJunctionAndLongPath`: em `t.TempDir()` criar `deep\...` com caminho > 260 caracteres contendo um arquivo de 10 B, e um junction `j` → `deep` via `exec.Command("cmd", "/c", "mklink", "/J", ...)`. Varre com `OSFS()`: arquivo longo contado uma vez, `j` com `FlagLink`.

- [ ] **Step 3: Rodar e ver falhar**

Run: `go test ./internal/scanner/ -race`
Expected: FAIL.

- [ ] **Step 4: Implementar**

`OSFS.ReadDir` usa `os.ReadDir` com prefixo `\\?\` e `DirEntry.Info()` (cacheado no Windows); atributos via `Info().Sys().(*syscall.Win32FileAttributeData)`. `WalkScanner`: fila de diretórios + contador de pendentes (`sync.WaitGroup` ou contador atômico) com N workers; cada worker tem um `tree.Worker`; `Progress.Current` atualizado a cada diretório.

- [ ] **Step 5: Benchmark**

`BenchmarkWalkSynthetic`: `fakeFS` com ~500 mil arquivos em ~20 mil diretórios. Run: `go test ./internal/scanner/ -bench Walk -benchmem -run ^$` e registrar o resultado na mensagem do commit.

- [ ] **Step 6: Rodar e ver passar**

Run: `go test ./internal/scanner/ -race`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/scanner && git commit -m "feat(scanner): concurrent WalkScanner with reparse/cloud/denied handling"
```

### Task 4: `volumes` e `elevation`

**Files:**
- Create: `internal/volumes/volumes_windows.go`, `internal/elevation/elevation_windows.go`
- Test: `internal/volumes/volumes_windows_test.go`, `internal/elevation/elevation_windows_test.go`

**Interfaces:**
- Produces:
```go
// volumes
type DriveType string // "fixed" | "removable" | "network" | "cdrom" | "ram" | "unknown"
type Volume struct { Root string /* "C:\" */; Label, FS string; Type DriveType; Total, Free uint64; HasRecycleBin bool }
func List() ([]Volume, error)  // HasRecycleBin = Type == "fixed"
// elevation
func IsElevated() bool
func RestartElevated(args []string) error // ShellExecute "runas" em os.Executable(); o chamador encerra o app
```

- [ ] **Step 1: Testes**

`TestListIncludesSystemDrive`: `List()` contém o volume de `os.Getenv("SystemDrive") + "\"` com `Type == "fixed"`, `Total > 0`, `Free <= Total`, `FS != ""`.
`TestIsElevatedDoesNotPanic`: chama e loga o valor (o resultado depende do ambiente).

- [ ] **Step 2: Rodar e ver falhar** — Run: `go test ./internal/volumes/ ./internal/elevation/` → FAIL.

- [ ] **Step 3: Implementar**

`GetLogicalDrives`, `GetDriveType`, `GetVolumeInformation`, `GetDiskFreeSpaceEx` (x/sys/windows). Volumes sem mídia (CD vazio) são omitidos sem erro. `IsElevated` via `windows.GetCurrentProcessToken().IsElevated()`. `RestartElevated` via `windows.ShellExecute(0, "runas", exe, args, cwd, SW_NORMAL)`.

- [ ] **Step 4: Rodar e ver passar** — mesmo comando → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/volumes internal/elevation && git commit -m "feat: volume enumeration and elevation helpers"
```

### Task 5: `rules` — schema, carregamento e validação

**Files:**
- Create: `internal/rules/types.go`, `internal/rules/load.go`, `internal/rules/builtin/` (vazio exceto `README.md` por ora)
- Test: `internal/rules/load_test.go`

**Interfaces:**
- Produces:
```go
type Verdict string
const (Safe Verdict = "safe"; Caution Verdict = "caution"; Keep Verdict = "keep")
func MostConservative(a, b Verdict) Verdict // keep > caution > safe
type Category string // dev-cache | build-artifact | pkg-cache | temp | browser-cache | system | user-data
type Action string
const (ActionDeleteDir Action = "delete_dir"; ActionDeleteContents Action = "delete_contents"; ActionManual Action = "manual")
type Source string
const (SourceProtected Source = "protected"; SourceRule Source = "rule"; SourceAI Source = "ai"; SourceUnknown Source = "unknown")
type Match struct {
    Path string `yaml:"path"`; Dirname string `yaml:"dirname"`; Sibling string `yaml:"sibling"`
    Glob string `yaml:"glob"`; OlderThanDays int `yaml:"older_than_days"`
}
type Rule struct {
    ID string `yaml:"id"`; Name string `yaml:"name"`; Category Category `yaml:"category"`; Match Match `yaml:"match"`
    Verdict Verdict `yaml:"verdict"`; Regenerable string `yaml:"regenerable"`; Action Action `yaml:"action"`
    Command string `yaml:"command"`; Explain string `yaml:"explain"`; Builtin bool `yaml:"-"`
}
type LoadProblem struct { File, RuleID, Err string }
func Validate(r Rule) error
func LoadBuiltin() ([]Rule, error)                   // embed builtin/*.yaml; qualquer inválida → erro
func LoadUserDir(dir string) ([]Rule, []LoadProblem) // inválidas são puladas e relatadas; dir inexistente → nada
```
`Sibling` aceita glob simples (ex.: `*.csproj`), avaliado com `filepath.Match` contra os nomes do diretório pai.

- [ ] **Step 1: Testes**

`TestValidate` (tabela): sem `id` → erro; zero ou mais de um entre `path|dirname|glob` → erro; `verdict` fora do enum → erro; `action: manual` sem `command` → erro; `sibling` sem `dirname` → erro; regra válida → nil.
`TestLoadUserDirSkipsInvalid`: dir temporário com `ok.yaml` (1 regra válida) e `bad.yaml` (uma inválida + YAML quebrado em outro arquivo) → 1 regra, 2 problemas com `File` preenchido.
`TestLoadUserDirDuplicateID`: duas regras com mesmo `id` → só a primeira, 1 problema.
`TestMostConservative`: `(safe,caution)→caution`, `(keep,safe)→keep`.

- [ ] **Step 2–4: falhar → implementar (`yaml.v3`, `//go:embed builtin/*.yaml`) → passar**

Run: `go test ./internal/rules/`

- [ ] **Step 5: Commit**

```bash
git add internal/rules && git commit -m "feat(rules): rule schema, loading and validation"
```

### Task 6: `rules` — proteção e classificador

**Files:**
- Create: `internal/rules/protect.go`, `internal/rules/classify.go`, `internal/rules/pathnorm.go`
- Test: `internal/rules/protect_test.go`, `internal/rules/classify_test.go`

**Interfaces:**
- Consumes: `tree.Tree`, `tree.NodeID` (Task 2); `Rule` etc. (Task 5).
- Produces:
```go
type Env func(key string) string // os.Getenv em produção
func NormalizePath(p string) string // `/`→`\`, sem barra final (exceto raiz "C:\"), sem prefixo \\?\, minúsculas
type ProtectKind int
const (ProtectNone ProtectKind = iota; ProtectExact; ProtectSubtree)
type Protector struct{ /* ... */ }
func NewProtector(env Env, volumeRoots []string, extra []string) *Protector
func (p *Protector) Check(path string) ProtectKind       // ProtectSubtree também quando path está DENTRO de uma sub-árvore protegida
func (p *Protector) HasProtectedDescendant(path string) bool // algum caminho protegido (exato ou sub-árvore, exceto .git) fica abaixo de path
type Classification struct {
    RuleID string; Verdict Verdict; Category Category; Action Action
    Inactive bool; Source Source
}
type Classifications struct{ /* map[tree.NodeID]Classification + herança */ }
func (c *Classifications) Get(t *tree.Tree, id tree.NodeID) Classification // herda de ancestral protegido-subárvore ou delete_dir; senão SourceUnknown
type Classifier struct{ /* ... */ }
func NewClassifier(rs []Rule, p *Protector, env Env, exists func(path string) bool, listNames func(dir string) []string) *Classifier
func (c *Classifier) Classify(t *tree.Tree, now time.Time) *Classifications
func (c *Classifier) ClassifyFile(path string, modTime time.Time, now time.Time) (Classification, bool) // regras glob
func Candidates(t *tree.Tree, cls *Classifications, minBytes int64) []tree.NodeID // unknown, ≥ minBytes, sem ancestral classificado; desc por tamanho
func (c *Classifier) Rule(id string) (Rule, bool)
```
Listas fixas (spec §4): sub-árvore `%SystemRoot%`, `%ProgramFiles%`, `%ProgramFiles(x86)%`, `%ProgramData%\Microsoft`, `<raiz>\System Volume Information`, `<raiz>\$Recycle.Bin`, qualquer componente `.git`; exceção `%SystemRoot%\Temp`. Exatas: raízes, `<raiz>\pagefile.sys|hiberfil.sys|swapfile.sys`, `%USERPROFILE%` e `%USERPROFILE%\{Documents,Desktop,Pictures,Videos,Music,Downloads}`.

- [ ] **Step 1: Testes de proteção (`protect_test.go`)**, com `env` falso (`SystemRoot=C:\Windows`, `USERPROFILE=C:\Users\dev`, …)

`TestProtectCaseAndForm` (Review Focus 3): `c:\windows\SYSTEM32`, `C:/Windows/System32/`, `\\?\C:\Windows` → todos `ProtectSubtree`.
`TestWindowsTempException`: `C:\Windows\Temp` e `C:\Windows\Temp\x` → `ProtectNone`; `C:\Windows\Temp2` → `ProtectSubtree`.
`TestExactOnly`: `C:\Users\dev\Documents` → `ProtectExact`; `C:\Users\dev\Documents\proj\node_modules` → `ProtectNone`.
`TestGitSubtree`: `D:\src\app\.git\objects` → `ProtectSubtree`; `D:\src\app\.github` → `ProtectNone`.
`TestHasProtectedDescendant`: `C:\Users\dev` → true (Documents); `C:\Users\dev\Documents\proj` → false.
`TestExtraPaths`: extra `D:\vault` → `D:\vault\a` `ProtectSubtree`.

- [ ] **Step 2: Testes de classificação (`classify_test.go`)**, com árvore montada via `tree.Builder`

`TestNodeModulesNeedsSibling`: `proj\node_modules` com `proj\package.json` → regra `node.node_modules`; sem `package.json` → `SourceUnknown`.
`TestPathRuleBeatsDirname`: regra `path: %LOCALAPPDATA%\x\cache` e regra `dirname: cache` → vence a de `path`.
`TestSiblingMoreSpecific`: duas regras `dirname: target`, uma com `sibling: Cargo.toml` → vence a com sibling.
`TestConsumedSubtree`: `node_modules\a\node_modules` → só o externo no mapa; `Get` do interno retorna a classificação do externo.
`TestProtectedOverridesRule`: regra `path: %SystemRoot%\System32` com `safe` → `Get` retorna `Keep`, `SourceProtected`, e **mantém `RuleID`** da regra casada (para a UI exibir `command`/`explain` de regras manuais em áreas protegidas, ex.: `windows.update_download`).
`TestInactiveProject`: `build-artifact` com `TreeLatest` do pai (excluindo o artefato) há 91 dias → `Inactive true`; 89 dias → false.
`TestCandidates`: unknown 600 MB e 400 MB, e unknown 700 MB dentro de `node_modules` → só o de 600 MB.
`TestClassifyFileGlob`: `glob: "*.dmp"`, `older_than_days: 7` → arquivo de 8 dias casa; de 6 dias não.

- [ ] **Step 3–5: falhar → implementar → passar**

Run: `go test ./internal/rules/ -race`. Inatividade: `max(pai.OwnLatest, TreeLatest dos irmãos)` comparado a `now - 90 dias`.

- [ ] **Step 6: Commit**

```bash
git add internal/rules && git commit -m "feat(rules): protection list and tree classifier"
```

### Task 7: Regras embutidas do MVP

**Files:**
- Create: `internal/rules/builtin/{node,go,rust,dotnet,python,jvm,android,containers,windows,browsers,editors}.yaml`
- Test: `internal/rules/builtin_test.go`

**Interfaces:**
- Consumes: `LoadBuiltin`, `Classifier` (Tasks 5–6).

Conteúdo mínimo (ids exatos; `explain` em PT-BR, 1–2 frases):

| id | match | verdict | action |
|---|---|---|---|
| `node.node_modules` | dirname `node_modules`, sibling `package.json` | safe | delete_dir |
| `node.npm_cache` | path `%LOCALAPPDATA%\npm-cache` | safe | delete_dir |
| `node.pnpm_store` | path `%LOCALAPPDATA%\pnpm\store` | safe | manual `pnpm store prune` |
| `node.yarn_cache` | path `%LOCALAPPDATA%\Yarn\Cache` | safe | delete_dir |
| `node.bun_cache` | path `%USERPROFILE%\.bun\install\cache` | safe | delete_dir |
| `go.build_cache` | path `%LOCALAPPDATA%\go-build` | safe | manual `go clean -cache` |
| `go.mod_cache` | path `%USERPROFILE%\go\pkg\mod` | safe | manual `go clean -modcache` |
| `rust.target` | dirname `target`, sibling `Cargo.toml` | safe | delete_dir |
| `rust.registry` | path `%USERPROFILE%\.cargo\registry` | safe | delete_dir |
| `dotnet.bin` / `dotnet.obj` | dirname `bin` / `obj`, sibling `*.csproj` | safe | delete_dir |
| `dotnet.nuget` | path `%USERPROFILE%\.nuget\packages` | safe | manual `dotnet nuget locals all --clear` |
| `python.pycache` | dirname `__pycache__` | safe | delete_dir |
| `python.venv` | dirname `.venv`, sibling `pyproject.toml` | caution | delete_dir |
| `python.pip_cache` | path `%LOCALAPPDATA%\pip\Cache` | safe | delete_dir |
| `python.uv_cache` | path `%LOCALAPPDATA%\uv\cache` | safe | manual `uv cache clean` |
| `jvm.gradle_caches` | path `%USERPROFILE%\.gradle\caches` | safe | delete_dir |
| `jvm.gradle_build` | dirname `build`, sibling `build.gradle*` | safe | delete_dir |
| `jvm.maven_repo` | path `%USERPROFILE%\.m2\repository` | caution | delete_dir |
| `android.avd` | path `%USERPROFILE%\.android\avd` | caution | manual "Excluir AVDs pelo Android Studio (Device Manager)" |
| `containers.docker_vhdx` | path `%LOCALAPPDATA%\Docker\wsl` | caution | manual `docker system prune -a` |
| `containers.wsl_vhdx` | glob `ext4.vhdx` | caution | manual `wsl --manage <distro> --set-sparse true` |
| `windows.user_temp` | path `%TEMP%` | safe | delete_contents |
| `windows.system_temp` | path `%SystemRoot%\Temp` | safe | delete_contents |
| `windows.crashdumps` | path `%LOCALAPPDATA%\CrashDumps` | safe | delete_contents |
| `windows.update_download` | path `%SystemRoot%\SoftwareDistribution\Download` | caution | manual "Use Limpeza de Disco → Limpar arquivos do sistema" |
| `browsers.chrome_cache` / `edge_cache` | path `%LOCALAPPDATA%\Google\Chrome\User Data\Default\Cache` / `...\Microsoft\Edge\User Data\Default\Cache` | safe | delete_contents |
| `browsers.firefox_cache` | path `%LOCALAPPDATA%\Mozilla\Firefox\Profiles` | caution | manual "about:preferences#privacy → Limpar dados" |
| `editors.vscode_cache` | path `%APPDATA%\Code\Cache` e `CachedData` (2 regras) | safe | delete_contents |
| `editors.jetbrains_cache` | path `%LOCALAPPDATA%\JetBrains` | caution | delete_dir |

`windows.update_download` está dentro de `%SystemRoot%` e continuará `keep` pela proteção. Isso é intencional (o comando manual aparece no painel de detalhes via `Classifier.Rule(id)` quando o nó casa com um `path` de regra, mesmo protegido).

- [ ] **Step 1: Testes**

`TestBuiltinLoads`: `LoadBuiltin()` sem erro, ≥ 30 regras, ids únicos.
`TestBuiltinTable`: tabela caminho+irmãos → id esperado, um caso por linha da tabela acima (árvore sintética com `env` falso).
`TestBuiltinNeverUnprotects`: para cada regra, se o `path` expandido cai em `ProtectSubtree`, `Get` retorna `Keep`.

- [ ] **Step 2–4: falhar → escrever YAMLs → passar** — Run: `go test ./internal/rules/`

- [ ] **Step 5: Commit**

```bash
git add internal/rules && git commit -m "feat(rules): built-in rules for dev ecosystems, Windows and browsers"
```

### Task 8: `store` — SQLite

**Files:**
- Create: `internal/store/store.go`, `internal/store/migrations/001_init.sql`, `internal/store/snapshot.go`, `internal/store/cleanups.go`, `internal/store/kv.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `tree.Tree` (Task 2).
- Produces:
```go
func Open(path string) (*Store, error) // cria diretório, aplica migrações (tabela schema_version)
func (s *Store) Close() error
type Snapshot struct { Volume string; ScannedAt time.Time; Engine string; Duration time.Duration; Tree *tree.Tree }
func (s *Store) SaveSnapshot(ctx context.Context, sn Snapshot) error            // substitui o anterior do volume, numa transação
func (s *Store) LoadSnapshot(ctx context.Context, volume string) (*Snapshot, error) // nil, nil se não houver
func (s *Store) LatestScans(ctx context.Context) (map[string]time.Time, error)
type CleanupRecord struct { Path string; Size int64; Mode, Source, RuleID, Status, Reason string; Freed int64; At time.Time }
func (s *Store) AddCleanup(ctx context.Context, r CleanupRecord) error
func (s *Store) ListCleanups(ctx context.Context, limit int) ([]CleanupRecord, error) // desc por At
func (s *Store) TotalFreed(ctx context.Context) (int64, error)                     // soma Freed com Status "ok"
func (s *Store) PutAIReport(ctx context.Context, key, provider, model string, body []byte) error
func (s *Store) GetAIReport(ctx context.Context, key string) ([]byte, bool, error)
func (s *Store) GetSetting(ctx context.Context, key string) ([]byte, bool, error)
func (s *Store) SetSetting(ctx context.Context, key string, value []byte) error
```
Tabelas conforme spec §7. Driver: `modernc.org/sqlite` (`sql.Open("sqlite", path)`), `PRAGMA journal_mode=WAL`.

- [ ] **Step 1: Testes** (banco em `t.TempDir()`)

`TestSnapshotRoundTrip`: árvore de 3 níveis com top files, extensões e denied → `LoadSnapshot` devolve árvore com mesmos nós/tamanhos/flags/`Path`.
`TestSnapshotReplaces`: salvar dois para `C:\` → só o segundo existe (contar linhas de `snapshot_nodes`).
`TestLoadSnapshotMissing`: volume sem snapshot → `nil, nil`.
`TestCleanupsAndTotal`: 3 registros (ok 100, skipped 0, ok 50) → `TotalFreed == 150`, `ListCleanups` em ordem desc.
`TestAIReportCache` e `TestSettings`: put/get, ausente → `false`.
`TestReopenKeepsData`: fechar e reabrir mantém os dados; migração não reaplica.
`TestSave100kNodes`: snapshot com 100 mil nós salva em < 5 s (log do tempo; inserção com prepared statement numa transação).

- [ ] **Step 2–4: falhar → implementar → passar** — Run: `go test ./internal/store/ -race`

- [ ] **Step 5: Commit**

```bash
git add internal/store go.mod go.sum && git commit -m "feat(store): SQLite snapshots, cleanup history, AI cache and settings"
```

### Task 9: `secrets` — Credential Manager

**Files:**
- Create: `internal/secrets/secrets_windows.go`
- Test: `internal/secrets/secrets_windows_test.go`

**Interfaces:**
- Produces:
```go
var ErrNotFound = errors.New("secrets: not found")
func Set(profileID, apiKey string) error // alvo "NodeGarbage/ai/<profileID>", via github.com/danieljoos/wincred
func Get(profileID string) (string, error)
func Delete(profileID string) error      // ausente → nil
```

- [ ] **Step 1: Teste** `TestRoundTrip`: perfil `test-<random>`: `Set` → `Get` igual → `Delete` → `Get` retorna `ErrNotFound`. `t.Cleanup` apaga.
- [ ] **Step 2–4: falhar → implementar → passar** — Run: `go test ./internal/secrets/`
- [ ] **Step 5: Commit** — `git commit -m "feat(secrets): store API keys in Windows Credential Manager"`

---

## Fase C — IA

### Task 10: `ai` — payload, anonimização e orçamento

**Files:**
- Create: `internal/ai/payload.go`, `internal/ai/anonymize.go`
- Test: `internal/ai/payload_test.go`

**Interfaces:**
- Consumes: `tree.Tree`, `tree.ExtStat`, `tree.FileInfo`; `rules.Classifications`, `rules.Candidates`, `rules.Verdict`; `volumes.Volume`.
- Produces:
```go
type Anonymizer struct{ /* ... */ }
func NewAnonymizer(userName string, extra []string) *Anonymizer
func (a *Anonymizer) Apply(path string) string // C:\Users\<nome>\ → C:\Users\<user>\ (case-insensitive); termos extras → <redacted>
type Item struct { Path string; Size int64; Files uint32; ModTime int64; RuleID string; Verdict rules.Verdict; Exts []string }
type VolumeSummary struct { Root, FS string; Total, Free uint64 }
type Payload struct { Volumes []VolumeSummary; Classified, Unknown []Item; Extensions []tree.ExtStat; TopFiles []tree.FileInfo }
type PathMap map[string]string // anonimizado (normalizado) → real
type ScanData struct { Volume volumes.Volume; Tree *tree.Tree; Classes *rules.Classifications }
func BuildPayload(scans []ScanData, anon *Anonymizer, budgetTokens int) (Payload, PathMap)
func (p Payload) Compact() ([]byte, error) // listas como {"cols":[...],"rows":[[...]]}
func EstimateTokens(b []byte) int          // len(b)/4, arredondado para cima
```
Limites: 200 classificados (nós com `Source` rule/protected, desc por tamanho), 150 desconhecidos (`rules.Candidates` com 500 MB), 30 extensões, 50 arquivos. `Exts` do item desconhecido: as 5 maiores extensões entre os `TopFiles` dentro do nó (aproximação documentada).

- [ ] **Step 1: Testes**

`TestAnonymizeUser`: `C:\Users\Vinicius\src` → `C:\Users\<user>\src`; `c:\users\VINICIUS\x` também; `C:\Users\Public` intacto.
`TestAnonymizeExtra`: termo `acme` → `D:\clientes\<redacted>\app`.
`TestPayloadLimits`: árvore com 300 classificados e 200 desconhecidos → `len(Classified) == 200`, `len(Unknown) == 150`, ordenados desc.
`TestBudgetTrimsSmallestFirst`: orçamento de 2.000 tokens → `EstimateTokens(Compact()) ≤ 2000` e o maior item de cada lista permanece.
`TestPathMapRoundTrip`: todo `Path` do payload está no `PathMap` e mapeia para o caminho real.
`TestNoRealUserNameInPayload`: `Compact()` não contém o nome real (busca case-insensitive).

- [ ] **Step 2–4: falhar → implementar → passar** — Run: `go test ./internal/ai/`
- [ ] **Step 5: Commit** — `git commit -m "feat(ai): anonymized compact payload with token budget"`

### Task 11: `ai` — provedores, resposta e mescla

**Files:**
- Create: `internal/ai/provider.go`, `internal/ai/prompt.go`, `internal/ai/openaicompat.go`, `internal/ai/anthropic.go`, `internal/ai/response.go`, `internal/ai/merge.go`
- Test: `internal/ai/openaicompat_test.go`, `internal/ai/anthropic_test.go`, `internal/ai/response_test.go`, `internal/ai/merge_test.go`

**Interfaces:**
- Consumes: `Payload`, `PathMap` (Task 10); `rules.Classification`, `rules.MostConservative`.
- Produces:
```go
type Kind string
const (KindOpenAICompat Kind = "openai-compat"; KindAnthropic Kind = "anthropic")
type Profile struct { ID, Name string; Kind Kind; BaseURL, Model string }
type ItemVerdict struct { Path string `json:"path"`; Verdict rules.Verdict `json:"verdict"`; Reason string `json:"reason"`; Confidence float64 `json:"confidence"` }
type Analysis struct { Items []ItemVerdict `json:"items"`; ReportMarkdown string `json:"report_markdown"` }
type Provider interface { Analyze(ctx context.Context, p Payload) (Analysis, error) }
func NewProvider(pr Profile, apiKey string, hc *http.Client) (Provider, error)
var (ErrAuth, ErrModelNotFound, ErrRateLimited, ErrInvalidResponse, ErrTimeout, ErrNetwork error)
func ParseAnalysis(text string) (Analysis, error) // JSON puro, cercas ```json, ou primeiro objeto {...} balanceado
type Resolved struct { RealPath string; Verdict rules.Verdict; Reason string; Confidence float64 }
func Resolve(a Analysis, pm PathMap) []Resolved // descarta caminhos fora do PathMap e vereditos inválidos
func Merge(base rules.Classification, r Resolved) rules.Classification
func CacheKey(payloadJSON []byte, kind Kind, model string) string // hex sha256
```
`Merge`: `SourceProtected` → `base` intacto; `SourceUnknown` → `Verdict = r.Verdict`, `Source = SourceAI`; `SourceRule` → `Verdict = MostConservative(base.Verdict, r.Verdict)`, `Source` continua `rule`.
OpenAI-compat: `POST {BaseURL}/chat/completions` (BaseURL padrão `https://api.openai.com/v1`), `Authorization: Bearer` só se houver chave, `response_format: {type: json_schema, ...}`; se o servidor responder 400 citando `response_format`, repetir com `{type: json_object}`. Anthropic: `POST https://api.anthropic.com/v1/messages`, headers `x-api-key` e `anthropic-version: 2023-06-01`, uma tool `report_analysis` com o schema e `tool_choice` forçando ela. Mapeamento HTTP: 401/403 → `ErrAuth`; 404 → `ErrModelNotFound`; 429 → `ErrRateLimited`; timeout → `ErrTimeout`. Retentativas: 2, backoff 1 s e 3 s, só em 429/5xx/erro de rede. Timeout do client: 120 s. Prompt de sistema em `prompt.go` (PT-BR): papel de especialista em limpeza de disco para devs Windows, regras "nunca recomende apagar dados do usuário, repositórios ou configurações", schema de saída, relatório em Markdown com seções "Pode apagar com segurança", "Avalie antes", "Não apague".

- [ ] **Step 1: Testes de `ParseAnalysis` (`response_test.go`)** (Review Focus 4)

JSON puro → ok; com cerca ```` ```json ```` → ok; texto "Aqui está:" + objeto → ok; objeto com `{` dentro de string → ok; texto sem JSON → `ErrInvalidResponse`.

- [ ] **Step 2: Testes de provedores com `httptest.Server`**

`TestOpenAICompatSendsSchemaAndAuth`: verifica caminho `/chat/completions`, header `Authorization: Bearer k`, `response_format.type == "json_schema"`; responde com análise válida → `Analysis` correta.
`TestOpenAICompatFallbackJSONObject`: 1ª resposta 400 com corpo citando `response_format` → 2ª requisição com `json_object` → ok.
`TestOpenAICompatNoKey`: chave vazia → sem header `Authorization`.
`TestAnthropicToolUse`: verifica headers e `tool_choice`; responde bloco `tool_use` com `input` = análise → ok.
`TestErrorMapping` (tabela, os dois provedores): 401→`ErrAuth`, 404→`ErrModelNotFound`, 429 persistente→`ErrRateLimited` após 3 tentativas (contador no servidor), 500 depois 200→sucesso.
`TestTimeout`: servidor dorme além do timeout configurado no teste (client com 50 ms) → `ErrTimeout`.

- [ ] **Step 3: Testes de `Resolve`/`Merge`/`CacheKey` (`merge_test.go`)**

`TestResolveDropsHallucinated`: item com caminho fora do `PathMap` → descartado; verdict "delete" → descartado.
`TestMerge` (tabela): protected+safe→keep/protected; unknown+safe→safe/ai; rule safe+caution→caution/rule; rule caution+safe→caution/rule.
`TestCacheKeyStable`: mesmo input → mesma chave; modelo diferente → chave diferente.

- [ ] **Step 4–6: falhar → implementar → passar** — Run: `go test ./internal/ai/ -race`
- [ ] **Step 7: Commit** — `git commit -m "feat(ai): OpenAI-compatible and Anthropic providers with validated merge"`

---

## Fase D — Exclusão

### Task 12: `cleaner` — planejamento e tokens

**Files:**
- Create: `internal/cleaner/types.go`, `internal/cleaner/plan.go`, `internal/cleaner/deps_windows.go`
- Test: `internal/cleaner/plan_test.go`

**Interfaces:**
- Consumes: `rules.Protector`, `rules.Classification`, `rules.Action`.
- Produces:
```go
type Mode string
const (ModeRecycle Mode = "recycle"; ModePermanent Mode = "permanent")
type Request struct { Path string; Size int64; Class rules.Classification }
type PlanItem struct { Path string; Size int64; Class rules.Classification; Warnings []string }
type Refusal struct { Path, Reason string }
const ( // Reason
    ReasonNotFound = "not_found"; ReasonProtected = "protected"; ReasonContainsProtected = "contains_protected"
    ReasonContainsGit = "contains_git"; ReasonNotScanned = "not_scanned"; ReasonNoRecycleBin = "no_recycle_bin"
    ReasonExceedsRecycleBin = "exceeds_recycle_bin"; ReasonManualAction = "manual_action"; ReasonKeep = "keep" )
const WarnManyFiles = "many_files" // ≥ 20.000 arquivos no modo Lixeira
type Plan struct { ID string; Mode Mode; Items []PlanItem; Refused []Refusal; TotalBytes int64; Token string; ExpiresAt time.Time }
type Deps interface {
    Canonicalize(path string) (string, error)         // GetFinalPathNameByHandle; inexistente → fs.ErrNotExist
    ContainsGit(path string) bool                     // existe algum diretório .git abaixo de path
    RecycleBinRemaining(volumeRoot string) (remaining int64, hasBin bool, err error)
    VolumeOf(path string) string
}
type Planner struct{ /* ... */ }
func NewPlanner(p *rules.Protector, d Deps, scanned func(volumeRoot string) bool, now func() time.Time) *Planner
func (pl *Planner) Plan(reqs []Request, mode Mode, fileCount func(path string) uint32) (Plan, error)
func (pl *Planner) Redeem(planID, token string) (Plan, error) // ErrInvalidToken, ErrExpired; uso único
var (ErrInvalidToken, ErrExpired error)
```
Ordem das checagens por item: `Class.Verdict == keep` → `keep`; `Action == manual` → `manual_action`; `Canonicalize` falha → `not_found`; `Protector.Check(canon) != ProtectNone` → `protected`; `HasProtectedDescendant` → `contains_protected`; `ContainsGit && Class.Action != delete_dir` → `contains_git`; volume não varrido → `not_scanned`; modo Lixeira sem Lixeira → `no_recycle_bin`; soma acumulada dos itens aceitos no volume > `remaining` → `exceeds_recycle_bin`. Token: `crypto/rand` 32 bytes em hex, amarrado ao `ID` e ao sha256 dos caminhos aceitos; validade 5 min.
`RecycleBinRemaining` (Windows): capacidade = `MaxCapacity` (MB) em `HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\BitBucket\Volume\{GUID}`; se ausente, 5% do total do volume; uso atual via `SHQueryRecycleBinW`. `hasBin = false` para drives não-fixos.

- [ ] **Step 1: Testes** (com `fakeDeps`)

`TestRefusalReasons` (tabela, um caso por motivo acima).
`TestJunctionIntoProtectedRefused`: `Canonicalize("D:\link")` → `C:\Windows\System32` → `protected`.
`TestStaleItemRefused` (Review Focus 2): caminho que deixou de existir → `not_found`, os outros itens seguem aceitos.
`TestGitInsideNodeModulesAllowed`: `node_modules` (delete_dir) com `.git` dentro → aceito; pasta `unknown` com `.git` → `contains_git`.
`TestRecycleCapacityCumulative`: `remaining` 100, itens 60 e 60 → primeiro aceito, segundo `exceeds_recycle_bin`; modo permanente → ambos aceitos.
`TestManyFilesWarning`: `fileCount` 25.000 no modo Lixeira → `WarnManyFiles`.
`TestTokenSingleUse`: `Redeem` ok; 2º `Redeem` → `ErrInvalidToken`.
`TestTokenExpires`: `now` avança 5 min + 1 s → `ErrExpired`.
`TestTokenWrongPlan`: token do plano A com ID do plano B → `ErrInvalidToken`.

- [ ] **Step 2–4: falhar → implementar → passar** — Run: `go test ./internal/cleaner/ -race`
- [ ] **Step 5: Commit** — `git commit -m "feat(cleaner): validated deletion plans with single-use tokens"`

### Task 13: `cleaner` — execução (Lixeira e permanente)

**Files:**
- Create: `internal/cleaner/execute.go`, `internal/cleaner/delete_windows.go`, `internal/cleaner/recycle_windows.go`
- Test: `internal/cleaner/execute_test.go`, `internal/cleaner/delete_windows_test.go`

**Interfaces:**
- Consumes: `Plan`, `PlanItem` (Task 12).
- Produces:
```go
type Status string
const (StatusOK Status = "ok"; StatusSkipped Status = "skipped"; StatusError Status = "error")
type ItemResult struct { Path string; Status Status; Reason string; Freed int64; SkippedFiles []string }
type Deleter interface {
    Recycle(path string) error
    RemoveAll(path string) (freed int64, skipped []string, err error)      // pula arquivos em uso
    RemoveContents(path string) (freed int64, skipped []string, err error) // mantém a raiz
}
func OSDeleter() Deleter
func Execute(ctx context.Context, plan Plan, d Deleter, onItem func(i int, r ItemResult)) []ItemResult
```
`Execute` respeita `plan.Mode` e `Class.Action` (`delete_contents` no modo Lixeira → `Recycle` em cada filho direto). Cancelamento checado entre itens; itens não iniciados → `skipped` com motivo `canceled`. `Freed` no modo Lixeira = `PlanItem.Size`. Item com arquivos pulados mas algo apagado → `ok` + `SkippedFiles`. `Recycle`: `IFileOperation` via go-ole (`CLSID_FileOperation`), flags `FOFX_RECYCLEONDELETE|FOF_NOCONFIRMATION|FOF_SILENT|FOF_NOERRORUI|FOF_WANTNUKEWARNING`, numa goroutine com `runtime.LockOSThread` + `CoInitializeEx(COINIT_APARTMENTTHREADED)`. `RemoveAll`: caminhos `\\?\`, limpa `FILE_ATTRIBUTE_READONLY` antes de apagar, `ERROR_SHARING_VIOLATION`/`ERROR_ACCESS_DENIED` → pula e registra.

- [ ] **Step 1: Testes com `fakeDeleter` (`execute_test.go`)**

`TestExecuteModes`: recycle+delete_dir → `Recycle`; permanent+delete_contents → `RemoveContents`; recycle+delete_contents → `Recycle` por filho.
`TestExecuteCancelBetweenItems`: cancelar no `onItem` do item 0 → itens 1..n `skipped`/`canceled`.
`TestExecuteManyItems`: 2.000 itens → 2.000 resultados, `onItem` chamado 2.000 vezes em ordem.
`TestErrorDoesNotAbort`: item 1 falha → `error`, item 2 `ok`.

- [ ] **Step 2: Testes reais em `TempDir` (`delete_windows_test.go`)**

`TestRemoveAllReadOnly`: arquivo somente-leitura dentro de `x` → `x` removido, `freed` = tamanho.
`TestRemoveAllSkipsLocked`: arquivo aberto com `windows.CreateFile` sem `FILE_SHARE_DELETE` → pulado, demais removidos, raiz permanece.
`TestRemoveContentsKeepsRoot`.
`TestRecycleMovesToBin`: arquivo temporário → `Recycle` → não existe mais no caminho original (pular com `t.Skip` se `CI_NO_RECYCLE=1`).

- [ ] **Step 3–5: falhar → implementar → passar** — Run: `go test ./internal/cleaner/ -race`
- [ ] **Step 6: Commit** — `git commit -m "feat(cleaner): recycle bin and permanent deletion executors"`

---

## Fase E — Fachada do app

### Task 14: `app` — sessão, varredura, resultados e eventos

**Files:**
- Create: `internal/app/deps.go`, `internal/app/dto.go`, `internal/app/events.go`, `internal/app/scan.go`, `internal/app/results.go`
- Modify: `internal/app/app.go`, `main.go`
- Test: `internal/app/scan_test.go`, `internal/app/results_test.go`, `internal/app/events_test.go`

**Interfaces:**
- Consumes: todos os pacotes anteriores.
- Produces:
```go
type Emitter interface { Emit(event string, data any) }
type Deps struct {
    Store *store.Store; Volumes func() ([]volumes.Volume, error); NewScanner func(volumeRoot string) scanner.Scanner
    Rules []rules.Rule; RuleProblems []rules.LoadProblem; Protector *rules.Protector; Classifier *rules.Classifier
    Planner *cleaner.Planner; Deleter cleaner.Deleter
    NewProvider func(ai.Profile, string) (ai.Provider, error); SecretGet func(string) (string, error); SecretSet func(string, string) error; SecretDelete func(string) error
    Emitter Emitter; Now func() time.Time; Elevated func() bool; RestartElevated func() error
}
// eventos
const (EvScanProgress = "scan:progress"; EvScanDone = "scan:done"; EvCleanProgress = "clean:progress"; EvCleanDone = "clean:done"; EvAIDone = "ai:done")
type Throttle struct{ /* ... */ } // NewThrottle(interval time.Duration, now func() time.Time); Allow() bool
// bindings (métodos de *App)
func (a *App) ListVolumes() ([]VolumeDTO, error)
func (a *App) IsElevated() bool
func (a *App) RestartElevated() error
func (a *App) StartScan(roots []string) error // assíncrono; um volume por vez, em sequência; erro se já houver varredura
func (a *App) CancelScan()
func (a *App) GetSummary(volume string) (SummaryDTO, error)
func (a *App) GetChildren(volume string, id int32) ([]NodeDTO, error) // id -1 = raiz
func (a *App) ListFiles(volume string, id int32) ([]FileDTO, error)
func (a *App) GetTopFiles(volume string) ([]FileDTO, error)
func (a *App) GetExtensions(volume string) ([]ExtDTO, error)
```
DTOs (`dto.go`, tags json camelCase): `VolumeDTO{Root, Label, FS, Type, Total, Free, HasRecycleBin, LastScan *time.Time, EstimatedJunk int64}`; `ProgressDTO{Volume, Files, Dirs, Bytes, Denied int64; Current string; BytesPerSec float64}`; `ScanDoneDTO{Volume string; DurationMs int64; Error string; DeniedCount int; DeniedSample []string}`; `NodeDTO{ID int32; Name, Path string; Size int64; Files uint32; LatestMod int64; HasChildren, Link, Cloud, Denied, Inactive bool; Verdict, Category, Source, RuleID, Action, Explain, Regenerable, Command string; AI *AIOpinionDTO}`; `AIOpinionDTO{Verdict, Reason string; Confidence float64}`; `FileDTO{Name, Path string; Size, ModTime int64; Verdict, RuleID string}`; `ExtDTO{Ext string; Count uint64; Bytes int64}`; `SummaryDTO{Volume string; SafeBytes, CautionBytes, KeepBytes int64; ByCategory []CategoryDTO}`; `CategoryDTO{Category string; Bytes int64; Count int}`.
Sessão: `map[volume]*session{tree, classes, aiOpinions map[NodeID]ai.Resolved, scannedAt}`, protegida por `sync.RWMutex`; ao iniciar, carrega snapshots salvos e reclassifica. `SummaryDTO` soma só nós "topo" classificados (sem ancestral classificado) para não contar duas vezes. `EstimatedJunk` = `SafeBytes`.

- [ ] **Step 1: Testes** (Deps falsos: scanner falso alimentando o Builder, emitter que grava eventos, store em TempDir)

`TestThrottle`: 100 `Allow()` em 1 s simulado → ≤ 10 true (+1 final garantido pelo chamador).
`TestStartScanEmitsAndSaves`: `StartScan(["C:\"])` → eventos `scan:progress` (≥ 1) e `scan:done` sem erro; `LoadSnapshot("C:\")` não-nil.
`TestScanRootUnavailableNotSaved` (Review Focus 5): scanner retorna `ErrRootUnavailable` → `scan:done.Error != ""`, nenhum snapshot salvo.
`TestStartScanWhileRunning` → erro.
`TestGetChildrenSortedWithClassification`: filhos desc por tamanho; `node_modules` com `Verdict "safe"`, `Explain` e `Regenerable` preenchidos.
`TestSummaryNoDoubleCount`: `node_modules` aninhado não soma duas vezes.
`TestReloadFromSnapshotReclassifies`: novo `App` com o mesmo store → `GetChildren` funciona sem nova varredura.

- [ ] **Step 2–4: falhar → implementar → passar** — Run: `go test ./internal/app/ -race`

- [ ] **Step 5: Ligar em `main.go`**

Montar `Deps` reais (`store.Open(%LOCALAPPDATA%\NodeGarbage\nodegarbage.db)`, `rules.LoadBuiltin` + `LoadUserDir`, `os.Getenv` como `Env`, `scanner.WalkScanner{FS: scanner.OSFS()}`, `cleaner.OSDeleter()`, `secrets.*`, `elevation.*`); `Emitter` usa `runtime.EventsEmit(ctx, ...)` do Wails. Run: `wails build` → compila.

- [ ] **Step 6: Commit** — `git commit -m "feat(app): scan session, results bindings and throttled events"`

### Task 15: `app` — IA, limpeza, histórico e configurações

**Files:**
- Create: `internal/app/ai.go`, `internal/app/clean.go`, `internal/app/settings.go`
- Test: `internal/app/ai_test.go`, `internal/app/clean_test.go`, `internal/app/settings_test.go`

**Interfaces:**
- Consumes: Task 14 e pacotes `ai`, `cleaner`, `store`.
- Produces:
```go
func (a *App) ListAIProfiles() ([]AIProfileDTO, error)                  // AIProfileDTO{ID, Name, Kind, BaseURL, Model string; HasKey bool}
func (a *App) SaveAIProfile(p AIProfileDTO, apiKey string) (AIProfileDTO, error) // apiKey "" = manter a atual; ID vazio = novo (uuid)
func (a *App) DeleteAIProfile(id string) error
func (a *App) TestAIProfile(id string) error                            // payload mínimo; devolve erro amigável
func (a *App) PreviewAIPayload(profileID string) (PayloadPreviewDTO, error) // {JSON string; Tokens int; Provider, Model string; Cached bool}
func (a *App) RunAI(profileID string, force bool) error                 // assíncrono → evento ai:done {Report AIReportDTO | Error string}
func (a *App) CancelAI()
func (a *App) GetAIReport() (*AIReportDTO, error)                       // AIReportDTO{Markdown string; Items []AIItemDTO{Path, Verdict, Reason string; Confidence float64}; CreatedAt time.Time}
func (a *App) PlanClean(paths []string, mode string) (PlanDTO, error)   // PlanDTO{ID, Mode, Token string; Items []PlanItemDTO; Refused []RefusalDTO; TotalBytes int64; ExpiresAt time.Time}
func (a *App) ExecuteClean(planID, token string) error                  // assíncrono → clean:progress {Index, Total int; Result ItemResultDTO}, clean:done {Freed int64; Results []ItemResultDTO}
func (a *App) CancelClean()
func (a *App) GetHistory(limit int) (HistoryDTO, error)                 // {TotalFreed int64; Items []CleanupDTO}
func (a *App) GetSettings() (SettingsDTO, error)                        // {RedactTerms, ExtraProtected []string; AIMinBytes int64; Workers int; UserRulesDir string; RuleProblems []RuleProblemDTO}
func (a *App) SaveSettings(s SettingsDTO) error
```
Perfis guardados em `settings` (chave `ai.profiles`, JSON sem chave de API). `RunAI`: monta payload com `ai.BuildPayload` (anonimizador com `os.Getenv("USERNAME")` + `RedactTerms`), consulta `GetAIReport` do store pela `CacheKey` (salvo `force`), chama o provedor, aplica `Resolve` → `Merge` nas sessões e guarda `aiOpinions`. `PlanClean` resolve cada caminho na sessão (`Tree.Find` → `Classifications.Get`, `Size` = `TotalSize`) e usa `Planner.Plan`; caminho não encontrado em nenhuma sessão → recusa `not_scanned`. `ExecuteClean` chama `Planner.Redeem` e só então `cleaner.Execute`; cada resultado vira `store.AddCleanup`; ao final, remove da sessão os nós apagados (`ok`) e atualiza os totais dos ancestrais.

- [ ] **Step 1: Testes**

`TestProfileKeyNeverInSettings`: `SaveAIProfile(p, "sk-123")` → `SecretSet` chamado; `GetSetting("ai.profiles")` não contém `sk-123`; `HasKey true`.
`TestPreviewMatchesSent`: `PreviewAIPayload` JSON == corpo enviado ao provedor falso em `RunAI`.
`TestRunAIUsesCache`: 2ª `RunAI(force=false)` não chama o provedor; `force=true` chama.
`TestRunAIMergesOpinions`: provedor falso marca um unknown como safe → `GetChildren` mostra `Source "ai"` e `AI` preenchido.
`TestExecuteRequiresValidToken`: `ExecuteClean(id, "errado")` → erro e `Deleter` nunca chamado.
`TestCleanRecordsHistoryAndUpdatesTree`: plano de 2 itens → `GetHistory` com 2 itens; nó removido de `GetChildren`; tamanho do pai reduzido.
`TestOnlyExecuteCleanDeletes`: percorrer todos os outros bindings públicos (`ListVolumes`, `GetChildren`, `RunAI`, `PlanClean`, …) com `Deleter` falso → zero chamadas ao `Deleter`.

- [ ] **Step 2–4: falhar → implementar → passar** — Run: `go test ./internal/app/ -race`
- [ ] **Step 5: Commit** — `git commit -m "feat(app): AI analysis, two-step cleaning, history and settings bindings"`

---

## Fase F — Frontend

### Task 16: Fundação do frontend (Svelte 5, Tailwind, tokens, shell, Mica)

**Files:**
- Modify: `frontend/package.json`, `frontend/vite.config.ts`, `frontend/src/main.ts`, `frontend/src/App.svelte`, `main.go` (opções Windows)
- Create: `frontend/src/styles/tokens.css`, `frontend/src/styles/app.css`, `frontend/src/lib/strings.ts`, `frontend/src/lib/format.ts`, `frontend/src/lib/api.ts`, `frontend/src/lib/route.svelte.ts`, `frontend/src/components/layout/Sidebar.svelte`, `frontend/src/components/ui/{Button,Panel,VerdictBadge,Skeleton,EmptyState,ErrorState,ProgressBar}.svelte`, `frontend/vitest.config.ts`, `frontend/src/test/setup.ts`
- Test: `frontend/src/lib/format.test.ts`, `frontend/src/components/ui/VerdictBadge.test.ts`, `frontend/src/components/layout/Sidebar.test.ts`

**Interfaces:**
- Consumes: bindings gerados em `frontend/wailsjs/go/app/App` e `frontend/wailsjs/runtime`.
- Produces:
  - `api.ts`: reexporta cada binding da Task 14/15 com os mesmos nomes e tipos (`ListVolumes`, `StartScan`, …) e `on(event: string, cb: (data: any) => void): () => void`; é o único módulo que importa `wailsjs` (testes fazem `vi.mock('$lib/api')`).
  - `route.svelte.ts`: `type Route = 'home' | 'scan' | 'results' | 'ai' | 'history' | 'settings'`; `export const route = $state({ current: 'home' as Route, volume: '' as string })`.
  - `format.ts`: `formatBytes(n: number): string` (pt-BR, base 1024, 1 casa: `1,5 GB`), `formatDate(unixSec: number): string` (`dd/mm/aaaa`), `formatCount(n: number): string` (`1.234.567`).
  - `VerdictBadge` props `{ verdict: 'safe'|'caution'|'keep'|'unknown', source?: string }`: ícone + rótulo de `strings.verdict[verdict]` + "sugerido pela IA" quando `source === 'ai'`.

Tokens (`tokens.css`, valores da spec §8): `--surface: #F0F0F0; --glass: rgba(255,255,255,.55); --glass-border: rgba(255,255,255,.6); --accent: #105BFF; --accent-hover: #0A47D1; --warn: #FE8500; --warn-text: #B35C00; --safe: #157F3B; --keep: #5B6270; --text-primary: #16181D; --text-muted: #4B5160; --radius-sm: 6px; --radius-md: 10px; --radius-lg: 14px; --blur: 20px;` fontes `"Segoe UI Variable", "Segoe UI", sans-serif` e `"Cascadia Mono", Consolas, monospace`. Tailwind v4 com `@theme` mapeando esses tokens. `.glass { background: var(--glass); backdrop-filter: blur(var(--blur)); border: 1px solid var(--glass-border); }` com `@supports not (backdrop-filter: blur(1px)) { background: var(--surface) }` e `@media (prefers-reduced-motion: reduce)` zerando transições.
`main.go`: `Windows: &windows.Options{WebviewIsTransparent: true, WindowIsTranslucent: true, BackdropType: windows.Mica}`, `BackgroundColour: &options.RGBA{R: 240, G: 240, B: 240, A: 0}`. No Windows 10 (build < 22000) usar `BackdropType: windows.None` e alfa 255.

- [ ] **Step 1: Atualizar dependências** — Svelte 5, `@sveltejs/vite-plugin-svelte`, Vite, TypeScript, `tailwindcss` + `@tailwindcss/vite`, `vitest`, `@testing-library/svelte`, `@testing-library/user-event`, `jsdom`, `svelte-check`. Scripts: `dev`, `build`, `check` (`svelte-check`), `test` (`vitest`).

- [ ] **Step 2: Testes**

`format.test.ts`: `formatBytes(0) === '0 B'`, `formatBytes(1536) === '1,5 KB'`, `formatBytes(1610612736) === '1,5 GB'`, `formatCount(1234567) === '1.234.567'`.
`VerdictBadge.test.ts`: `caution` → texto `strings.verdict.caution` e ícone com `aria-hidden="true"`; `source: 'ai'` → texto "sugerido pela IA".
`Sidebar.test.ts`: 6 itens de navegação por `role="link"`/`button`; clicar em "Histórico" muda `route.current` para `history`; item ativo com `aria-current="page"`.

- [ ] **Step 3–5: falhar → implementar → passar** — Run: `npm --prefix frontend run check && npm --prefix frontend test -- --run`

- [ ] **Step 6: Verificar visual** — Run: `wails dev`; conferir Mica + sidebar em vidro em 1024 e 1920 px.

- [ ] **Step 7: Commit** — `git commit -m "feat(ui): Svelte 5 foundation, design tokens, glass shell"`

### Task 17: Telas Início e Varredura

**Files:**
- Create: `frontend/src/screens/Home.svelte`, `frontend/src/screens/Scan.svelte`, `frontend/src/components/home/VolumeCard.svelte`, `frontend/src/lib/stores/scan.svelte.ts`
- Test: `frontend/src/screens/Home.test.ts`, `frontend/src/screens/Scan.test.ts`

**Interfaces:**
- Consumes: `ListVolumes`, `IsElevated`, `RestartElevated`, `StartScan`, `CancelScan`, eventos `scan:progress`/`scan:done`.
- Produces: `scan.svelte.ts` → `scanState = $state({ running: boolean, queue: string[], progress: Record<string, ProgressDTO>, done: Record<string, ScanDoneDTO> })`, `startScan(roots: string[])`, `cancelScan()`; ao receber o último `scan:done` sem erro → `route.current = 'results'`, `route.volume = roots[0]`.

- [ ] **Step 1: Testes** (api mockada)

`Home.test.ts`: skeleton enquanto `ListVolumes` pende; 2 volumes → 2 cards com label, FS, livre formatado e barra com `aria-valuenow`; banner de admin visível quando `IsElevated` = false e botão "Reiniciar como administrador" chama `RestartElevated`; botão "Analisar" desabilitado sem volume marcado; marcando C: e clicando → `StartScan(['C:\\'])`; erro em `ListVolumes` → `ErrorState` com "Tentar novamente".
`Scan.test.ts`: emitir `scan:progress` → contadores formatados e região `aria-live="polite"`; "Cancelar" chama `CancelScan`; `scan:done` com `Error` → `ErrorState` com a mensagem e botão voltar.

- [ ] **Step 2–4: falhar → implementar → passar** — Run: `npm --prefix frontend test -- --run`
- [ ] **Step 5: Commit** — `git commit -m "feat(ui): home and scan screens"`

### Task 18: Tela Resultados — resumo, árvore virtualizada e detalhes

**Files:**
- Create: `frontend/src/screens/Results.svelte`, `frontend/src/components/results/{SummaryBar,TreeList,TreeRow,DetailsPanel,Breadcrumb}.svelte`, `frontend/src/lib/stores/results.svelte.ts`, `frontend/src/lib/stores/selection.svelte.ts`, `frontend/src/lib/virtual.ts`
- Test: `frontend/src/components/results/TreeList.test.ts`, `frontend/src/components/results/SummaryBar.test.ts`, `frontend/src/lib/virtual.test.ts`, `frontend/src/lib/stores/selection.test.ts`

**Interfaces:**
- Consumes: `GetSummary`, `GetChildren`, `ListFiles`.
- Produces:
  - `results.svelte.ts`: `results = $state({ volume, currentId: -1, path: NodeDTO[], children: NodeDTO[], categoryFilter: string|null, loading, error })`, `openNode(id: number)`, `goUp(index: number)`, `refresh()`.
  - `selection.svelte.ts`: `selection = $state({ paths: Map<string, NodeDTO> })`, `toggle(n: NodeDTO)`, `clear()`, `selectedBytes(): number`; **nunca** pré-seleciona; `toggle` ignora nós com `verdict === 'keep'` ou `action === 'manual'` (checkbox desabilitado).
  - `virtual.ts`: `visibleRange(scrollTop: number, viewport: number, rowHeight: number, count: number, overscan = 8): { start: number; end: number }`.

- [ ] **Step 1: Testes**

`virtual.test.ts`: `visibleRange(0, 400, 40, 1000)` → `{start: 0, end: 18}`; `visibleRange(4000, 400, 40, 1000)` → `{start: 92, end: 118}`; fim limitado a `count`.
`selection.test.ts`: `toggle` de `keep` não seleciona; `selectedBytes` soma; nenhum item selecionado após `openNode`.
`TreeList.test.ts`: 5.000 filhos → no máximo ~40 linhas no DOM; `role="tree"` com `role="treeitem"`; ↓/↑ movem foco, Espaço alterna seleção, Enter chama `openNode`; linha mostra `VerdictBadge`, tamanho e selo "projeto inativo" quando `inactive`.
`SummaryBar.test.ts`: mostra "X recuperáveis com segurança" e "Y exigem cuidado" formatados; clicar no chip de categoria define `categoryFilter` (com `aria-pressed`).
`DetailsPanel`: (dentro do teste do TreeList) item com `command` mostra o comando em `<code>` com botão "Copiar" (`navigator.clipboard.writeText` mockado); item com `ai` mostra parecer e confiança.

- [ ] **Step 2–4: falhar → implementar → passar** — Run: `npm --prefix frontend test -- --run`
- [ ] **Step 5: Commit** — `git commit -m "feat(ui): results screen with virtualized tree and details"`

### Task 19: Treemap em Canvas

**Files:**
- Create: `frontend/src/components/treemap/Treemap.svelte`, `frontend/src/components/treemap/layout.ts`
- Modify: `frontend/src/screens/Results.svelte`
- Test: `frontend/src/components/treemap/layout.test.ts`

**Interfaces:**
- Consumes: `results.children` (Task 18).
- Produces: `layout.ts` → `computeTreemap(nodes: {id: number; size: number}[], width: number, height: number): Rect[]` com `Rect = {id, x, y, w, h}` (d3-hierarchy `treemap().tile(treemapSquarify)`, padding 2); `hitTest(rects: Rect[], x: number, y: number): number | null`. `Treemap.svelte` props `{ nodes: NodeDTO[], onOpen: (id: number) => void, onHover: (id: number|null) => void }`: desenha em `<canvas>` com `devicePixelRatio`, cor por veredito via `getComputedStyle` dos tokens (`--safe`, `--warn`, `--keep`, `--accent` para unknown), rótulo quando `w > 60 && h > 18`; `ResizeObserver`; canvas com `aria-hidden="true"` (a lista é a alternativa acessível).

- [ ] **Step 1: Testes** — `computeTreemap` de tamanhos `[50, 30, 20]` em 100×100: soma das áreas ≈ 100×100 menos padding (tolerância 5%), maior retângulo pertence ao id do 50; nós com `size 0` omitidos; `hitTest` no centro do maior retorna seu id e fora retorna `null`.
- [ ] **Step 2–4: falhar → implementar → passar** — Run: `npm --prefix frontend test -- --run`
- [ ] **Step 5: Verificar visual** — `wails dev` numa varredura real: clique abre a pasta, hover destaca a linha na lista.
- [ ] **Step 6: Commit** — `git commit -m "feat(ui): canvas treemap"`

### Task 20: IA — perfis, pré-visualização e relatório

**Files:**
- Create: `frontend/src/screens/AIReport.svelte`, `frontend/src/components/ai/{PayloadPreview,ReportView,ProfileForm}.svelte`, `frontend/src/lib/markdown.ts`, `frontend/src/lib/stores/ai.svelte.ts`
- Test: `frontend/src/lib/markdown.test.ts`, `frontend/src/components/ai/ProfileForm.test.ts`, `frontend/src/screens/AIReport.test.ts`

**Interfaces:**
- Consumes: `ListAIProfiles`, `SaveAIProfile`, `DeleteAIProfile`, `TestAIProfile`, `PreviewAIPayload`, `RunAI`, `CancelAI`, `GetAIReport`, evento `ai:done`.
- Produces: `markdown.ts` → `renderMarkdown(md: string): string` (`marked` com `breaks: true`, depois `DOMPurify.sanitize` com `ALLOWED_URI_REGEXP` só `http`/`https`, sem `<img>`, `<iframe>`, `<script>`, `<style>`, atributos `on*`). `ProfileForm` props `{ profile?: AIProfileDTO, onSaved: (p: AIProfileDTO) => void }`: campos nome, tipo (OpenAI-compatível | Anthropic), base URL (pré-preenchida `https://api.openai.com/v1` para OpenAI-compat; oculta para Anthropic), modelo, chave (`type="password"`, placeholder "•••• (mantida)" quando `hasKey`), botão "Testar conexão".

- [ ] **Step 1: Testes**

`markdown.test.ts`: `<script>alert(1)</script>` removido; `<img src=x onerror=alert(1)>` removido; `[x](javascript:alert(1))` sem `href` perigoso; `<a href="https://ok">` mantido; `**negrito**` vira `<strong>`.
`ProfileForm.test.ts`: troca para Anthropic oculta base URL; salvar sem alterar a chave envia `apiKey ''`; "Testar conexão" mostra sucesso ou a mensagem de erro.
`AIReport.test.ts`: sem perfis → `EmptyState` com link para Configurações; com perfil → "Ver o que será enviado" mostra JSON em `<pre>` + tokens + provedor/modelo; "Enviar" chama `RunAI(id, false)` e desabilita até `ai:done`; `ai:done` com relatório → HTML renderizado via `renderMarkdown`; clicar num caminho de `Items` leva aos Resultados (`route.current = 'results'`) e abre o nó pai; erro → `ErrorState`.

- [ ] **Step 2–4: falhar → implementar → passar** — Run: `npm --prefix frontend test -- --run`
- [ ] **Step 5: Commit** — `git commit -m "feat(ui): AI profiles, payload preview and sanitized report"`

### Task 21: Diálogo de limpeza, Histórico e Configurações

**Files:**
- Create: `frontend/src/components/clean/CleanDialog.svelte`, `frontend/src/screens/History.svelte`, `frontend/src/screens/Settings.svelte`, `frontend/src/lib/stores/clean.svelte.ts`
- Modify: `frontend/src/screens/Results.svelte` (barra de ação "Limpar selecionados (X)")
- Test: `frontend/src/components/clean/CleanDialog.test.ts`, `frontend/src/screens/History.test.ts`, `frontend/src/screens/Settings.test.ts`

**Interfaces:**
- Consumes: `PlanClean`, `ExecuteClean`, `CancelClean`, eventos `clean:progress`/`clean:done`; `GetHistory`; `GetSettings`/`SaveSettings`; `selection` (Task 18).
- Produces: `CleanDialog` props `{ open: boolean, onClose: () => void }`; usa `<dialog>` com `showModal()`, `aria-labelledby`, Esc fecha (exceto durante execução), foco inicial no seletor de modo e restaurado ao botão de origem ao fechar.

Fluxo do diálogo: ao abrir, `PlanClean(paths, 'recycle')`; trocar modo refaz o plano; lista de itens aceitos (virtualizada se > 100) com avisos e lista de recusados com motivo traduzido (`strings.refusal[reason]`); itens `caution` com checkbox individual obrigatório; modo permanente exige checkbox "Entendo que isso não pode ser desfeito"; botão "Limpar X" desabilitado até tudo marcado; executar chama `ExecuteClean(plan.id, plan.token)`, mostra progresso e resultado final (liberado, pulados, erros); token expirado → mensagem e botão "Gerar novo plano".

- [ ] **Step 1: Testes**

`CleanDialog.test.ts`: abre com `PlanClean` chamado em modo `recycle`; item `caution` sem checkbox → botão desabilitado; marcando → habilitado; modo permanente sem confirmação → desabilitado; recusa `exceeds_recycle_bin` mostra texto de `strings.refusal` e sugestão de modo permanente; confirmar chama `ExecuteClean(id, token)` exatamente uma vez; Esc fecha antes de executar e não fecha durante; após `clean:done`, `selection` é limpa e `results.refresh()` chamado.
`History.test.ts`: total liberado formatado; tabela com caminho (mono), tamanho, modo, data e status com ícone + texto; vazio → `EmptyState`.
`Settings.test.ts`: lista de perfis (usa `ProfileForm`); adicionar termo de anonimização e caminho protegido extra e salvar → `SaveSettings` com os arrays; `RuleProblems` exibidos como lista de avisos com arquivo e erro.

- [ ] **Step 2–4: falhar → implementar → passar** — Run: `npm --prefix frontend run check && npm --prefix frontend test -- --run`

- [ ] **Step 5: Verificação manual ponta a ponta** — `wails dev`: varrer um disco, selecionar um `node_modules` de teste criado para isso, limpar para a Lixeira, conferir na Lixeira do Windows, conferir o Histórico. Navegar o app inteiro só com teclado (Tab/setas/Espaço/Enter/Esc).

- [ ] **Step 6: Commit** — `git commit -m "feat(ui): clean confirmation dialog, history and settings"`

---

## Fase G — Open source

### Task 22: CI, release e documentação do repositório

**Files:**
- Create: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `README.md`, `README.pt-BR.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, `.github/ISSUE_TEMPLATE/{bug.yml,new-rule.yml}`, `.github/pull_request_template.md`, `build/windows/installer/` (gerado pelo Wails, ajustar nome/ícone)

**Interfaces:**
- Consumes: comandos de teste/build das tasks anteriores.

- [ ] **Step 1: `ci.yml`** — `on: [push, pull_request]`, `runs-on: windows-latest`; passos: `actions/setup-go` (`go-version: '1.27'`), `actions/setup-node` (LTS), `go vet ./...`, `staticcheck ./...` (via `go install honnef.co/go/tools/cmd/staticcheck@latest`), `go test -race ./...` com `CI_NO_RECYCLE=1`, `npm ci --prefix frontend`, `npm --prefix frontend run check`, `npm --prefix frontend test -- --run`, instalar Wails CLI, `wails build -clean`.

- [ ] **Step 2: `release.yml`** — `on: push: tags: ['v*.*.*']`; build `wails build -clean -nsis -ldflags "-X main.version=${{ github.ref_name }}"`; gera `SHA256SUMS` (`Get-FileHash`); publica `nodegarbage.exe`, o instalador NSIS e `SHA256SUMS` com `softprops/action-gh-release`.

- [ ] **Step 3: Documentação**

`README.md` (inglês): o que é, screenshot, recursos, **"Only you delete: AI and rules only advise"**, privacidade (o que vai para a IA, anonimização, pré-visualização, zero telemetria), instalação, como compilar (`wails build`), limitações conhecidas (hardlinks contam em dobro no motor por diretórios; MFT na v0.2), licença. `README.pt-BR.md`: mesma estrutura em português. `CONTRIBUTING.md`: setup, comandos de teste, convenção de commits, **"Como adicionar uma regra"** (schema YAML, onde colocar, teste de tabela obrigatório em `builtin_test.go`, regra nunca pode liberar caminho protegido). `SECURITY.md`: como reportar (GitHub Security Advisories), escopo (exclusão indevida é severidade alta). `CODE_OF_CONDUCT.md`: Contributor Covenant 2.1. Templates de issue (bug com versão/Windows/passos; nova regra com caminho, ecossistema, como regenerar, fonte) e de PR (checklist: testes, regra com teste de tabela).

- [ ] **Step 4: Verificar** — `actionlint` (se disponível) nos workflows; push numa branch e conferir o CI verde no GitHub.

- [ ] **Step 5: Commit** — `git commit -m "chore: CI, release workflow and open source docs"`
