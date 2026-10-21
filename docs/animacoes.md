# Animações

**Estado:** implementado (ADR-053).

Hoje há duas: a intro do título, com os `0`/`1` que revelam o ASCII (ADR-025, ADR-043), e o mesmo efeito ao passar o mouse no título da home. Este documento descreve animações escolhidas por **momento** (o que acontece) e por **efeito** (como aparece), com prévia no config.

## Momentos

| Momento | O que anima |
|---|---|
| Hover do mouse | O que está sob o mouse: nome de tab, título de painel, linha de lista, o corvo |
| Abrir e fechar painel | O painel que abre ou fecha, e o espaço que os vizinhos ganham ou cedem |
| Abrir e fechar tab | O nome na barra e o conteúdo da tab |
| Foco | A borda e o título do painel que perde e do que ganha o foco |
| Mover painel | O painel movido até o lugar novo |
| Tela inteira | O painel crescendo até a tab, e voltando |
| Popup | Qualquer popup ao abrir e fechar |
| Atividade | Um painel ou tab sem foco cuja saída mudou |

A atividade depende do indicador de "atividade não vista", previsto em [tui.md](tui.md) e ainda não feito.

## Efeitos

| Efeito | Como é |
|---|---|
| `none` | Sem animação |
| `01` | Células viram `0`/`1` sorteados e se revelam numa varredura com jitter, como a intro |
| `fade` | A cor vai da antiga à nova por mistura (o `mix` que já existe) |
| `jitter` | Caracteres especiais se misturam até virar o caractere certo, como o *decrypt* de filmes |

A mola (`spring`) e o `flicker` existiram e saíram (ADR-053, nota de 2026-10-21): na mola o conteúdo aparecia de uma vez e só as bordas chegavam atrasadas, e o flicker era lento e pedia um aviso a cada passagem.

## Config

`Appearance → animations`:

- uma linha geral, `on`/`off`;
- embaixo, uma linha por momento, que escolhe o efeito com `←`/`→`, e sob ela `time`, o tempo do momento em ms (de 50 a 2000; `←`/`→` andam de 50 em 50, e um número digitado vale direto). A lista de efeitos vem de um registro, como `new tab opens` vem de `panelKinds`: um efeito novo aparece sozinho em todas as linhas.

O `animate intro` continua como está, à parte.

**Padrões:**

| Momento | Efeito | Tempo |
|---|---|---|
| Abrir e fechar painel, abrir e fechar tab | `fade` | 300 ms |
| Mover | `fade` | 200 ms |
| Tela inteira | `fade` | 250 ms |
| Foco | `fade` | 150 ms |
| Popup | `jitter` | 300 ms |
| Atividade | pulso de cor (`fade` indo e voltando) | 600 ms |
| Hover | `01` | 400 ms |

## Regras

- **O programa nunca sabe da animação.** Ela só muda o que é desenhado. O PTY é redimensionado uma vez (no fim, ou no começo), nunca a cada quadro, para não mandar um SIGWINCH por quadro e fazer o shell e o vim redesenharem no meio.
- **Abrir, fechar e popup** podem animar o conteúdo (o texto do painel novo "decifrando"). **Foco, mover, hover e atividade** só animam a moldura (bordas, títulos, a barra), porque acontecem toda hora e não podem mexer no que se está lendo.
- **Hover** anima uma vez por passagem; o mouse parado não repete. Nunca no conteúdo dos programas.
- **Cada animação segue o alvo:** o painel, o título dele, o nome da tab ou o popup são encontrados de novo a cada quadro, então uma tab que muda de nome ou um painel que se move levam a animação junto. Popups animam só dentro do próprio retângulo.
- **Foco** anima só o painel que ganha o foco. Trocar o foco de novo no meio não interrompe: o painel deixado para trás termina voltando à cor dele, e trocar rápido deixa um rastro.
- **Teclas não esperam:** qualquer tecla leva as outras animações ao estado final e é tratada normalmente, sem ser engolida (diferente da intro, que só pula).
- **Curtas:** cada momento tem o próprio tempo, no config, com o limite de ~60 quadros/s que o desenho já tem.
- **Código próprio:** easing é pouca matemática sobre o laço de quadros da intro e o `mix`; sem dependência nova.

## Prévia

O config das animações tem o palco ([temas.md](temas.md#o-palco)) embaixo da lista, com 30% da altura e o subtítulo `example · space repeats it` (some quando não cabe), tocando o efeito do momento selecionado: na linha "abrir painel", um mini-painel abre no palco; em "popup", um mini-popup aparece; em "hover", um cursor `▲` falso passa sobre uma tab.

- Toca uma vez ao chegar na linha (do efeito ou do tempo) e a cada troca (`←`/`→`); `space` repete. Sem loop.
- Com o mesmo tempo e a mesma curva de quando acontece de verdade, só menor; mudar o tempo toca de novo.
- Com `animations: off`, o palco mostra o estado final parado, com a nota `animations off`.

## Catálogo para depois

Efeitos de outras bibliotecas que servem aos momentos do noct, para entrar no registro quando houver vontade. Do [tachyonfx](https://github.com/junkdog/tachyonfx) vem também a ideia de **padrões espaciais** (radial, diagonal, espiral, varredura), que dizem em que ordem as células animam: o mesmo efeito com padrões diferentes vira vários.

| Efeito | De onde | Como é | Onde no noct |
|---|---|---|---|
| BinaryPath | [TTE](https://github.com/ChrisBuilds/terminaltexteffects) | Caracteres viram binário e andam em ângulo reto até o lugar | Abrir tab, intro |
| Evolve | tachyonfx | Cada célula passa por uma série de símbolos (`░▒▓█`) até o caractere | Variante do `jitter` |
| Coalesce / Dissolve | tachyonfx | Células aparecem ou somem em ordem aleatória | Abrir e fechar painel |
| Crumble | TTE | O texto vira pó e cai | Fechar painel ou tab |
| Sweep / Wipe | ambos | Uma faixa atravessa a área revelando | Trocar de sessão, com o tema novo |
| Highlight | TTE | Um brilho especular passa pelo texto | Hover, foco |
| Slide / Slice | ambos | Entra de fora, ou duas metades de lados opostos | Popup, mover |
| Expand / MiddleOut | ambos | Cresce do centro para fora | Tela inteira, popup |
| Stretch | tachyonfx | Estica com caracteres de bloco | Resize |
| HSL shift / ColorShift | ambos | Ciclo de matiz | Atividade, execução rodando |
| Unstable / VHSTape | TTE | Tremor e glitch de fita | Execução que falhou |
| Matrix / Rain | TTE, cmatrix | Chuva de caracteres | Home ociosa |
| LaserEtch / Print | TTE | Um laser ou uma cabeça de impressão escreve o texto | Intro alternativa |
| Explode | tachyonfx | Células espalham como partículas | Fechar |

Todos os da TTE estão no [showroom](https://chrisbuilds.github.io/terminaltexteffects/showroom/).

## Ao implementar

- ADR; em `tui.md`, a linha de Estética "Animações só onde informam algo" passa a apontar para este documento, e o Config ganha `animations`; `CHANGELOG.md`.
