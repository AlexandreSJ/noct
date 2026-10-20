# Polimento: roteiro

O que vem da fase 5 do [roadmap](roadmap.md), na ordem combinada com o mantenedor, e o que ficou para depois. Cada item tem o próprio documento; este é o mapa. Cada passo é um release que se usa sozinho, e vira ADR em [decisoes.md](decisoes.md) no commit que o implementa, como os outros itens de polimento antecipados.

| # | Item | Documento | Estado |
|---|---|---|---|
| 1 | Terminal novo no diretório do painel com foco | [tui.md](tui.md#tabs-e-painéis), ADR-048 | Feito |
| 2 | Temas editáveis no noct e tema por sessão | [temas.md](temas.md), ADR-049, ADR-050 | Feito |
| 3 | Cores por painel e por tab | [temas.md](temas.md#cores-por-painel-e-por-tab) | Decidido |
| 4 | Templates de tab | [templates.md](templates.md) | Decidido |
| 5 | Animações por momento, com prévia | [animacoes.md](animacoes.md) | Decidido |

O palco (a mini-workspace de prévia) nasce no passo 2 e é reutilizado nos passos 4 e 5.

## 1. Terminal novo no diretório do painel com foco

Feito: `p`, `t` e `g` abrem no diretório do painel com foco. A regra está em [tui.md](tui.md#tabs-e-painéis) e a decisão na ADR-048.

## Teclas novas

No modo noct, trocáveis nos binds (ADR-041): `T` abre o popup de templates, `R` salva a tab como template, `P` abre as cores do painel com foco e da tab dele. Como atalho direto, `Alt+Shift+T/R/P`. Com o shortcut só em `ctrl`, não há atalho direto: na codificação legada `Ctrl+Shift+T` chega igual a `Ctrl+T` (ADR-012), então só o prefixo serve.

As maiúsculas foram pedidas pelo mantenedor nesta rodada, mas a ADR-047 registra que ele não gosta de atalhos com letra maiúscula, e só `m` e `z` estão livres em minúscula. Confirmar ao implementar.

## Depois

Ideias que ficaram fora da ordem acima, sem compromisso:

- **Paleta ANSI nos painéis:** um tema com as 16 cores ANSI, usadas nas cores indexadas dos programas com `background = theme`, para o workspace inteiro seguir o tema.
- **Importar temas** do kitty, Ghostty ou base16.
- **Templates de projeto** (`.noct/templates/` no repositório), com os comandos sempre em `ask`, por segurança.
- **Sessão a partir de templates** (`noct new <nome> --template …`).
- **Diretório e comando por terminal no template** (hoje os terminais abrem no diretório herdado).
- **OSC 7** como segunda fonte do diretório, para o macOS e para saber quando ele é de outra máquina.
- **Painéis sem foco esmaecidos**, estilo de borda no tema (fina, arredondada, grossa, nenhuma), ícone por tipo de painel no título.
- **Diretório no título** do terminal sem título próprio (`zsh · ~/proj`).
- **Duplicar painel:** terminal novo no mesmo diretório e com o mesmo comando, na caixa `run?`.
