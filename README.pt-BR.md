# NodeGarbage

[English](README.md) | **Português (BR)**

O NodeGarbage é um aplicativo desktop gratuito e de código aberto (MIT) para limpeza de disco no Windows, feito para desenvolvedores e power users. Ele varre os seus discos, classifica os arquivos e caches que mais ocupam espaço, explica o que pode ou não ser apagado com segurança e apaga somente o que você confirmar.

<!-- screenshot: docs/screenshot.png (to be added) -->

O foco são caches e artefatos de desenvolvimento (`node_modules`, caches de gerenciadores de pacotes, saídas de build, discos do Docker/WSL, caches de IDEs), sem esquecer os caches gerais do Windows e dos navegadores.

## Principais recursos

- **Varredura rápida de diretórios** de qualquer volume fixo: percurso concorrente, suporte a caminhos longos (`\\?\`), progresso ao vivo, os 1000 maiores arquivos e histograma de extensões.
- **Regras embutidas** para Node, Python, Rust, Go, JVM, .NET, Android, contêineres, navegadores, editores e Windows, cada uma com veredito (`safe`, `caution`, `keep`), explicação e o comando que regenera os dados.
- **Caminhos protegidos** (pastas do sistema, raízes de disco e similares) que nunca podem ser limpos, com ou sem administrador.
- **Tela de resultados** com árvore ordenável, treemap da pasta atual, filtros por categoria, detecção de "projeto inativo" (90 dias) e painel de detalhes.
- **Relatório de IA opcional** que explica os itens que as regras não reconhecem (veja [Privacidade](#privacidade)).
- **Limpeza segura**: diálogo de confirmação, Lixeira por padrão (exclusão permanente é opcional e exige confirmação extra), resultado por item e histórico de limpezas.
- **Suas próprias regras**: coloque arquivos YAML em `%LOCALAPPDATA%\NodeGarbage\rules.d\`.
- Funciona 100% sem IA. Abre sem administrador e oferece "reiniciar como administrador" para uma varredura mais completa.

## Só você apaga: a IA e as regras apenas aconselham

Regras e IA classificam e explicam; nunca apagam. O único caminho de código que remove arquivos é a sua confirmação explícita: o app monta um plano de limpeza, mostra em um diálogo e só um token de uso único (32 bytes, válido por 5 minutos) emitido para aquele plano exato pode executá-lo. Nada vem pré-selecionado pela IA, a IA não tem ferramentas nem acesso ao app, e a resposta dela é tratada como dado não confiável: caminhos fora do que foi enviado são descartados, itens protegidos nunca mudam e, quando IA e regra divergem, vale o veredito mais conservador.

## Privacidade

O NodeGarbage tem **zero telemetria**. O único tráfego de rede é a análise de IA que você mesmo dispara, para o provedor que você configurou.

O que vai para a IA, somente quando você clica em enviar:

- Caminhos, tamanhos, contagens de arquivos, datas de modificação e extensões (até 200 itens classificados, 150 desconhecidos, 30 extensões, 50 arquivos, cerca de 30.000 tokens), mais os totais dos volumes.
- **Nenhum conteúdo de arquivo**, nunca.
- Seu nome de usuário do Windows é anonimizado (`C:\Users\<nome>\` vira `C:\Users\<user>\`). Limitação conhecida: nomes de usuário com menos de 4 caracteres só são substituídos quando formam um componente inteiro do caminho.
- Você pode adicionar **termos de redação** (por exemplo, o nome de um cliente ou projeto); toda ocorrência é mascarada como `<redacted>`.
- **Pré-visualização exata**: o app mostra o JSON exato e uma estimativa de tokens antes de enviar. Nada é enviado sem o seu clique.
- As chaves de API ficam no **Gerenciador de Credenciais do Windows**, nunca em arquivos, no banco de dados ou em logs. A chave é opcional (Ollama, LM Studio).

## Provedores de IA suportados

- **Compatível com OpenAI** (qualquer base URL): OpenAI, Ollama, LM Studio, OpenRouter, Azure OpenAI e outros que exponham `/chat/completions`.
- **Anthropic** (API Messages nativa).

## Instalação

Baixe a versão mais recente em [Releases](https://github.com/engviniciusbs/nodegarbage/releases):

- `nodegarbage.exe`: executável portátil.
- `nodegarbage-amd64-installer.exe`: instalador NSIS (instala o WebView2 se necessário).
- `SHA256SUMS`: checksums para conferir os downloads.

Os binários **ainda não são assinados digitalmente**, então o Windows SmartScreen exibirá "O Windows protegeu o computador". Clique em "Mais informações" e depois em "Executar assim mesmo", depois de conferir o SHA256 com o `SHA256SUMS`. Requer Windows 10 ou 11 com o runtime do WebView2.

## Compilar a partir do código

Requisitos: Go 1.27, Node.js (LTS) e a [CLI do Wails v2](https://wails.io/docs/gettingstarted/installation).

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
npm ci --prefix frontend
wails build
```

O executável é gerado em `build/bin/`. Use `wails build -nsis` para o instalador (requer o [NSIS](https://nsis.sourceforge.io/)) e `wails dev` para desenvolvimento.

## Executar os testes

```powershell
wails generate module            # gera frontend/wailsjs, necessário para o svelte-check
go vet ./...
go test ./...
go test -race ./...              # requer CGO e gcc
npm --prefix frontend run check
npm --prefix frontend test -- --run
```

Defina `CI_NO_RECYCLE=1` para pular os testes que usam a Lixeira real.

## Limitações conhecidas

- O motor de varredura por diretórios **conta hardlinks em dobro** (principalmente em `WinSxS`), então algumas pastas do sistema parecem maiores do que são.
- **Ainda não há motor MFT** (previsto para a v0.2); a varredura é feita por diretórios.
- O visual no Windows 10 (fundo sólido no lugar do Mica) **não foi testado**.
- A Lixeira **não está disponível em discos removíveis e de rede**.
- **Placeholders de nuvem** (OneDrive e similares, não baixados) contam 0 byte e nunca são lidos nem baixados.
- Os textos da interface estão apenas em português do Brasil (ainda sem i18n).

## Roadmap

Motor de varredura MFT (v0.2); tema escuro; comparação de histórico de varreduras; busca de arquivos individuais; execução de comandos externos (por exemplo, `docker system prune`); assinatura de código (SignPath Foundation) e winget; internacionalização; mais provedores de IA nativos; outros sistemas operacionais.

## Contribuir e segurança

Veja [CONTRIBUTING.md](CONTRIBUTING.md) (em inglês), [SECURITY.md](SECURITY.md) e o [Código de Conduta](CODE_OF_CONDUCT.md).

## Licença

[MIT](LICENSE)
