# Animações: plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Animações por momento (hover, painel, tab, foco, mover, tela inteira, popup, atividade) com efeitos escolhidos no config (`none`, `01`, `spring`, `fade`, `jitter`, `flicker`), que nunca redimensionam os programas nem esperam teclas, com prévia no palco.

**Architecture:**
- Um animador no `App`: uma lista de animações em curso e um `<-chan time.Time` que só existe enquanto há alguma. É o mesmo padrão do `homeTick` (home.go:298). As animações são desenhadas por cima do quadro normal, numa camada como o `themedScreen` (draw.go:108).
- Os momentos não são espalhados pelo código. Antes de cada quadro, o animador compara um retrato do estado (tab ativa, painéis e seus quadros, foco, tela inteira, modo) com o do quadro anterior e começa a animação de cada diferença.
- Para fechar, usa as células do quadro anterior, guardadas por uma tela que grava o que foi desenhado.

**Tech Stack:** Go, `internal/app`, `internal/config`. Sem dependências novas (easing e mola próprios).

**Spec:** `docs/animacoes.md`; o palco em `docs/temas.md`.

**Depende de:** o plano de temas (o palco e o palette com sucesso e erro).

## Global Constraints

- **Momentos e config:** `hover`, `panels` (abrir e fechar painel), `tabs` (abrir e fechar tab), `focus`, `move`, `fullscreen`, `popup`, `activity`, em `[animations]` no `config.toml`, mais `Appearance.Animations` com `on` (padrão) ou `off`.
- **Padrões:** `panels`, `tabs`, `move` e `fullscreen` com `spring`; `focus` com `fade`; `popup` com `jitter`; `activity` com `fade` (pulso de ida e volta); `hover` com `01`. Um valor desconhecido vira o padrão do momento, e `flicker` nunca é padrão.
- **Durações:**

  | Efeito | Duração |
  |---|---|
  | `spring` | 250 ms |
  | `fade` | 150 ms (o pulso de atividade: 600 ms) |
  | `jitter` | 300 ms |
  | `01` | 400 ms |
  | `flicker` | 1000 ms, no máximo 3 acesos por segundo |

- **O programa nunca é redimensionado durante a animação:** o `Resize` das views acontece uma vez, como hoje. A animação só muda o que é desenhado.
- **Conteúdo e moldura:** abrir e fechar painel e tab, e o popup, podem animar o conteúdo. Foco, mover, hover e atividade só animam a moldura (bordas, títulos, a barra).
- **Teclas:** qualquer tecla termina todas as animações no estado final e é tratada normalmente. A intro continua como está (ADR-025/043).
- **Flicker:** escolher `flicker` mostra o aviso de epilepsia e pede `enter`.
- **Quadros:** no máximo ~60 por segundo (o `frameInterval` de hoje).
- Mesmas regras de idioma, CHANGELOG, codemap e `make check` dos outros planos.

## Review Focus

1. **Programas que pedem eventos de movimento do mouse** (vim, htop): com o hover ligado, o noct liga o modo de movimento em todo lugar. Os programas só recebem movimento se o pediram, como hoje. Teste na Task 3.
2. **Animações em sequência rápida** (`p` três vezes seguidas, ou setas segurando): cada nova começa do estado atual, sem acumular nem travar. O animador não passa de uma animação por painel e por momento. Teste na Task 1.
3. **Sessão trocada ou cliente desconectado no meio da animação:** as animações em curso são descartadas, e a sessão nova não herda nenhuma. Teste na Task 1.
4. **Terminal redimensionado durante uma animação:** ela termina na hora, porque os quadros mudaram. Teste na Task 1.
5. **Atividade de um painel sem foco que imprime sem parar** (`tail -f`): o pulso não se repete a cada pedaço de saída; volta só depois de o painel ganhar foco e perder de novo. Teste na Task 3.

---

### Task 1: animador, efeitos e abrir/fechar painel

**Files:**
- Create: `internal/app/animation.go` (animador, retrato do estado, camada de desenho), `internal/app/effects.go` (funções puras dos efeitos), `internal/app/animation_test.go`, `internal/app/effects_test.go`
- Modify: `internal/config/config.go` (`Appearance.Animations`, `Animations` com um campo por momento, padrões e `normalized`), `internal/app/app.go` (case `animTick` no loop; `handleEvent` termina as animações a cada tecla), `internal/app/draw.go` (`render`/`draw` passam pela camada e gravam o quadro), `internal/app/sessions.go:114` (`switchSession` descarta)
- Modify: `docs/codemap.md`

**Interfaces:**
- Produces:
  - `type moment int` (`momentHover, momentPanels, momentTabs, momentFocus, momentMove, momentFullscreen, momentPopup, momentActivity`) e `var moments = []momentInfo{{key, label string, def string}}` (`key` como no config, `label` como no painel).
  - `type effect int` (`effectNone, effect01, effectSpring, effectFade, effectJitter, effectFlicker`) e `var effects = []effectInfo{{name string, duration time.Duration, warn bool}}`. Um efeito novo entra aqui e aparece em todas as linhas.
  - `func (a *App) effectFor(m moment) effect`: `effectNone` com `animations = off`.
  - `type animation struct{ moment moment; effect effect; start time.Time; pane layout.PaneID; from, to image.Rectangle; cells *uv.ScreenBuffer; closing bool }`.
  - **Campos no `App`:** `animations []*animation`, `animTick <-chan time.Time`, `lastShot shot`, `lastFrame *uv.ScreenBuffer`.
  - `type shot struct{ session *session; tab *tab; frames map[layout.PaneID]image.Rectangle; focused layout.PaneID; fullscreen bool; mode mode; tabs int }` e `func (a *App) takeShot() shot`.
  - `func (a *App) startAnimations(prev, now shot, at time.Time)`: nesta task, só painel novo (id em `now.frames` sem estar em `prev`) e painel fechado (o inverso, com `cells` recortadas de `a.lastFrame`).
  - `func (a *App) advanceAnimations(at time.Time) bool`: tira as que acabaram e rearma `animTick` com `time.After(frameInterval)`, ou zera.
  - `func (a *App) finishAnimations()`.
  - `type animatedScreen struct{ uv.Screen; ... }`, que desenha as animações em curso sobre as células.
  - **Efeitos** (`effects.go`, puros, `t` de 0 a 1):
    - `func springAt(t float64) float64`: mola subamortecida, `1 - e^{-6t}·cos(9t)` normalizada para valer 0 em 0 e 1 em 1, passando um pouco de 1 no meio;
    - `func easeOut(t float64) float64`;
    - `func rectAt(from, to image.Rectangle, f float64) image.Rectangle`;
    - `func binaryAt(r rune, t float64, revealAt float64, rnd *rand.Rand) rune`: `0`/`1` até `revealAt`;
    - `func jitterAt(r rune, t, settleAt float64, rnd *rand.Rand) rune`: um de `░▒▓█#%&@$*+=` até `settleAt`;
    - `func flickerOn(elapsed time.Duration) bool`: aceso nos primeiros 167 ms de cada 333 ms.

- [ ] **Step 1: Escrever os testes que falham**

  - `TestSpringStartsAndEndsInPlace` (effects_test.go): `springAt(0) == 0`; `springAt(1) == 1` (tolerância de 0,001); algum `t` em (0,1) passa de 1.
  - `TestFlickerNeverPassesThreeFlashesASecond`: contar as passagens de apagado para aceso de `flickerOn` em passos de 1 ms ao longo de 1 s dá no máximo 3.
  - `TestBinaryAndJitterSettleOnTheRealRune`: com `t >= revealAt`, `binaryAt('x', …) == 'x'`; antes, é `'0'` ou `'1'`. O mesmo para `jitterAt`, com o conjunto dele.
  - `TestAnimationsAreOffWhenTurnedOff` (animation_test.go): `Appearance.Animations = "off"` dá `effectFor(momentPanels) == effectNone`.
  - `TestOpeningAPaneStartsASpring`: `testApp` com uma tab, `drawApp`, depois `a.openPanel(a.startTerminal)` e `drawApp`. Há uma animação `momentPanels` para o painel novo, e `animTick != nil`.
  - `TestTheSpringGrowsThePaneWithoutResizingIt`: no meio da animação (`start` recuado 125 ms), as células da borda do painel novo não estão todas no quadro final, e o `fakeView.size` do painel novo foi definido uma vez só, no tamanho final.
  - `TestAnAnimationEndsInTheNormalFrame`: com `start` recuado 300 ms e `advanceAnimations(now)`, a lista fica vazia, `animTick == nil`, e o buffer é igual ao de uma app sem animações.
  - `TestClosingAPaneShrinksWhatItShowed`: desenhar, fechar o painel (`closeFocused`) e desenhar no começo da animação. As células onde o painel estava ainda mostram o quadro anterior (de `lastFrame`) dentro do retângulo encolhendo.
  - `TestAKeyFinishesTheAnimationsAndIsHandled`: com uma animação em curso, `handleEvent(letter('x'))` esvazia a lista, e o `fakeView` com foco recebe o `x`.
  - `TestNewAnimationsReplaceOldOnesOfTheSamePane`: abrir três painéis seguidos, desenhando entre cada um, deixa no máximo uma animação por painel.
  - `TestSwitchingSessionDropsTheAnimations`: com uma animação em curso, `switchSession(outra)` esvazia a lista.
  - `TestResizingTheTerminalEndsTheAnimations`: com uma animação em curso, mudar `a.size` e desenhar esvazia a lista.
  - `TestConfigHasTheDefaultEffects` (`internal/config`): `Default()` tem `panels = "spring"`, `focus = "fade"`, `popup = "jitter"`, `hover = "01"`, `activity = "fade"`; um `panels = "boing"` lido vira `"spring"`; `flicker` lido continua `flicker`.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ ./internal/config/ -run 'TestSpring|TestFlicker|TestBinaryAndJitter|TestAnimations|TestOpeningAPane|TestTheSpring|TestAnAnimation|TestClosingAPane|TestAKeyFinishes|TestNewAnimations|TestSwitchingSession|TestResizingTheTerminal|TestConfigHasTheDefault'`
  Expected: FAIL, não compila.

- [ ] **Step 3: Implementar**

  - **`render`:** desenha numa tela que grava em `a.lastFrame` (um `uv.ScreenBuffer` do tamanho da tela, reaproveitado entre quadros).
  - **Antes de `draw`:** `now := a.takeShot()`, `a.startAnimations(a.lastShot, now, time.Now())` e `a.lastShot = now`.
  - **`draw`:** passa a tela por `animatedScreen` quando há animações.
  - **Loop:** o case `<-a.animTick` chama `advanceAnimations`.
  - **`handleEvent`:** um `KeyPressEvent` chama `finishAnimations()` antes do resto.
  - **Retratos que não se comparam** (sessão diferente, ou o tamanho mudou): não começam animação e terminam as que estão em curso.

  O `spring` de abrir revela o painel num retângulo que cresce do centro do quadro final (`rectAt(centro, quadro, springAt(t))`). Fora dele, as células do painel mostram o fundo. Fechar faz o inverso, com as células de `cells`.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/app/ ./internal/config/`
  Expected: PASS.

- [ ] **Step 5: Commit**

  `docs/codemap.md`: linhas para `animation.go` ("Animations: what changed since the last frame, the ones running and drawing them over the frame") e `effects.go` ("The animation effects as functions of time: spring, easing, 01, jitter and flicker").

  ```bash
  git add internal/app internal/config docs/codemap.md
  git commit -m "Animate panels opening and closing, without resizing their programs"
  ```

### Task 2: tabs, foco, mover, tela inteira e popups

**Files:**
- Modify: `internal/app/animation.go`, `internal/app/animation_test.go`

**Interfaces:**
- Consumes: Task 1.
- Produces: em `startAnimations`, cada diferença dos retratos começa uma animação:
  - **tab aberta ou fechada** (`tabs` mudou): o span da tab na barra e o conteúdo dela;
  - **foco** (`focused` mudou na mesma tab): bordas e títulos dos dois painéis;
  - **mover** (o mesmo id com outro quadro, com o modo `moveMode` antes ou depois): o painel do quadro antigo ao novo, só a moldura;
  - **tela inteira** (`fullscreen` mudou): o painel com foco, entre o quadro dele e a tab;
  - **popup** (`mode` saiu de `normalMode` para um modo de popup): o retângulo do popup.

  Com `fade`, a moldura vai das cores antigas às novas com `mix`. Com `jitter`, `01` e `flicker`, cada efeito age nas células da área do momento, conforme as regras de conteúdo e moldura das Global Constraints.

- [ ] **Step 1: Escrever os testes que falham**

  Um teste por momento, com o efeito padrão:
  - `TestOpeningATabAnimatesItsNameInTheBar`;
  - `TestFocusFadesTheBorders`: no meio, a borda do painel que ganhou foco tem uma cor entre `unfocusedFrame` e `focusedFrame`, nem uma nem outra;
  - `TestMovingAPaneSlidesItsFrame`;
  - `TestFullscreenGrowsThePane`;
  - `TestAPopupAppearsWithJitter`: no começo, alguma célula do texto do popup é de `jitterAt`, e no fim tudo é o texto real.

  E mais:
  - `TestFocusNeverAnimatesTheContent`: com `focus = jitter`, as células de conteúdo dos dois painéis são iguais às do quadro sem animação.
  - `TestEachMomentCanBeTurnedOff`: com o momento em `none`, a mesma ação não começa animação.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ -run 'TestOpeningATab|TestFocus|TestMovingAPane|TestFullscreenGrows|TestAPopupAppears|TestEachMoment'`
  Expected: FAIL.

- [ ] **Step 3: Implementar** conforme o Produces.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/app/`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/app
  git commit -m "Animate tabs, focus, moving, fullscreen and popups"
  ```

### Task 3: hover e atividade

**Files:**
- Modify: `internal/app/mouse.go:44` (`applyMouseMode`) e `:69` (`handleMouse`), `internal/app/home.go:241` (o hover do título passa pelo animador, mantendo o `01` de hoje), `internal/app/workspace.go:53-59` (`OnScreenChange` por painel), `internal/app/tab.go` (`unseen map[layout.PaneID]bool`), `internal/app/animation.go`, `internal/app/animation_test.go`

**Interfaces:**
- Consumes: Tasks 1 e 2; `tabAt(x)` (tabbar.go:156), `t.paneAt`, `t.frameOf`, `configLayout().lineAt`.
- Produces:
  - `func (a *App) hoverAt(point image.Point)`: o que está sob o mouse (o nome de uma tab, o título de um painel, uma linha de lista, o título da home) começa uma animação `momentHover` só na moldura, uma vez por entrada no alvo.
  - `func (a *App) markActivity(id layout.PaneID)`: chamado no loop pelo `OnScreenChange` do painel. Num painel sem foco ou numa tab que não está à vista, sem marca, marca e começa o pulso no título do painel e no nome da tab. O foco no painel tira a marca.

- [ ] **Step 1: Escrever os testes que falham**

  - `TestHoverAnimatesATabNameOncePerVisit`: mover o mouse sobre a tab 2 começa uma animação; mover dentro dela não começa outra; sair e voltar começa.
  - `TestHoverNeedsTheMotionModeOnlyWhenOn`: com `hover = none`, `applyMouseMode` fica como hoje (movimento só na home e no config); com `01`, liga o movimento em todo lugar.
  - `TestProgramsGetMotionOnlyIfTheyAskedForIt`: com o movimento ligado pelo hover, um painel cujo programa não pediu eventos de movimento não recebe o evento (`fakeView` sem teclas e sem mouse).
  - `TestTheHomeTitleKeepsItsBinaryHover`: o hover no título da home continua com o efeito `01` nas células, como `home_test.go` já verifica.
  - `TestActivityPulsesOnceUntilSeen`: `markActivity(2)` com o foco no 1 começa um pulso; cinco `markActivity(2)` seguidos não começam outro; focar o 2, focar o 1 e chamar `markActivity(2)` começa de novo.
  - `TestActivityOfTheFocusedPaneIsIgnored`.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ -run 'TestHover|TestProgramsGetMotion|TestTheHomeTitleKeeps|TestActivity'`
  Expected: FAIL.

- [ ] **Step 3: Implementar**

  - **`OnScreenChange`:** cada painel ganha o próprio, `func() { a.notifyScreenChanged(); a.onLoop(func() { a.markActivity(id) }) }`, com um `atomic.Bool` por painel para não enfileirar uma tarefa por pedaço de saída enquanto a marca já está posta.
  - **`forwardMouse`:** continua mandando movimento só aos programas que pediram.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/app/`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/app
  git commit -m "Animate hovering and activity in panels out of focus"
  ```

### Task 4: config, aviso do flicker, prévia no palco e documentação

**Files:**
- Modify: `internal/app/config_panel.go` (cabeçalho `Animations`, a linha `animations` `on`/`off` e uma linha por momento com `◂ efeito ▸`), `internal/app/stage.go` (`stagePreview`), `internal/app/animation.go` (prévia no palco)
- Create: `internal/app/animation_config_test.go`
- Modify: `docs/decisoes.md` (ADR-053), `docs/tui.md` (Config e Estética), `docs/animacoes.md` e `docs/polimento.md` (estado), `CHANGELOG.md`

**Interfaces:**
- Consumes: tudo antes; `drawStage` e `askToConfirm`.
- Produces:
  - `type stagePreview struct{ moment moment; effect effect; start time.Time }` e `func drawStageAnimated(screen uv.Screen, area image.Rectangle, p *palette, preview *stagePreview, now time.Time)`, que encena o momento no palco:
    - **painel:** o painel da direita abre;
    - **tab:** a tab `logs` aparece;
    - **foco:** o foco passa da esquerda à direita;
    - **mover:** o painel da direita passa para baixo do da esquerda;
    - **tela inteira:** o da esquerda cresce;
    - **popup:** a caixa `popup` aparece;
    - **hover:** um `▲` falso passa sobre a tab `logs`;
    - **atividade:** a tab `logs` pulsa.
  - Campo `preview *stagePreview` no `configPanel`, começado ao chegar numa linha de momento, a cada `←`/`→` e com `space`, e tocado uma vez (sem loop).

- [ ] **Step 1: Escrever os testes que falham**

  - `TestTheConfigListsEveryMomentWithItsEffect`: as linhas `hover`, `panels`, `tabs`, `focus`, `move`, `fullscreen`, `popup` e `activity`, cada uma com o efeito padrão. `→` em `panels` passa a `fade` e grava `[animations] panels = "fade"`.
  - `TestFlickerAsksBeforeItIsUsed`: chegar a `flicker` com `→` abre a confirmação com o aviso (o texto contém `epilepsy`); `esc` deixa o efeito anterior; `enter` grava `flicker`.
  - `TestThePreviewPlaysOnceOnArrivingAndChanging`: chegar na linha `popup` começa a prévia; com `start` recuado além da duração e o tick, ela para; `space` começa de novo; `→` também.
  - `TestThePreviewUsesTheRealDuration`: a prévia de `spring` acaba em 250 ms, como a animação real.
  - `TestTheFlickerPreviewWaitsForTheWarning`: com a confirmação aberta, não há prévia.
  - `TestWithAnimationsOffTheStageIsStill`: com `animations = off`, o palco não tem prévia e mostra `animations off`.

- [ ] **Step 2: Rodar e ver falhar**

  Run: `go test -race ./internal/app/ -run 'TestTheConfigLists|TestFlickerAsks|TestThePreview|TestTheFlickerPreview|TestWithAnimationsOff'`
  Expected: FAIL.

- [ ] **Step 3: Implementar** conforme o Produces.

  O texto do aviso: `flicker flashes colors up to 3 times a second, which can trigger seizures in people with photosensitive epilepsy. use it?`, com `enter use · esc keep <efeito anterior>`.

- [ ] **Step 4: Rodar e ver passar**

  Run: `go test -race ./internal/app/ ./internal/config/`
  Expected: PASS.

- [ ] **Step 5: Documentação**

  - **`docs/decisoes.md`, ADR-053 "Animações por momento":**
    - decisão: momentos e efeitos, o retrato do estado no lugar de ganchos espalhados, o quadro anterior para fechar, nada de resize no meio, a tecla que termina, o flicker com limite e aviso, a prévia no palco, sem dependência nova;
    - consequências: com o hover ligado, o mouse fica em modo de movimento; a atividade tem o primeiro uso do `OnScreenChange` por painel.
  - **`docs/tui.md`:** a linha "Animações só onde informam algo…" de Estética passa a apontar para `animacoes.md`, e as linhas de `Appearance → animations` entram na tabela do Config.
  - **Estados:** `docs/animacoes.md` "implementado (ADR-053)" e `docs/polimento.md` item 5 "Feito".
  - **`CHANGELOG.md`, `### Added`:** "Animations. Panels and tabs open and close with a spring, focus fades from one panel to the other, popups appear with a jitter, and a panel out of focus pulses when it prints. Each moment and its effect (`01`, `spring`, `fade`, `jitter`, `flicker`) is in **Appearance → animations**, with a preview beside the list. Any key ends an animation at once, and programs in panels are never resized during one."

- [ ] **Step 6: Gate e commit**

  Run: `make check`
  Expected: sem erro.

  ```bash
  git add internal/app internal/config docs CHANGELOG.md
  git commit -m "Choose each animation in the config, with a preview on the stage"
  ```
