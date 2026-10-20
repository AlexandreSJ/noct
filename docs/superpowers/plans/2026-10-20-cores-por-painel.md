# Cores por painel e por tab: plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cada painel e cada tab podem ter cores próprias por cima do tema da sessão, escolhidas no popup do `prefixo` + `P` e gravadas na sessão.

**Architecture:** As cores próprias são um `colorSet` (componente → cor) por painel e por tab, guardados no `tab`. O desenho dos painéis e da barra de tabs aplica o `colorSet` sobre o palette. O popup reaproveita o seletor de cores e o palco do plano de temas.

**Tech Stack:** Go, `internal/app`, `internal/session`, `internal/theme` (do plano de temas).

**Spec:** `docs/temas.md`, seção "Cores por painel e por tab"; teclas em `docs/polimento.md`, seção "Teclas novas".

**Depende de:** `docs/superpowers/plans/2026-10-20-temas.md` inteiro (componentes, `newColorPicker` com `allowTheme`, `drawStage`).

## Global Constraints

- Componentes do painel: `FocusedBorder`, `UnfocusedBorder`, `FocusedTitle`, `UnfocusedTitle`, `Background`, `Text`. Componentes da tab: `ActiveTabText`, `InactiveTabText`, `ActiveTabBackground`, `InactiveTabBackground`. Sem outros.
- Prioridade: painel e tab > tema da sessão. A ausência no `colorSet` quer dizer "a do tema".
- O fundo e o texto do painel funcionam como `background = theme`, mas por painel: mudam o fundo e o texto padrão dos programas, e as cores que os programas escolhem ficam.
- O aviso de contraste não impede a escolha.
- Teclas: `P` no contexto `commands` (`[]string{"P", "shift+p"}`), trocável nos binds. Como atalho direto, `Alt+Shift+P`.
- Mesmas regras de idioma, CHANGELOG, codemap e `make check` do plano de temas.

## Review Focus

1. **Painel movido para outra tab** (`prefixo` + `a` na borda): leva as cores junto, como leva o nome. Teste na Task 1.
2. **Painel fechado e id reaproveitado:** um painel novo nunca herda as cores de um fechado. Teste na Task 1.
3. **Fundo próprio com `background = terminal`:** o painel pinta o próprio fundo, os outros continuam com o do terminal. Teste na Task 1.
4. **`Alt+Shift+h/j/k/l` continuam movendo o painel** depois que `Alt+Shift+letra` passa a valer como comando. Teste na Task 2.
5. **Popup aberto numa tab vazia ou na home:** `P` não faz nada (é ação de tab, como `r`/`e`). Teste na Task 2.

---

### Task 1: cores no desenho e na sessão

**Files:**
- Create: `internal/app/colorset.go`, `internal/app/colorset_test.go`
- Modify: `internal/app/tab.go` (campos e `removeView:190`), `internal/app/workspace.go:311-336` (`movePaneToAdjacentTab`), `internal/app/draw.go:71-108` (`drawPanes`), `internal/app/tabbar.go:142` (`drawTabBar`), `internal/app/sessions.go:256-370` (`savedTab`, `savedPane`, `restoreTab`, `restorePane`), `internal/session/session.go` (`Tab.Colors`, `Pane.Colors`)
- Modify: `docs/codemap.md`

**Interfaces:**
- Consumes: `theme.Component`, `theme.Color`, `theme.Components[c].Key`.
- Produces:
  - `type colorSet map[theme.Component]theme.Color`.
  - `func (s colorSet) saved() map[string]string` e `func colorSetFrom(m map[string]string) colorSet`, usando `Group + "." + Key` (`"panels.focused_border"`); chaves desconhecidas ou cores inválidas são ignoradas.
  - Campos no `tab`: `paneColors map[layout.PaneID]colorSet`, criado em `newEmptyTab`, e `colors colorSet`.
  - `func (p palette) forPane(s colorSet) palette`: copia o palette trocando `focusedFrame`, `unfocusedFrame`, `focusedTitle`, `unfocusedTitle`, `background` e `text` pelas cores presentes.
  - `func (p palette) forTab(s colorSet) (active, inactive uv.Style)`.
  - `session.Pane.Colors` e `session.Tab.Colors` (`map[string]string`, `json:"colors,omitempty"`).

- [ ] **Step 1: Escrever os testes que falham**

  - `TestAPaneColorOverridesTheTheme`: tab 1 | 2, `paneColors[2] = {FocusedBorder: "#ff0000"}`, foco no 2. No buffer de `drawApp`, a borda do 2 tem `Fg` `#ff0000` e a do 1 a do tema. Sem foco no 2, a borda dele é `unfocusedFrame` do tema.
  - `TestAPaneBackgroundPaintsOnlyThatPane`: com `background = terminal` e `paneColors[2] = {Background: "#202020", Text: "#eeeeee"}`, uma célula de conteúdo do 2 sem cor própria tem `Bg` `#202020` e `Fg` `#eeeeee`; uma célula do 1 tem `Bg == nil`.
  - `TestATabColorPaintsItsNameInTheBar`: `colors = {InactiveTabText: "#00ff00"}` na tab 2 de duas; o nome da tab 2 na linha 0 tem `Fg` `#00ff00`. Ativa, usa `ActiveTabText` (do tema, se não definido).
  - `TestColorsMoveWithThePane`: mover o painel com cor para a tab vizinha (`movePaneToAdjacentTab`) leva o `colorSet` junto, e a tab de origem fica sem ele.
  - `TestAClosedPanesColorsAreForgotten`: fechar o painel com cor apaga `paneColors[id]`.
  - `TestColorsAreSavedWithTheSession`: `savedTab` grava `Colors` da tab e de cada painel com as chaves `panels.focused_border`, e `restoreTab` as devolve nos ids novos.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ -run 'TestAPaneColor|TestAPaneBackground|TestATabColor|TestColorsMove|TestAClosedPanes|TestColorsAreSaved'`
  Expected: FAIL, não compila.

- [ ] **Step 3: Implementar**

  - **`drawPanes`:** cada painel usa `pal := a.palette.forPane(t.paneColors[id])` para `fillRect`, os estilos de borda e título e o `themedScreen`. O wrapper passa a valer também quando só o painel tem fundo ou texto próprio, mesmo com `background = terminal`.
  - **`drawTabBar`:** usa `forTab(a.session.tabs[span.tab].colors)`.
  - **`removeView`:** apaga a cor junto com o nome, e `movePaneToAdjacentTab` copia as duas coisas.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/app/ ./internal/session/`
  Expected: PASS.

- [ ] **Step 5: Commit**

  `docs/codemap.md`: linha para `internal/app/colorset.go` ("A pane's or a tab's own colors over the session's theme").

  ```bash
  git add internal/app internal/session docs/codemap.md
  git commit -m "Draw and keep colors of a pane or a tab of their own"
  ```

### Task 2: o popup do `P` e `Alt+Shift` + letra

**Files:**
- Create: `internal/app/colors_popup.go`, `internal/app/colors_popup_test.go`
- Modify: `internal/app/keymap.go` (`"colors"` no contexto `commands`, contexto `colors popup`, `modeKeyContexts`), `internal/app/keys.go` (modo `colorsMode`, `commandShortcuts`, `actionKeys`, `runTabAction`, `handleKey`), `internal/app/draw.go` (`drawModeOverlay`), `internal/app/shortcut.go:47-51`
- Modify: `docs/decisoes.md` (ADR-051), `docs/tui.md` (Atalhos e Estética), `docs/temas.md` e `docs/polimento.md` (estado), `docs/codemap.md`, `CHANGELOG.md`

**Interfaces:**
- Consumes: `colorSet`, `forPane`, `forTab` (Task 1); `newColorPicker(current, allowTheme, done)` e `drawStage` (plano de temas).
- Produces:
  - `type colorsPopup struct{ tab *tab; pane layout.PaneID; selected int; picker *colorPicker }`, com as linhas: título `panel`, os 6 componentes do painel, título `tab`, os 4 da tab.
  - `func (a *App) openColors()`, `func (a *App) handleColorsKey(key uv.KeyPressEvent)`, `func (a *App) drawColors(screen uv.Screen)`.
  - `func contrast(a, b color.Color) float64`: a razão de contraste do WCAG 2 (luminância relativa, `(L1+0.05)/(L2+0.05)`).
  - `func (a *App) colorsWarning(t *tab, id layout.PaneID) string`: `"low contrast: 2.1:1"` quando o texto do painel sobre o fundo dele fica abaixo de 4,5:1, ou uma borda ou um título sobre o fundo fica abaixo de 3:1. Sem problema, `""`.

- [ ] **Step 1: Escrever os testes que falham**

  - `TestColorsOpenForTheFocusedPaneAndItsTab`: com tab e painel em foco, `prefixo` + `P` abre `colorsMode`, com as linhas `focused border` … `inactive background`.
  - `TestColorsNeedATab`: na home ou numa tab vazia, `prefixo` + `P` não muda o modo.
  - `TestPickingAPaneColorKeepsIt`: `enter` em `focused border`, escolher 196 e `enter`: `paneColors[id][FocusedBorder] == "#ff0000"`, e o popup volta à lista.
  - `TestThemeInThePickerTakesTheColorAway`: no seletor com `allowTheme`, a tecla `t` dá `done("", true)`, e o componente sai do `colorSet`.
  - `TestLowContrastIsWarnedButAllowed`: fundo `#000000` e texto `#111111` no painel: `colorsWarning` começa com `low contrast`, e as cores ficam gravadas.
  - `TestContrastOfBlackOnWhiteIs21`: `contrast(white, black)` é 21 (com tolerância de 0,01).
  - `TestAltShiftLetterRunsTheUppercaseCommand`: com o shortcut em `alt`, `Alt+Shift+P` abre o popup.
  - `TestAltShiftHjklStillMoveThePane`: com o shortcut em `alt`, `Alt+Shift+L` continua movendo o painel com foco.
  - `TestColorsPopupShowsTheStageWithTheCandidate`: com o seletor aberto, o palco do popup é desenhado com `forPane` e a cor sob a seleção.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ -run 'TestColors|TestPickingAPane|TestThemeInThePicker|TestLowContrast|TestContrastOf|TestAltShift'`
  Expected: FAIL.

- [ ] **Step 3: Implementar**

  - **Teclas:** a ação `colors` com `{"colors", "colors of the panel and tab", []string{"P", "shift+p"}}`, o `shortcut` em `commandShortcuts` com `needsTab: true`, `"P"` em `actionKeys`, e o case em `runTabAction`.
  - **`shortcut.go`:** com shift e uma tecla que não é seta, procura `commandKey(strings.ToUpper(pressed))`. Sendo uma ação de `actionKeys`, devolve `(name, false, true)`; senão continua como hoje.
  - **`allowTheme` do seletor (plano de temas):** passa a ter a tecla `t` ("use the theme"), mostrada na dica.
  - **O popup:** lista à esquerda, palco à direita, o aviso de contraste embaixo em `status`.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/app/`
  Expected: PASS.

- [ ] **Step 5: Documentação**

  - **`docs/decisoes.md`, ADR-051 "Cores por painel e por tab":**
    - decisão: os componentes, a prioridade sobre o tema, o fundo por painel como o `background = theme`, o aviso de contraste (4,5:1 para texto e 3:1 para bordas e títulos, sem bloquear), as cores na sessão, e `P` com `Alt+Shift+letra`;
    - registrar que o mantenedor pediu as maiúsculas, apesar da ADR-047.
  - **`docs/tui.md`:** `prefixo` + `P` nos atalhos; uma linha em Estética.
  - **Estados:** `docs/temas.md` "implementado" e `docs/polimento.md` item 3 "Feito".
  - **`docs/codemap.md`:** linha para `colors_popup.go`.
  - **`CHANGELOG.md`, `### Added`:** "`prefix` + `P` gives the focused panel and its tab colors of their own: borders, titles, background and text of the panel, and the tab's name and background. They stay with the session, and a warning tells when text gets hard to read."

- [ ] **Step 6: Gate e commit**

  Run: `make check`
  Expected: sem erro.

  ```bash
  git add internal/app docs CHANGELOG.md
  git commit -m "Choose a panel's and a tab's colors with prefix + P"
  ```
