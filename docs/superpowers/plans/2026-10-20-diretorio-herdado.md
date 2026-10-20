# Diretório herdado: plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Um terminal novo (`p`, `g`, `t`) abre no diretório do painel com foco, em vez do diretório do último attach.

**Architecture:** Uma função, `App.terminalDir(t *tab) string`, aplica a regra a partir do painel com foco da tab dada. Os shells já dizem o diretório (`directoryTeller`, via `/proc`); `runView` passa a dizer onde a execução começou. `startTerminal` usa a regra para `p` e `g`; `openNewTab` lê o diretório antes de criar a tab, para `t` herdar da tab que se deixa.

**Tech Stack:** Go, pacote `internal/app`, testes com views falsas (`fakeView`, `recordLaunches`).

**Spec:** `docs/polimento.md`, seção "1. Terminal novo no diretório do painel com foco".

## Global Constraints

- Sem opção no config: é o comportamento padrão (CONTRIBUTING §2).
- Regra, a primeira que der resposta: terminal → diretório atual do shell; execução de script ou passo de fluxo → diretório em que a execução começou; painéis sem diretório e home → `~`; terminal cujo diretório não se lê (macOS) → `a.workDir`.
- Um diretório que não existe mais não é usado.
- Testes do app trocam `App.launch` por views falsas; nada de processos reais.
- Código, comentários e textos de UI em inglês; ADR e `tui.md` em português; `CHANGELOG.md` em inglês, em `### Changed`, com as teclas como `prefix` + `p`.
- `make check` passa antes de cada commit.

## Review Focus

1. **Pasta apagada:** no Linux, `readlink /proc/<pid>/cwd` de uma pasta apagada devolve `"/caminho (deleted)"`. O shell novo não pode receber esse caminho; abre em `~`. Teste na Task 1.
2. **Painel flutuante com foco:** `g` sobre um flutuante, ou `p` com um flutuante em foco, usa o diretório dele (`t.focused` é o flutuante). Teste na Task 1.
3. **`$HOME` vazio:** `os.UserHomeDir` falha; o terminal abre em `a.workDir`, não numa string vazia (que seria o diretório do daemon). Teste na Task 1.
4. **Shell que já saiu, ainda na tab:** `Process.Dir()` devolve `""` depois da saída; cai em `a.workDir`, como um shell ilegível. Coberto pelo teste do diretório ilegível na Task 1.
5. **`t` a partir da home:** sem tab anterior, abre em `~`. Teste na Task 2.

---

### Task 1: `p` e `g` no diretório do painel com foco

**Files:**
- Modify: `internal/app/workspace.go` (`startTerminal`, mais `terminalDir` e `homeDir`)
- Modify: `internal/app/run_view.go` (campo `dir`, método `Dir`, parâmetro novo em `newRunView`)
- Modify: `internal/app/scripts.go:208` e `internal/app/pane_request.go:~47` (passam o `dir` a `newRunView`)
- Create: `internal/app/terminal_dir_test.go`
- Modify: `docs/decisoes.md` (ADR-048), `docs/tui.md:26`, `docs/codemap.md` (linha de `workspace.go`), `CHANGELOG.md`

**Interfaces:**
- Consumes: `directoryTeller` (`sessions.go:276`), `existingDir(dir, fallback string) string` (`sessions.go:390`), `a.shells` (um painel é shell quando `a.shells[id] != nil`).
- Produces:
  - `func (a *App) terminalDir(t *tab) string`: o diretório de um terminal novo aberto a partir do painel com foco de `t`; `t` pode ser `nil` ou vazia.
  - `func homeDir(fallback string) string`: `os.UserHomeDir()`, ou `fallback` quando falha ou volta vazio.
  - `func newRunView(name string, p *palette, start runStarter, close func(), size pty.Size, dir string) (*runView, error)` e `func (v *runView) Dir() string`, que devolve o `dir` da criação.

- [ ] **Step 1: Escrever os testes que falham** em `internal/app/terminal_dir_test.go`.

  Helper:

  ```go
  // dirView is a terminal that tells the folder its shell is in.
  type dirView struct {
  	fakeView
  	dir string
  }

  func (v *dirView) Dir() string { return v.dir }
  ```

  Cada teste monta a tab com `newTab(1, view)`, `a := testApp(t, tab)`, marca o shell com `a.shells[1] = &shell{}` quando o painel é um terminal, faz `t.Setenv("HOME", home)` com `home := t.TempDir()`, chama `l := recordLaunches(a)` e confere `l.last(t).dir`:

  | Teste | Painel com foco | Ação | `dir` esperado |
  |---|---|---|---|
  | `TestNewTerminalOpensInTheFocusedShellsFolder` | shell `&dirView{dir: d}`, `d := t.TempDir()` | `a.openPanel(a.startTerminal)` | `d` |
  | `TestNewTerminalBesideARunOpensWhereTheRunStarted` | `newRunView("run", &a.palette, start, func() {}, pty.Size{Cols: 80, Rows: 24}, d)`, com `start` devolvendo `&fakeView{}` | `a.openPanel(a.startTerminal)` | `d` |
  | `TestNewTerminalBesideAPanelWithoutAFolderOpensAtHome` | `&fakeView{}` sem shell | `a.openPanel(a.startTerminal)` | `home` |
  | `TestNewTerminalOnTheHomeScreenOpensAtHome` | sem tabs (`testApp(t)`) | `a.openPanel(a.startTerminal)` | `home` |
  | `TestNewTerminalBesideAShellWhoseFolderCannotBeReadOpensInTheAttachFolder` | shell `&dirView{dir: ""}` | `a.openPanel(a.startTerminal)` | `a.workDir` |
  | `TestNewTerminalBesideAShellInADeletedFolderOpensAtHome` | shell `&dirView{dir: gone + " (deleted)"}`, `gone` um `t.TempDir()` removido com `os.Remove` | `a.openPanel(a.startTerminal)` | `home` |
  | `TestFloatingTerminalOpensInTheFocusedShellsFolder` | shell `&dirView{dir: d}` | `a.openFloating()` | `d` |
  | `TestNewTerminalBesideAFocusedFloatingPaneUsesItsFolder` | tile `&fakeView{}`; depois `a.openFloating()` com `a.launch` devolvendo `&dirView{dir: d}`, e `a.shells` com o id do flutuante | `a.openPanel(a.startTerminal)` | `d` |
  | `TestNewTerminalWithoutAHomeFolderOpensInTheAttachFolder` | `&fakeView{}` sem shell, `t.Setenv("HOME", "")` | `a.openPanel(a.startTerminal)` | `a.workDir` |

  No penúltimo, o `dirView` precisa ser devolvido pelo `a.launch` e registrado em `a.shells` pelo próprio `startShell`; depois troque `a.launch` por `recordLaunches(a)` antes do `openPanel`.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ -run 'TestNewTerminal|TestFloatingTerminal'`
  Expected: FAIL. Não compila por causa do parâmetro novo de `newRunView`, ou os casos de diretório devolvem `a.workDir`.

- [ ] **Step 3: `runView` diz onde começou**

  Campo `dir string` em `runView`, preenchido por `newRunView`, e `func (v *runView) Dir() string { return v.dir }`, com o comentário de que é o diretório em que a execução começou e não o atual do programa. Nas chamadas, `openTrackedRun` (`scripts.go:208`) passa `p.dir`, e `pane_request.go` passa `orDefault(req.Dir, a.workDir)`, que é o que o `program` dele já usa (calcule uma vez e use nos dois).

  Isso não muda as sessões: `savedPane` só olha `directoryTeller` em shells (`a.shells[id] != nil`), e execuções não são shells.

- [ ] **Step 4: `terminalDir` e `homeDir` em `workspace.go`; `startTerminal` usa `a.terminalDir(a.currentTab())`**

  Ordem dentro de `terminalDir`:
  1. sem tab ou tab vazia → `homeDir(a.workDir)`;
  2. a view com foco implementa `directoryTeller` e `Dir()` não é vazio → `existingDir(dir, homeDir(a.workDir))`;
  3. a view com foco é um shell (`a.shells[t.focused] != nil`), com `Dir()` vazio → `a.workDir`;
  4. senão → `homeDir(a.workDir)`.

  O teste da pasta apagada passa pelo `existingDir`, porque `"… (deleted)"` não existe. `openFloating` já chama `startTerminal` antes de mudar o foco, e `openPanelBeside` chama o `start` antes de `t.focus(id)`, então o painel com foco ainda é o de onde se partiu. Não mexa nessa ordem.

- [ ] **Step 5: Rodar e ver passar**

  Run: `go test -race ./internal/app/`
  Expected: PASS, inclusive os testes de sessões (`sessions_test.go`, `rerun_test.go`), que abrem terminais em tabs vazias.

- [ ] **Step 6: Documentação no mesmo commit**

  - `docs/decisoes.md`: `## ADR-048 — Terminal novo no diretório do painel com foco`, no formato contexto → decisão → consequências. Contexto: o diretório do attach muda conforme de onde se reconectou, e um `cd` não valia para o terminal seguinte; é polimento da fase 5, antecipado pelo mantenedor. Decisão: a regra das Global Constraints, sem opção no config; execuções guardam onde começaram; uma pasta que sumiu cai em `~`. Consequências: no macOS o diretório não se lê (`/proc`), então lá continua o do attach; um shell rodando `ssh` dá o diretório local; o `t` vem na Task 2.
  - `docs/tui.md:26`: troque "no diretório em que o `noct` foi aberto" por "no diretório do painel com foco: o atual do shell, ou onde a execução começou; ao lado de painéis sem diretório e na home, em `~`; e, onde o diretório de um shell não se lê (macOS), no diretório de onde se fez o attach (ADR-048)".
  - `docs/codemap.md`: na linha de `internal/app/workspace.go`, acrescente "the folder a new terminal opens in (`terminalDir`)".
  - `CHANGELOG.md`, em `## Unreleased`, um `### Changed` antes do `### Fixed`:

    ```markdown
    - New terminals (`prefix` + `p`, `prefix` + `g`) open in the folder of the focused panel: where its shell is now, or where its script ran. Beside panels without a folder, such as scripts or logs, they open in your home folder.
    ```

- [ ] **Step 7: Gate e commit**

  Run: `make check`
  Expected: gofmt, vet e testes com `-race` sem erro.

  ```bash
  git add internal/app/workspace.go internal/app/run_view.go internal/app/scripts.go internal/app/pane_request.go internal/app/terminal_dir_test.go docs/decisoes.md docs/tui.md docs/codemap.md CHANGELOG.md
  git commit -m "Open new terminals in the focused panel's folder"
  ```

### Task 2: `t` herda da tab que se deixa

**Files:**
- Modify: `internal/app/panel_kinds.go` (`openNewTab`)
- Modify: `internal/app/terminal_dir_test.go`
- Modify: `docs/decisoes.md` (ADR-048), `docs/tui.md:26`, `CHANGELOG.md`, `docs/polimento.md`

**Interfaces:**
- Consumes: `a.terminalDir(t *tab) string` e `a.startShell(id, frame, dir)` da Task 1.
- Produces: nada novo.

- [ ] **Step 1: Escrever os testes que falham**

  | Teste | Antes | Ação | `dir` esperado |
  |---|---|---|---|
  | `TestNewTabOpensInTheFolderOfThePanelLeftBehind` | tab com shell `&dirView{dir: d}`, `a.config.NewTab` no padrão (terminal) | `a.openNewTab()` | `d`, e `len(a.session.tabs) == 2` |
  | `TestNewTabFromTheHomeScreenOpensAtHome` | sem tabs, `HOME` num `t.TempDir()` | `a.openNewTab()` | `home` |

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ -run TestNewTab`
  Expected: FAIL no primeiro: `dir` é `home`, porque a tab nova já está vazia quando o terminal começa.

- [ ] **Step 3: Em `openNewTab`, ler o diretório antes de `a.openEmptyTab()`**

  `dir := a.terminalDir(a.currentTab())` antes de abrir a tab vazia. Quando o tipo escolhido é `config.NewTabTerminal`, o starter vira `func(id layout.PaneID, frame image.Rectangle) (view, error) { return a.startShell(id, frame, dir) }`; os outros tipos continuam com `kind.start(a)`.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/app/`
  Expected: PASS.

- [ ] **Step 5: Documentação**

  - ADR-048: a decisão passa a incluir `t` ("uma tab nova herda do painel com foco da tab que se deixa"); tire o "o `t` vem na Task 2" das consequências.
  - `docs/tui.md:26`: ajuste o texto para dizer que vale também para a tab nova.
  - `CHANGELOG.md`: a mesma entrada passa a citar `prefix` + `t`: "New terminals (`prefix` + `p`, `prefix` + `t`, `prefix` + `g`) open in …".
  - `docs/polimento.md`: o estado do item 1 vira "Feito (ADR-048)", e a seção 1 passa a apontar para o `tui.md` em vez de repetir a regra.

- [ ] **Step 6: Gate e commit**

  Run: `make check`
  Expected: sem erro.

  ```bash
  git add internal/app/panel_kinds.go internal/app/terminal_dir_test.go docs/decisoes.md docs/tui.md CHANGELOG.md docs/polimento.md
  git commit -m "Open a new tab's terminal in the folder of the panel left behind"
  ```
