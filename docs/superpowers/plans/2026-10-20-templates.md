# Templates de tab: plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `prefixo` + `R` grava a tab atual como template (layout, tipos, nomes, cores e o comando de cada terminal com `run`/`ask`/`none`), e `prefixo` + `T` abre uma tab nova a partir de um template, escolhido numa lista com a miniatura do layout.

**Architecture:** Um pacote novo, `internal/templates`, define o arquivo YAML, a conversão de e para `layout.Shape` e a pasta. O `app` monta o template a partir da tab (como `savedTab`) e abre a tab a partir dele (como `restoreTab`), com o diretório herdado da ADR-048 e a caixa `run?` da ADR-046.

**Tech Stack:** Go, `go.yaml.in/yaml/v3` (já no `go.mod`), `internal/layout`.

**Spec:** `docs/templates.md`; teclas em `docs/polimento.md`, seção "Teclas novas".

**Depende de:** o plano de cores por painel (o `colorSet` e as chaves de cor) e o plano de temas (o palco), já implementados.

## Global Constraints

- Um template é uma tab. Painéis que as sessões não reabrem (execuções, editores, ajuda, flutuantes) ficam de fora, e o layout se fecha sobre eles.
- `start`: `run` digita o comando no shell ao abrir; `ask` abre com a caixa `run?` (o `rerun` da ADR-046); `none` só abre o shell. O padrão ao gravar é `ask` para um terminal com comando.
- Os terminais abrem em `a.terminalDir(<tab de onde se abriu>)` (ADR-048).
- Arquivos em `<os.UserConfigDir()>/noct/templates/<nome>.yaml`; nome com a regra de scripts e fluxos.
- Teclas: `T` (`[]string{"T", "shift+t"}`, abrir, funciona também na home) e `R` (`[]string{"R", "shift+r"}`, gravar, precisa de tab), no contexto `commands`, trocáveis nos binds e como `Alt+Shift+T`/`Alt+Shift+R`.
- Mesmas regras de idioma, CHANGELOG, codemap e `make check` dos outros planos.

## Review Focus

1. **Template com um script que não existe mais, ou um tipo desconhecido no arquivo:** o painel fica de fora, a mensagem aparece na barra, e o resto da tab abre. Teste na Task 3.
2. **Comando com quebra de linha ou caracteres de controle** (lido do shell ou escrito à mão): não é digitado nem oferecido, como a ADR-046 já faz. Teste na Task 3.
3. **Gravar com um nome que já existe:** pergunta antes de sobrescrever, e `esc` mantém o arquivo antigo. Teste na Task 2.
4. **Tab com proporções ajustadas (resize) ou uma pilha:** o template guarda `share` e `stacked`, e a tab abre igual. Teste na Task 1.
5. **Arquivo apagado entre abrir a lista e apertar `enter`:** a mensagem aparece, e nada abre. Teste na Task 3.

---

### Task 1: pacote `internal/templates`

**Files:**
- Create: `internal/templates/templates.go`, `internal/templates/templates_test.go`
- Modify: `docs/codemap.md`

**Interfaces:**
- Consumes: `layout.Shape`, `layout.PaneID`, `layout.Direction` (`internal/layout/shape.go:10`).
- Produces:
  - `type Start string` com `Run = "run"`, `Ask = "ask"`, `None = "none"`.
  - `type Pane struct{ Kind, Name, Command string; Start Start; Colors map[string]string }`, com Kind `terminal`, `scripts`, `inspect` ou `logs`.
  - `type Template struct{ Name, Tab string; Colors map[string]string; Root Node }`, com `Name` vindo do arquivo e `Tab` o nome da tab.
  - `type Node struct{ Columns, Rows []Node; Stacked bool; Share float64; Pane *Pane }`: um nó tem `Pane` ou filhos, nunca os dois.
  - `func FromShape(s layout.Shape, panes map[layout.PaneID]Pane) Node` e `func (n Node) Shape(newID func() layout.PaneID) (layout.Shape, map[layout.PaneID]Pane)`.
  - `func Dir() (string, error)`, `func LoadAll(dir string) ([]Template, []error)` (ordenados pelo nome), `func Load(dir, name string) (Template, error)`, `func Save(dir string, t Template) error` (temporário e rename), `func Delete(dir, name string) error`, `func Rename(dir, from, to string) error`, `func CheckName(name string) error`.

  O YAML é o de `docs/templates.md`:
  - `name` (da tab), `colors`, `layout: columns|rows` e `panes:` no topo;
  - cada item de `panes` é `{terminal|scripts|inspect|logs: {name, run, start}}` com `colors`, `share` e `stacked` opcionais, ou `{columns|rows: [...]}` com `share`/`stacked`.

  `Share` só é gravado quando os irmãos não estão divididos por igual. A (de)serialização é feita com `yaml.Node` ou com tipos intermediários, e tipos desconhecidos são erro do arquivo.

- [ ] **Step 1: Escrever os testes que falham**

  - `TestAShapeBecomesATemplateAndBack`: `1 | (2 / 3)`, com o 2 e o 3 numa pilha e shares 0,6/0,4 na raiz. `FromShape` e `Shape` devolvem uma forma com as mesmas direções, `Stacked` e shares (tolerância de 0,001), e os painéis com o mesmo Kind/Name/Command/Start.
  - `TestTheFileReadsLikeTheDocs`: o exemplo de `docs/templates.md` (com `fastfetch` `run`, `claude` `ask` e `scripts` com `colors`) carrega sem erro, na forma esperada.
  - `TestEvenSharesAreLeftOut`: dois painéis por igual gravam sem `share`.
  - `TestABrokenTemplateIsReportedAndSkipped`: `{bogus: {}}`, um nó com `terminal` e `columns` ao mesmo tempo, e `start: soon`. Cada um dá um erro com o nome do arquivo, e os outros templates carregam.
  - `TestSaveRenameDelete`: `Save`, `Rename("a", "b")` (sem `a.yaml`, com `b.yaml`), `Delete("b")`; `Rename` para um nome que existe é erro.
  - `TestCheckNameUsesTheScriptRule`: `"dev"` ok; `""`, `"-x"` e `"a b"` são erro.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/templates/`
  Expected: FAIL, não compila.

- [ ] **Step 3: Implementar** conforme o Produces.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/templates/`
  Expected: PASS.

- [ ] **Step 5: Commit**

  `docs/codemap.md`: linha para `internal/templates/templates.go` ("Tab templates: the YAML file, layouts to and from it, and the templates folder").

  ```bash
  git add internal/templates docs/codemap.md
  git commit -m "Add the templates package: tab layouts as YAML files"
  ```

### Task 2: gravar com `R`

**Files:**
- Create: `internal/app/template_save.go`, `internal/app/template_save_test.go`
- Modify: `internal/app/keymap.go` (`save-template` em `commands`, contexto `save template`), `internal/app/keys.go` (modo, `commandShortcuts`, `actionKeys`, `runTabAction`), `internal/app/draw.go` (`drawModeOverlay`), `internal/app/textfield.go:133` (`typedField` para o nome)

**Interfaces:**
- Consumes: `templates.FromShape`, `templates.Save`, `templates.CheckName`, `templates.Load` (Task 1); `colorSet.saved()` (plano de cores); `commandTeller.ForegroundCommand()` e `a.shells[id].rerun` (como `savedPane` em sessions.go:285).
- Produces:
  - `type templateSave struct{ tab *tab; name textField; panes []savePane; selected int }` e `type savePane struct{ id layout.PaneID; kind, name, command string; start templates.Start }`.
  - `func (a *App) openTemplateSave()`: o nome começa com o nome da tab (ou vazio); os painéis em ordem de `tree.Panes()`, sem os que sessões não reabrem.
  - `func (a *App) handleTemplateSaveKey(key uv.KeyPressEvent)` e `func (a *App) drawTemplateSave(screen uv.Screen)`.
  - `func (a *App) templateOf(s *templateSave) templates.Template`.

- [ ] **Step 1: Escrever os testes que falham**

  - `TestSaveTemplateListsThePanesWithTheirCommands`: tab com shell (`commandView{command: "claude"}`), scripts panel e um `runView`. O popup tem 2 painéis (o run fica de fora): `terminal claude ask` e `scripts`.
  - `TestStartCyclesRunAskNone`: na linha do terminal, `→` passa por `none`, `run`, `ask`, e `←` volta. Um terminal sem comando não tem `start`.
  - `TestEnterSavesTheTemplate`: nome `dev` e `enter` gravam `dev.yaml`, com o layout da tab, as cores da tab e dos painéis, e `start: ask` no claude. O popup fecha com a mensagem `saved template dev`.
  - `TestSavingOverATemplateAsksFirst`: com `dev.yaml` existente, `enter` pergunta `replace template dev?`; `esc` mantém o arquivo antigo; `enter` troca.
  - `TestABadNameStaysInThePopup`: o nome `a b` mostra o erro e não grava.
  - `TestSaveTemplateNeedsATab`: na home, `prefixo` + `R` não faz nada.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ -run 'TestSaveTemplate|TestStartCycles|TestEnterSaves|TestSavingOver|TestABadName'`
  Expected: FAIL.

- [ ] **Step 3: Implementar**

  - **O popup:** a linha `name` (campo de texto) em cima, depois uma linha por painel: tipo, nome e comando cortado com `…`, e `◂ ask ▸` nos terminais com comando.
  - **Teclas:** `↑↓`/`jk` andam, `←→` mudam o `start`, `enter` grava e `esc` cancela.
  - **Comandos com caracteres de controle:** ficam sem comando, como em `restoreView`.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/app/`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/app
  git commit -m "Save the tab as a template with prefix + R"
  ```

### Task 3: abrir com `T`, com miniatura, e documentação

**Files:**
- Create: `internal/app/templates_popup.go`, `internal/app/templates_popup_test.go`
- Modify: `internal/app/stage.go` (`drawLayoutStage`), `internal/app/keymap.go` (`templates` em `commands`, contexto `templates`), `internal/app/keys.go` (modo, `commandShortcuts`, `actionKeys`, `runAction`), `internal/app/draw.go`, `internal/app/rerun.go` (`typeInView` reaproveitado)
- Modify: `docs/decisoes.md` (ADR-052), `docs/tui.md` (Tabs e painéis, Atalhos), `docs/templates.md` e `docs/polimento.md` (estado), `docs/codemap.md`, `CHANGELOG.md`

**Interfaces:**
- Consumes: Tasks 1 e 2; `a.terminalDir` e `a.startShell` (ADR-048); `a.startScripts`, `a.startInspect`, `a.startLogs`; `typeInView(v view, text string, enter bool)` (rerun.go:91); `colorSetFrom`; `openEditor` (editor.go), `openPrompt`, `askToConfirm`.
- Produces:
  - `func (a *App) openTemplates()`, `func (a *App) handleTemplatesKey(key uv.KeyPressEvent)`, `func (a *App) drawTemplates(screen uv.Screen)`.
  - `func (a *App) openTemplate(t templates.Template) error`: abre uma tab nova no fim e a seleciona; devolve o erro de um painel que não abriu.
  - `func drawLayoutStage(screen uv.Screen, area image.Rectangle, p *palette, shape layout.Shape, names map[layout.PaneID]string, colors map[layout.PaneID]colorSet)`: um quadro por painel, em `layout.FromShape(...).Rects(area)`, com `drawFrame`, o nome no título e `forPane`.

- [ ] **Step 1: Escrever os testes que falham**

  - `TestTemplatesListTheFiles`: com `api.yaml` e `dev.yaml`, `prefixo` + `T` abre a lista com `api` e `dev`, a seleção em `api`. Na home também abre.
  - `TestTheListShowsTheLayoutOfTheSelected`: com `dev` (dois painéis lado a lado, nomes `server` e `claude`), `screenText` mostra os dois títulos na área da miniatura.
  - `TestOpeningATemplateBuildsTheTab`: `enter` em `dev` cria uma tab `dev` com o layout, os nomes e as cores, e o foco no primeiro painel. Os terminais abrem em `a.terminalDir(<tab anterior>)` (com `recordLaunches`).
  - `TestRunTypesTheCommandAndAskOffersIt`: um terminal `run` recebe `fastfetch` + enter (`typed(view)`); um `ask` tem `pendingRerun(id) == "claude"`; um `none` não tem nenhum dos dois.
  - `TestAPaneThatCannotOpenIsLeftOut`: um painel de tipo desconhecido, ou um comando com `\n`, fica de fora ou sem comando, e a mensagem diz qual; o resto da tab abre.
  - `TestTemplatesEditRenameDelete`: `e` chama o editor com o caminho do arquivo; `r` com o nome `web` renomeia; `d` pergunta e apaga.
  - `TestAGoneTemplateSaysSoOnEnter`: apagar `dev.yaml` com a lista aberta e apertar `enter` mostra o erro, sem abrir tab.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ -run 'TestTemplates|TestTheListShows|TestOpeningATemplate|TestRunTypes|TestAPaneThatCannot|TestAGoneTemplate'`
  Expected: FAIL.

- [ ] **Step 3: Implementar**

  - **A lista:** à esquerda (como sessões), a miniatura à direita com `drawLayoutStage`, as dicas embaixo.
  - **`enter`:** relê o arquivo com `templates.Load`.
  - **`openTemplate`:** o diretório é lido antes de criar a tab, como `openNewTab` faz.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/app/`
  Expected: PASS.

- [ ] **Step 5: Documentação**

  - **`docs/decisoes.md`, ADR-052 "Templates de tab":**
    - decisão: um template é uma tab; YAML na pasta do config; o que entra; `run`/`ask`/`none`; o diretório herdado; as teclas `T`/`R`;
    - consequências: sem templates de projeto nem flutuantes, e os comandos de `run` são do próprio usuário.
  - **`docs/tui.md`:** `T` e `R` nos atalhos, e "Layouts nomeados…" passa a descrever os templates.
  - **Estados:** `docs/templates.md` "implementado (ADR-052)" e `docs/polimento.md` item 4 "Feito".
  - **`docs/codemap.md`:** linhas para `template_save.go` e `templates_popup.go`.
  - **`CHANGELOG.md`, `### Added`:** "Tab templates. `prefix` + `R` saves the tab's layout, names and colors as a template, and for each terminal whether its command runs when the template opens, asks first, or does not run. `prefix` + `T` opens a template as a new tab, picked from a list that shows its layout."

- [ ] **Step 6: Gate e commit**

  Run: `make check`
  Expected: sem erro.

  ```bash
  git add internal/app docs CHANGELOG.md
  git commit -m "Open a tab from a template with prefix + T"
  ```
