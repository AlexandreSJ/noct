# Templates de tab

**Estado: decidido, não implementado** (passo 4 de [polimento.md](polimento.md)).

Um template é **uma tab** gravada para abrir de novo: o layout, o tipo e o nome de cada painel, as cores do painel e da tab ([temas.md](temas.md#cores-por-painel-e-por-tab)) e, em cada terminal, o comando e como ele começa. É o "layouts nomeados podem ser salvos e reabertos" de [tui.md](tui.md).

O que as sessões já gravam (a árvore `layout.Shape`, nomes, painéis flutuantes, o comando em primeiro plano de cada shell, ADR-037 e ADR-046) é a base: um template é uma tab de sessão sem o que é só daquela execução (logs, ids, diretórios) e com o que se quer repetir.

## Criar: `R`

Monta-se a tab à mão e grava-se. Ninguém precisa escrever YAML.

`R` abre um popup com:

- o nome do template; um nome que já existe pergunta antes de sobrescrever;
- a lista dos painéis da tab, cada um com o tipo, o nome e, nos terminais, o comando que roda agora (lido do shell, como as sessões fazem);
- em cada terminal com comando, como ele começa ao abrir o template:
  - `run`: roda logo (um `fastfetch`);
  - `ask`: o painel abre com a caixa `run?` da ADR-046, e só roda com `Enter` (um `claude`);
  - `none`: só o shell.

  O padrão é `ask`. Um terminal sem comando abre só o shell.

As cores do painel e da tab vão junto, sem perguntar. Painéis que as sessões não reabrem (execuções, editores, ajuda) ficam de fora, e o layout se fecha sobre eles, como numa sessão.

## Abrir: `T`

`T` abre o popup dos templates:

- a lista, com a miniatura do layout do template selecionado ao lado, desenhada pelo palco ([temas.md](temas.md#o-palco)) nas cores salvas;
- `Enter` abre uma tab nova com o template;
- `e` abre o arquivo no editor do config, `r` renomeia, `d` apaga (com confirmação).

Os terminais abrem no diretório que um terminal novo teria naquele momento ([polimento.md](polimento.md#1-terminal-novo-no-diretório-do-painel-com-foco)), então o mesmo template serve para qualquer projeto. `run` é digitado no shell, como o `Enter` da caixa `run?`: o comando fica no histórico, e quando termina o shell continua lá.

## Arquivo

`~/.config/noct/templates/<nome>.yaml`, YAML como scripts e fluxos (é biblioteca do usuário, não config), legível e editável à mão. Esboço; o formato exato fica para a implementação:

```yaml
name: dev                  # o nome da tab
colors:
  active_background: "#9ece6a"
layout: columns            # lado a lado; rows = um embaixo do outro
panes:
  - terminal: { name: info, run: fastfetch, start: run }
  - rows:
      - terminal: { run: claude, start: ask }
      - scripts: {}
        colors: { focused_border: "#f7768e" }
```

O layout no arquivo é legível (`columns`/`rows` aninhados), não o `layout.Shape` interno.

Só os templates do usuário, por enquanto. Templates num repositório (`.noct/templates/`) trariam por `git pull` comandos com `start: run` de outra pessoa; ver [polimento.md](polimento.md#depois).

## Ao implementar

- ADR; `tui.md` (Tabs e painéis, Atalhos) e `CHANGELOG.md`; `docs/codemap.md` para o pacote ou os arquivos novos.
- Testes com views falsas no lugar de `App.launch`, como os outros testes do app.
