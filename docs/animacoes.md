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
| `spring` | O tamanho ou a posição chega ao valor final como uma mola: passa um pouco e volta |
| `fade` | A cor vai da antiga à nova por mistura (o `mix` que já existe) |
| `jitter` | Caracteres especiais se misturam até virar o caractere certo, como o *decrypt* de filmes |
| `flicker` | As cores piscam; ver [segurança](#flicker-e-epilepsia) |

## Config

`Appearance → animations`:

- uma linha geral, `on`/`off`;
- embaixo, uma linha por momento, que escolhe o efeito com `←`/`→`. A lista de efeitos vem de um registro, como `new tab opens` vem de `panelKinds`: um efeito novo aparece sozinho em todas as linhas.

O `animate intro` continua como está, à parte.

**Padrões:**

| Momento | Efeito |
|---|---|
| Abrir e fechar painel, abrir e fechar tab, mover, tela inteira | `spring` |
| Foco | `fade` |
| Popup | `jitter` |
| Atividade | pulso de cor (`fade` indo e voltando) |
| Hover | `01` só no corvo, como hoje |

`flicker` nunca é padrão.

## Regras

- **O programa nunca sabe da animação.** Ela só muda o que é desenhado. O PTY é redimensionado uma vez (no fim, ou no começo), nunca a cada quadro, para não mandar um SIGWINCH por quadro e fazer o shell e o vim redesenharem no meio.
- **Abrir, fechar e popup** podem animar o conteúdo (o texto do painel novo "decifrando"). **Foco, mover, hover e atividade** só animam a moldura (bordas, títulos, a barra), porque acontecem toda hora e não podem mexer no que se está lendo.
- **Hover** anima uma vez por passagem; o mouse parado não repete. Nunca no conteúdo dos programas.
- **Teclas não esperam:** qualquer tecla leva a animação ao estado final e é tratada normalmente, sem ser engolida (diferente da intro, que só pula).
- **Curtas:** cada efeito tem a própria duração e curva, com o limite de ~60 quadros/s que o desenho já tem. A ideia é ficar entre 100 e 300 ms.
- **Código próprio:** easing e mola são pouca matemática sobre o laço de quadros da intro e o `mix`; sem dependência nova. Se ficar complexo, o `harmonica` da Charm entra com ADR.

### Flicker e epilepsia

- O flicker nunca passa de 3 flashes por segundo (WCAG 2.3.1), mesmo ligado.
- Escolher `flicker` mostra um aviso de epilepsia e pede `Enter` de novo para confirmar.

## Prévia

O config das animações tem o palco ([temas.md](temas.md#o-palco)) ao lado da lista, tocando o efeito do momento selecionado: na linha "abrir painel", um mini-painel abre no palco; em "popup", um mini-popup aparece; em "hover", um cursor `▲` falso passa sobre uma tab.

- Toca uma vez ao chegar na linha e a cada troca de efeito (`←`/`→`); `space` repete. Sem loop.
- Com a mesma duração e a mesma curva de quando acontece de verdade, só menor.
- `flicker` só toca depois do aviso confirmado.
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
