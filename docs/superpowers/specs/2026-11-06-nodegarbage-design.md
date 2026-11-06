# NodeGarbage — Design (v0.1)

- **Data:** 2026-11-06
- **Status:** aprovado em brainstorming, aguardando revisão da spec escrita
- **Stack:** Go 1.27 + Wails v2 (WebView2) + Svelte + TypeScript + Tailwind

## 1. Entendimento e objetivo

**O que é:** ferramenta desktop open source (repo público, licença MIT) para Windows que varre os discos instalados, classifica arquivos e caches que mais ocupam espaço, explica o que pode ou não ser apagado com segurança e apaga o que o usuário confirmar.

**Para quem:** desenvolvedores e power users. O foco são os caches e artefatos de desenvolvimento (node_modules, caches de gerenciadores de pacotes, artefatos de build, Docker/WSL), sem perder os caches gerais do Windows e dos navegadores.

**Critérios de sucesso:**
- Varrer um disco de ~1 TB com milhões de arquivos em poucos minutos (motor por diretórios) e em segundos (motor MFT, fase 2).
- Mostrar claramente "quanto dá para recuperar com segurança" e onde está.
- Nunca apagar algo protegido nem algo que o usuário não confirmou explicitamente.
- Funcionar 100% sem IA. A IA é opcional e acrescenta explicação e classificação dos itens desconhecidos.

**Princípio inegociável:** só o usuário apaga. A IA (e as regras) apenas classificam e explicam; nenhum componente além do fluxo de confirmação do usuário (`PlanClean` → diálogo → `ExecuteClean` com token) pode disparar uma exclusão. A IA não tem acesso a nenhuma ferramenta ou binding; sua resposta é tratada como dado não confiável.

**Decisões tomadas:**

| Tema | Decisão |
|---|---|
| Exclusão | O app apaga. Lixeira por padrão, com opção de exclusão permanente |
| Autoridade | Regras locais classificam; a IA aconselha; o usuário marca e confirma. Nada é pré-selecionado pela IA |
| Privilégios | Abre sem admin; oferece "reiniciar como administrador" para varredura completa |
| Motores de varredura | MVP: varredura concorrente de diretórios. Fase 2: leitura da MFT (admin + NTFS) |
| IA | Qualquer provedor: OpenAI-compatível (base URL) + Anthropic nativo. Envio de tudo que for relevante, com anonimização e pré-visualização |
| Frontend | Svelte + TS, glassmorphism sobre Mica, tema claro apenas (tokens prontos para tema escuro) |
| Persistência | SQLite local: último snapshot por volume, histórico de limpezas, cache de relatórios da IA |
| Modelo de dados | Árvore de diretórios agregada em memória + top-N arquivos + histograma de extensões (abordagem A) |
| Licença / docs | MIT; README em inglês + versão PT-BR; zero telemetria |

## 2. Arquitetura

```
nodegarbage/
├── main.go                 # bootstrap Wails (janela, Mica, bindings)
├── app.go                  # App: fachada fina exposta ao frontend
├── internal/
│   ├── volumes/            # enumera discos: letra, tipo, FS, total/livre
│   ├── scanner/            # interface Scanner; WalkScanner (MVP); MFTScanner (fase 2)
│   ├── tree/               # Builder/Tree: arena de diretórios, top-N arquivos, histograma
│   ├── rules/              # motor de regras + builtin/*.yaml (go:embed) + lista de proteção
│   ├── ai/                 # Provider, openaicompat, anthropic, payload, anonimização
│   ├── cleaner/            # PlanClean/ExecuteClean; Lixeira (IFileOperation) e permanente
│   ├── store/              # SQLite (modernc.org/sqlite, sem CGO) + migrações
│   ├── secrets/            # chaves de API no Windows Credential Manager
│   └── elevation/          # detectar admin; relançar via ShellExecute "runas"
├── frontend/               # Svelte + TS + Tailwind
└── docs/
```

**Princípios:**
- `app.go` contém só cola. Toda a lógica fica em `internal/`, testável sem Wails.
- Chamadas do frontend via bindings Wails; operações contínuas via eventos (`scan:progress`, `scan:done`, `clean:progress`, `clean:done`, `ai:done`), com no máximo 10 eventos de progresso por segundo.
- Toda operação longa recebe `context.Context` e é cancelável pela UI.
- Só Windows (`//go:build windows` nos arquivos com Win32). `golang.org/x/sys/windows`, sem CGO.

**Fluxo principal:** `volumes` → usuário escolhe discos → `scanner` → `tree.Builder` → `rules` classifica → (opcional) `ai` analisa → usuário marca → `cleaner.PlanClean` → confirmação → `cleaner.ExecuteClean` → `store` registra.

**Bindings expostos (contrato inicial):**

| Método | Descrição |
|---|---|
| `ListVolumes()` | Volumes com uso e data da última varredura salva |
| `IsElevated()` / `RestartElevated()` | Estado e relançamento como admin |
| `StartScan(volumes)` / `CancelScan()` | Inicia/cancela; resultado via eventos |
| `GetSummary(volume)` | Totais recuperáveis por veredito e categoria |
| `GetChildren(nodeID)` | Filhos de um diretório (nós da árvore) com classificação |
| `ListFiles(nodeID)` | Arquivos de um diretório, via `ReadDir` sob demanda |
| `GetTopFiles(volume)` / `GetExtensions(volume)` | Top-N arquivos e histograma |
| `PreviewAIPayload(profileID)` / `RunAI(profileID)` / `CancelAI()` | Pré-visualização (JSON + tokens) e execução |
| `PlanClean(paths, mode)` / `ExecuteClean(planID, token)` / `CancelClean()` | Exclusão em duas etapas |
| `GetHistory()` | Histórico e total liberado |
| `Get/SaveSettings`, `Save/DeleteAIProfile`, `TestAIProfile` | Configurações |

## 3. Scanner e agregação

**Contrato:**
```go
type Scanner interface {
    Scan(ctx context.Context, root string, b *tree.Builder, p *Progress) error
}
```

**WalkScanner (MVP):**
- Pool de workers (padrão `GOMAXPROCS × 4`, configurável) consumindo uma fila de diretórios. Cada worker faz `os.ReadDir` (no Windows já traz tamanho e atributos), soma os arquivos localmente e enfileira os subdiretórios.
- Top-N arquivos (N = 1000) e histograma de extensões por worker, mesclados ao final. O único lock é na criação de nó de diretório.
- Caminhos com prefixo `\\?\` (suporte a caminhos longos).
- `Progress` com contadores atômicos (arquivos, diretórios, bytes, caminho de amostra).

**Tratamento de casos do Windows:**
- **Reparse points** (junctions/symlinks): não seguidos; o nó é registrado com flag `link` e tamanho 0.
- **Arquivos na nuvem** (`FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS`, `FILE_ATTRIBUTE_RECALL_ON_OPEN`, `FILE_ATTRIBUTE_OFFLINE`): contam 0 bytes em disco, flag `cloud`. Só leitura de atributos, nunca conteúdo (não dispara download).
- **Acesso negado:** a varredura continua; contador + até 100 caminhos guardados para a UI.
- **Hardlinks:** limitação conhecida do WalkScanner (podem contar em dobro, principalmente em `WinSxS`); documentada no README; resolvida no MFTScanner.

**Árvore (`tree`):**
- Arena `[]Node` com índices `int32`. `Node`: nome, pai, filhos, tamanho próprio (arquivos diretos), tamanho total, nº de arquivos (total), mtime mais recente, flags (`link`, `cloud`, `denied`).
- Tamanho total e contagens calculados numa passada pós-ordem ao fim da varredura.
- `Result`: árvore, top-N arquivos, histograma de extensões, erros, duração, motor usado.

**MFTScanner (fase 2, fora do MVP):** `FSCTL_ENUM_USN_DATA` + leitura de registros da MFT; só admin + NTFS; alimenta o mesmo `Builder`. Seleção automática: admin + NTFS → MFT; caso contrário → Walk.

## 4. Motor de regras

**Fonte:** `internal/rules/builtin/*.yaml` embutidos via `go:embed`, um arquivo por ecossistema; regras do usuário em `%LOCALAPPDATA%\NodeGarbage\rules.d\*.yaml`.

**Schema da regra:**
```yaml
- id: node.node_modules          # único, obrigatório
  name: "node_modules"
  category: build-artifact       # dev-cache | build-artifact | pkg-cache | temp | browser-cache | system | user-data
  match:                         # exatamente um de: path | dirname | glob
    dirname: node_modules
    sibling: package.json        # opcional: arquivo/pasta que deve existir no diretório pai
  verdict: safe                  # safe | caution | keep
  regenerable: "npm install"     # opcional, texto exibido
  action: delete_dir             # delete_dir | delete_contents | manual
  command: ""                    # obrigatório se action = manual; exibido, nunca executado
  explain: "texto em PT-BR"
```
- `path`: absoluto, com variáveis de ambiente (`%LOCALAPPDATA%`, `%TEMP%`, `%USERPROFILE%`…) expandidas e comparação sem diferenciar maiúsculas.
- `glob`: casa arquivos dentro do diretório avaliado (ex.: `*.dmp`), com campo opcional `older_than_days`.
- Regras inválidas (schema, id duplicado) em `rules.d` são ignoradas e relatadas na UI; regras embutidas inválidas quebram os testes.

**Precedência:**
1. **Lista de proteção fixa no código**, que nenhum YAML sobrescreve: `%SystemRoot%`, `Program Files`, `Program Files (x86)`, `ProgramData\Microsoft`, `System Volume Information`, `$Recycle.Bin`, `pagefile.sys`, `hiberfil.sys`, `swapfile.sys`, raízes de volume, `%USERPROFILE%` (a pasta em si), `Documents`, `Desktop`, `Pictures`, `Videos`, `Music`, `Downloads` (a pasta em si) e qualquer diretório `.git`. Mais caminhos protegidos extras definidos pelo usuário. Resultado: `keep`, `source: protected`.
2. Regras com `path`.
3. Regras com `dirname` / `glob`; empate resolvido pela mais específica (com `sibling` > sem `sibling`), depois pela ordem de carregamento.

**Sub-árvore consumida:** um nó que casa com `delete_dir` não tem os filhos reclassificados.

**Projeto inativo:** para regras de categoria `build-artifact`, se o mtime mais recente do diretório pai (excluindo o próprio artefato) tiver mais de 90 dias, o nó recebe a flag `inactive`.

**Regras iniciais (MVP):** npm/pnpm/yarn/bun (node_modules, caches globais); Go (`go-build`, `pkg/mod` → manual `go clean -modcache`); Rust (`target/` com `Cargo.toml`, `~/.cargo/registry`); .NET (`bin/`, `obj/` com `*.csproj`/`*.sln`, cache do NuGet); Python (`__pycache__`, `.venv` com `pyproject.toml`/`requirements.txt` → caution, caches do pip/uv/poetry); JVM (`.gradle/caches`, `build/` com `build.gradle*`, `.m2/repository` → caution); Android (`.android/avd` → caution, SDK → keep/manual); Docker/WSL (`docker_data.vhdx`, `ext4.vhdx` → caution/manual com comandos); `%TEMP%` e `%SystemRoot%\Temp` (delete_contents); caches do Chrome/Edge/Firefox; caches do VS Code e JetBrains; crash dumps (`*.dmp`, `CrashDumps`); `SoftwareDistribution\Download` (manual: Limpeza de Disco).

**Saída por nó:** `{ruleId?, verdict, category, action, inactive, source: protected|rule|ai|unknown}`. Candidatos à IA: nós `unknown` com tamanho ≥ 500 MB (configurável).

## 5. Integração com IA

**Interface:**
```go
type Provider interface {
    Analyze(ctx context.Context, p Payload) (Analysis, error)
}
```
- **openaicompat:** `POST {baseURL}/chat/completions`; tenta `response_format: json_schema`; fallback para `json_object`; último fallback, extração do primeiro objeto JSON do texto.
- **anthropic:** `POST https://api.anthropic.com/v1/messages` com tool use para a saída estruturada.
- **Perfis:** nome, tipo, base URL, modelo; chave no Windows Credential Manager (`secrets`), nunca em arquivo nem em log. Chave opcional (Ollama/LM Studio).

**Payload:**
- Resumo dos volumes (total, usado, livre, FS).
- Até 200 maiores itens classificados por regra (caminho, tamanho, nº de arquivos, mtime, ruleId, verdict).
- Até 150 maiores itens `unknown` (mesmos campos + top 5 extensões).
- Top 30 extensões e top 50 arquivos.
- Formato compacto (colunas + linhas). Orçamento de ~30k tokens (estimativa: caracteres/4); se exceder, remove primeiro os menores itens de cada lista.
- Instruções do sistema em PT-BR, pedindo explicação amigável para devs e o schema de resposta.

**Privacidade:** o segmento do nome de usuário em `C:\Users\<nome>\` vira `<user>`; termos extras configuráveis são mascarados (`<redacted>`). Nada de conteúdo de arquivos. `PreviewAIPayload` mostra o JSON exato e a estimativa de tokens; o envio exige clique explícito.

**Resposta:**
```json
{ "items": [{ "path": "...", "verdict": "safe|caution|keep", "reason": "...", "confidence": 0.0 }],
  "report_markdown": "..." }
```

**Validação e confiança:**
- Itens com `path` fora do payload são descartados (os caminhos são comparados na forma anonimizada e mapeados de volta para os reais localmente).
- `source: protected` nunca muda.
- Para itens `unknown`, o veredito da IA é aplicado com `source: ai`; aparece como "sugerido pela IA" e nunca vem pré-selecionado.
- Para itens com regra, se a IA divergir, os dois pareceres são exibidos e o veredito efetivo é o mais conservador (`keep` > `caution` > `safe`).

**Robustez:** timeout de 120 s; 2 retentativas com backoff exponencial em 429/5xx/erro de rede; erros mapeados para mensagens claras (chave inválida, modelo inexistente, limite de requisições, resposta inválida); cancelável.

**Cache:** chave `sha256(payload canônico + tipo do provedor + modelo)` no SQLite; "Reanalisar" ignora o cache.

## 6. Exclusão segura (cleaner)

**Duas etapas:**
1. `PlanClean(paths, mode)`, para cada caminho:
   - existe; canonizado com `GetFinalPathNameByHandle`;
   - não é protegido, nem está dentro de protegido, nem é ancestral de protegido. Exceção para `.git`: um diretório que contém `.git` pode ser apagado **somente** se ele próprio casou com uma regra `delete_dir` (ex.: `node_modules` com pacotes clonados via git); caso contrário, conter `.git` bloqueia o item (protege repositórios selecionados manualmente);
   - pertence a um volume varrido nesta sessão ou com snapshot salvo;
   - modo Lixeira: o volume tem Lixeira (não removível nem de rede) e o tamanho do item cabe na capacidade restante da Lixeira (`SHQueryRecycleBin` + cota configurada). Se não couber, o item é bloqueado nesse modo, com sugestão de modo permanente.
   - Retorna o plano: itens aceitos, itens recusados com motivo, total, avisos (ex.: muitos arquivos para a Lixeira) e um token aleatório de 32 bytes, de uso único, com validade de 5 minutos, amarrado ao `planID` e ao hash dos itens.
2. **Diálogo de confirmação:** lista, total, modo; checkbox individual para cada item `caution`; o modo permanente exige marcar "Entendo que isso não pode ser desfeito".
3. `ExecuteClean(planID, token)`: valida o token e executa somente os itens do plano.

**Lixeira:** `IFileOperation` (COM) com `FOFX_RECYCLEONDELETE | FOF_NOCONFIRMATION | FOF_SILENT | FOF_NOERRORUI | FOF_WANTNUKEWARNING`.

**Permanente:** remoção concorrente com caminhos `\\?\`; remove o atributo somente-leitura antes de apagar; arquivos em uso são pulados e reportados; `delete_contents` mantém a pasta raiz.

**Execução:** progresso por eventos; cancelamento entre itens; resultado por item (`ok | skipped | error`, motivo, bytes liberados).

**Registro:** cada item vai para a tabela `cleanups` (caminho, tamanho, modo, source/ruleId, data, resultado).

**Elevação:** a lista de proteção vale igualmente com e sem admin.

## 7. Persistência (store)

SQLite em `%LOCALAPPDATA%\NodeGarbage\nodegarbage.db` (`modernc.org/sqlite`), migrações versionadas embutidas.

| Tabela | Conteúdo |
|---|---|
| `snapshots` | volume, data, motor, duração, totais, erros |
| `snapshot_nodes` | snapshot_id, node_id, parent_id, nome, tamanhos, contagens, mtime, flags, classificação |
| `snapshot_top_files` / `snapshot_extensions` | top-N arquivos e histograma |
| `cleanups` | histórico de exclusões |
| `ai_reports` | chave de cache, provedor, modelo, data, resposta JSON |
| `settings` | chave/valor (perfis de IA sem a chave, termos de anonimização, caminhos protegidos extras, limites) |

Apenas o snapshot mais recente por volume é mantido; o anterior é apagado na mesma transação em que o novo é gravado.

## 8. Interface

**Janela:** mínima de 1024×680; Mica no Windows 11 (`BackdropType` Mica + WebView transparente); no Windows 10, fundo sólido `#F0F0F0`. Barra lateral de navegação + área de conteúdo.

**Telas:**
1. **Início:** cards de volume (barra de uso: azul = usado, laranja = lixo estimado), FS, livre, última varredura, checkbox; banner de "varredura limitada" sem admin.
2. **Varredura:** contadores ao vivo, velocidade, caminho atual (mono), Cancelar; `aria-live="polite"`.
3. **Resultados:** resumo no topo (seguro / cuidado) + chips de categoria como filtros; lista/árvore virtualizada ordenada por tamanho (checkbox, veredito com ícone + rótulo, origem, "projeto inativo"); treemap da pasta atual com breadcrumb; painel de detalhes (explicação, comando de regeneração, parecer da IA).
4. **Relatório da IA:** pré-visualização do payload → relatório renderizado; itens citados levam à linha nos Resultados.
5. **Confirmação de limpeza:** `<dialog>` modal (`showModal()`), foco preso, Esc fecha, foco restaurado ao fechar.
6. **Histórico:** total liberado + tabela.
7. **Configurações:** perfis de IA ("Testar conexão"), termos de anonimização, caminhos protegidos extras, limites, pasta de regras do usuário.

**Design tokens (CSS custom properties; componentes nunca usam hex direto):**

| Token | Valor | Uso |
|---|---|---|
| `--surface` | `#F0F0F0` | fundo base |
| `--glass` | `rgba(255,255,255,.55)` + `blur(20px)` + borda `rgba(255,255,255,.6)` 1px | painéis |
| `--accent` | `#105BFF` (≈4,7:1 sobre `#F0F0F0`; texto branco ≈5,3:1) | ação primária, foco, seleção |
| `--accent-hover` | tom mais escuro de `#105BFF` | hover/pressed |
| `--warn` | `#FE8500` (só preenchimento/ícone; texto por cima escuro) | "cuidado", barra de lixo |
| `--warn-text` | `#B35C00` | texto laranja |
| `--safe` / `--keep` | verde e cinza neutro de apoio (≥4,5:1) | vereditos |
| `--text-primary` / `--text-muted` | quase preto / cinza escuro | texto |

- Raios: 6 / 10 / 14 px. Espaçamento: escala de 4 px. Sombra mínima.
- Tipografia: Segoe UI Variable; Cascadia Mono para caminhos e tamanhos; `font-variant-numeric: tabular-nums`.
- Estado nunca indicado só por cor (ícone + rótulo sempre).

**Implementação:**
- Treemap: `d3-hierarchy` (squarify) + Canvas; a lista é a alternativa acessível e completa.
- Stores Svelte por domínio: `volumes`, `scan`, `results`, `selection`, `ai`, `clean`, `settings`. Telas como containers; componentes apresentacionais com menos de 200 linhas.
- Toda tela com estados vazio, carregando (skeleton) e erro com ação.
- Teclado: setas na árvore, Espaço marca, Enter abre; tudo alcançável por Tab.
- `prefers-reduced-motion` respeitado; fallback de fundo sólido se `backdrop-filter` não estiver disponível.
- Markdown da IA: `marked` + `DOMPurify`, sem HTML bruto e sem esquemas de URL além de `http`/`https`.
- Textos em PT-BR centralizados em `frontend/src/lib/strings.ts`.
- Testes de layout em 1024, 1366, 1920 e 2560 px e escalas de 100–200%.

## 9. Testes

- **scanner/tree:** árvores sintéticas em `t.TempDir()` (junctions via `mklink /J`, somente-leitura, caminhos > 260 caracteres); atributos de nuvem/negado via filesystem injetado; benchmark com ~500 mil arquivos sintéticos.
- **rules:** testes de tabela por regra embutida; teste garantindo que nenhum YAML libera caminho protegido; validação de schema.
- **ai:** `httptest.Server` para os dois provedores (válido, JSON malformado, caminhos alucinados, 429/5xx, timeout); anonimização; orçamento de tokens.
- **cleaner:** recusa de protegido/ancestral, junction para área protegida, token expirado/reutilizado/de outro plano, arquivo bloqueado, item maior que a Lixeira, volume sem Lixeira. Sempre em `TempDir`.
- **store:** banco temporário, migrações, substituição de snapshot, cache.
- `go test -race ./...`.
- **Frontend:** `svelte-check`; Vitest + Testing Library (estados, teclado na árvore, fluxo do modal); sanitização com payloads XSS conhecidos.

## 10. Build, CI e distribuição

- **CI (GitHub Actions, `windows-latest`), em cada PR:** `go vet`, `staticcheck`, `go test -race`, lint + testes do frontend, `wails build`.
- **Release (tag `vX.Y.Z`):** `.exe` portátil + instalador NSIS (`wails build -nsis`, com bootstrap do WebView2), `SHA256SUMS`, publicação no GitHub Releases. SemVer.
- **Repositório:** `README.md` (inglês) + `README.pt-BR.md`, `CONTRIBUTING.md` (com "Como adicionar uma regra"), `SECURITY.md`, `CODE_OF_CONDUCT.md`, `LICENSE` (MIT), templates de issue (bug, nova regra) e de PR. Zero telemetria, declarado no README.

## 11. Fora do escopo do MVP (v0.1)

Motor MFT (fase 2); tema escuro; histórico comparativo de varreduras; busca de arquivos individuais; execução de comandos externos (ex.: `docker system prune`); assinatura de código (SignPath Foundation) e winget; i18n; provedores de IA nativos além de OpenAI-compatível e Anthropic; suporte a outros sistemas operacionais.
