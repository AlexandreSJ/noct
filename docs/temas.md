# Temas e cores

**Estado:** temas, editor e tema por sessão implementados (ADR-049, ADR-050); as cores por painel e por tab ainda não (passo 3 de [polimento.md](polimento.md)).

Hoje há sete temas fixos em `internal/app/theme.go`, cada um com sete cores (`accent`, `onAccent`, `muted`, `text`, `attention`, `background`, `logo`), das quais os estilos do `palette` são derivados. Este documento descreve temas criados e editados dentro do noct, o tema de cada sessão e as cores por painel e por tab.

## Componentes de um tema

Um tema dá cor a cada componente desenhado pelo noct, não a uns poucos tokens: trocar o nome da tab inativa ou o título da ativa deve ser possível sem mexer no resto.

| Grupo | Componentes |
|---|---|
| Fundo | fundo, texto padrão (os dois só valem com `background = theme`) |
| Tabs | nome da tab ativa, fundo da tab ativa, nome da tab inativa, fundo da tab inativa |
| Painéis | borda com foco, borda sem foco, título com foco, título sem foco |
| Painéis do noct | texto, texto apagado, cabeçalho, linha selecionada (texto), linha selecionada (fundo), fundo dos popups |
| Estado | mensagem (atenção), sucesso, erro |
| Outros | corvo, calendário (os níveis do calendário são misturas dessa cor com o fundo, como hoje) |

Os temas embutidos passam a definir todos os componentes. A lista é a do editor e a do arquivo; um componente novo entra nos dois.

## Arquivo

`~/.config/noct/themes/<nome>.toml`, legível e editável à mão, relido quando muda (como as pastas da biblioteca em `library_watch.go`).

```toml
base = "noct"            # o tema de onde vêm os componentes que o arquivo não define

[tabs]
active_text = "#1a1b26"
active_background = "#9d7cd8"
inactive_text = 60       # índice 0–255; 0–15 seguem as cores do terminal de fora

[panels]
focused_border = "#9d7cd8"
```

Cores são `#rrggbb` ou um índice de 0 a 255 (com ou sem aspas). Só os fundos de tabs, linhas e popups aceitam `""`, que segue o fundo do tema. Sem `base`, o tema parte de `noct`. As chaves:

| Tabela | Chaves |
|---|---|
| `[background]` | `color`, `text` |
| `[tabs]` | `active_text`, `active_background`, `inactive_text`, `inactive_background` |
| `[panels]` | `focused_border`, `unfocused_border`, `focused_title`, `unfocused_title` |
| `[noct]` | `text`, `muted`, `heading`, `selected_text`, `selected_background`, `popup_background` |
| `[state]` | `attention`, `success`, `error` |
| `[other]` | `logo`, `calendar` |

## Editor de temas

Entra-se pelo config, em `Appearance → theme`: a lista de temas ganha `+ new theme`, e cada tema do usuário ganha `edit`. Os temas embutidos não são editados: editar um deles cria uma cópia com outro nome. Não há tecla nova.

1. **Nome e base:** o nome do tema e um tema já existente de onde ele começa.
2. **Componentes:** a lista da tabela acima, por grupo. `j`/`k` ou as setas andam, `Enter` escolhe, `Esc` volta, como no resto do config.
3. **Cor:** o seletor abaixo. `Enter` grava a cor no componente e volta à lista; `Esc` volta sem mudar.

Enquanto se edita, a tela inteira já mostra o tema em edição, como o `theme` do config faz hoje ao passar pelos temas, e o palco ao lado da lista mostra os componentes (abaixo).

### Seletor de cores

- Uma grade 16×16 com as 256 cores do terminal, navegada por `hjkl` ou setas; embaixo, o número e o hex da selecionada.
- Um campo para digitar `#rrggbb`, para qualquer cor (o noct desenha em truecolor, com fallback para 256).
- As cores 0–15 são definidas pelo tema do terminal de fora e mudam de um usuário para outro; são gravadas como índice, para continuarem seguindo o terminal. As 16–255 (cubo 6×6×6 e 24 cinzas) são fixas e gravadas como hex.
- Onde a cor pode voltar ao padrão (cores por painel), a grade tem também `theme`.

## O palco

Uma mini-workspace desenhada pelo noct, sem nenhum programa rodando: uma barra com 2 tabs e 2 ou 3 painéis falsos com bordas, títulos e algumas linhas de exemplo. É um componente só, usado em três lugares:

- no editor de temas e no popup de cores, pintado com as cores em edição; o componente selecionado na lista pisca uma vez no palco, para mostrar onde ele aparece ("borda sem foco");
- no config das animações, tocando o efeito do momento selecionado ([animacoes.md](animacoes.md#prévia));
- no popup de templates, como miniatura do layout de cada um, nas cores salvas ([templates.md](templates.md)).

O palco fica ao lado da lista, ou embaixo dela se o painel for estreito, e some se não houver espaço.

## Tema por sessão

O tema é da sessão, não do noct inteiro: com várias sessões, a cor diz em qual se está.

- `Appearance → theme` no config muda o tema da sessão na tela.
- Uma sessão nova nasce com o tema da sessão de onde foi criada.
- No popup de sessões (`prefixo` + `o`) e na busca (`prefixo` + `Space`), mover a seleção para uma sessão pinta a tela inteira com o tema dela, antes de abri-la; `Esc` volta ao tema da sessão na tela.
- O tema vai no `sessions.json` com o resto da sessão. Isso é o primeiro "configuração por sessão" que a ADR-037 deixou para depois; as demais opções continuam do noct inteiro.

## Cores por painel e por tab

`P` (ver [teclas novas](polimento.md#teclas-novas)) abre um popup com as cores do painel com foco e da tab dele, que sobrescrevem as do tema da sessão:

| Alvo | Componentes |
|---|---|
| Painel | borda com foco, borda sem foco, título com foco, título sem foco, fundo, texto padrão |
| Tab | nome ativo, nome inativo, fundo ativo, fundo inativo |

- Mesma navegação e mesmo seletor do editor de temas, com `theme` para tirar a cor e voltar à do tema.
- O fundo e o texto do painel funcionam como o `background = theme` já funciona, só que por painel: o fundo padrão e o texto padrão dos programas mudam, e as cores que os programas escolhem ficam.
- Um aviso aparece quando a combinação tem pouco contraste: o fundo do painel com o texto dele, ou com as cores do tema que aparecem por cima. O aviso não impede a escolha.
- As cores ficam gravadas na sessão (o painel reabre colorido) e vão para os templates salvos com `R`.
- Prioridade: painel e tab > tema da sessão.

## Ao implementar

- ADR para o editor e o arquivo de temas; outra para tema por sessão e cores por painel. `tui.md` (Config e Estética) e `CHANGELOG.md`.
- Com as cores por painel, um painel pode ter um fundo diferente do resto; o desenho das bordas e dos programas precisa receber o estilo do painel, não só o `palette` global.
