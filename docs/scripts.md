# Scripts

Um **script** é uma linha de comando que o usuário roda com frequência, com parâmetros perguntados na hora, a versão do programa que usa e notas de como e quando rodar. É a unidade básica do noct: fluxos (ver [fluxos.md](fluxos.md)) são feitos de scripts. O nome substitui "comando" (ADR-018).

## Arquivos

```
scripts/
  api-login/
    script.yaml   # definição
    notes.md      # como/quando rodar, armadilhas, links
```

Locais:

- `~/.local/share/noct/scripts/` (ou `$XDG_DATA_HOME/noct/scripts/`): pessoais. O local pode ser trocado no config (`Scripts → location`).
- **Pastas de scripts** dentro dela: uma subpasta sem `script.yaml` é uma pasta que agrupa scripts (ex.: `scripts/tests-noct/greet/`). Um nível só: pastas dentro de pastas não são lidas.
- **Pastas vinculadas:** qualquer pasta de scripts em outro lugar, adicionada pelo caminho (`a` no scripts panel; fica em `script_folders` no config). Os arquivos continuam onde estão.
- `.noct/scripts/` do projeto: o mais próximo, subindo a partir do diretório em que o `noct` foi aberto, como o git acha o repositório. Aparece como a pasta `project`.

O nome do script é o nome da sua pasta: letras, dígitos, `.`, `-` e `_`. O **ID** de um script é `pasta/nome` dentro de uma pasta, ou só `nome` fora delas; é por ele que valores lembrados, histórico de execuções e `noct -s` o identificam. Scripts com o mesmo nome em pastas diferentes convivem.

## Formato

```yaml
description: Faz login na API e mostra o token
program: curl                  # opcional: programa cuja versão e ajuda o noct mostra
version_cmd: curl --version    # opcional; padrão: tenta --version e -v
run: ./login.sh {{.env}} {{.user}} {{.password}}
workdir: .                     # opcional; relativo à pasta do script. Vazio: onde o noct foi aberto

params:
  - name: env
    prompt: Environment        # texto do formulário; padrão: o nome
    choices: [dev, staging, prod]
    default: staging           # sem default, vale a primeira escolha
  - name: user
    prompt: Test user
    remember: true             # começa com o último valor usado
  - name: password
    from: env:TEST_PASSWORD    # lido do ambiente, não perguntado
    secret: true               # mascarado no formulário, nunca lembrado

outputs: [token, expires]      # opcional: o que o script devolve a um fluxo, em linhas nome=valor
```

- **`run`** é um `text/template` do Go, renderizado com os valores dos parâmetros (`{{.nome}}`); um parâmetro que não existe é erro. A linha roda com `$SHELL -c`, então pipes, `&&` e variáveis de ambiente funcionam como no terminal.
- **Valor inicial** de cada parâmetro: o ambiente para `from: env:`; o último valor usado se `remember: true`; senão o `default`; senão a primeira das `choices`.
- **`outputs`** são o "retorno" do script num fluxo: para cada nome, a primeira linha `nome=valor` da saída vira `{{.steps.<id>.nome}}`, sem escrever um `capture` com regex. Um `capture` do passo com o mesmo nome vence. Os nomes aparecem no painel scope do inspect ([fluxos.md](fluxos.md#variáveis-e-escopo)).
- **Valores lembrados** ficam em `~/.local/state/noct/params.json`, compartilhados pela TUI e pela CLI. Segredos nunca são gravados.
- **Validação:** `run` não vazio, nomes de parâmetro como identificadores e sem repetição, `default` dentro das `choices`, `from` no formato `env:NOME`, `outputs` como identificadores, template que compila e só usa parâmetros que existem. Um script inválido não some: aparece como problema no scripts panel e em `noct check`.

## Na TUI

O **scripts panel** (`prefixo` + `s`) lista os scripts com a versão do programa à direita; num painel largo, mostra ao lado a descrição, a linha `run`, a origem (pessoal ou projeto) e as notas.

| Tecla | Ação |
|---|---|
| `↑↓` / `jk`, `g` / `G`, clique, roda do mouse | Selecionar |
| `Enter` | Rodar: abre o formulário de parâmetros, se houver algum a perguntar |
| `e` | Editar no formulário: nome, script (a linha `run`) e descrição |
| `n` / `Ctrl+N` | Novo script, no mesmo formulário vazio |
| `Ctrl+O` | Abrir a pasta dos scripts no gerenciador de arquivos do config (padrão: um terminal na pasta) |
| `Delete` | Apagar, depois de confirmar |
| `Ctrl+D` | Fechar o painel |

A biblioteca é relida ao abrir um scripts panel, quando o editor fecha e depois de criar ou apagar.

**Scripts quebrados.** Um script que não carrega (YAML ou template com erro) continua na lista, com `✗` e `broken` no lugar da versão; ao lado aparece o erro. `Enter` não o roda, e `e` o abre direto no editor para corrigir. Antes de cada edição (formulário ou editor), o noct guarda o `script.yaml` que funciona em `script.yaml.bak`, e nunca sobrescreve essa cópia com uma versão quebrada. Quando o editor fecha deixando o script quebrado, a barra de tabs mostra o erro e onde está a última versão que funcionava. `noct ls` mostra os quebrados com `✗`, e `noct check` os lista.

**Formulário do script** (`e`, `n`). Em cima, `name` e `program`, digitados na própria linha. Embaixo, quatro caixas com bordas próprias, duas a duas, cada uma com metade da largura: `script` (a linha `run`) ao lado de `description`, e `params` ao lado de `notes`. `Enter` numa caixa abre só aquela parte no editor do config (padrão `vim`; ver [tui.md](tui.md)); ao fechar o editor, a parte volta para a caixa e o formulário reaparece, para editar a próxima. `Tab` segue a ordem, as setas andam pelas caixas como estão dispostas, `Ctrl+S` salva tudo e `Esc` descarta.

`params` é editado como a lista YAML de parâmetros (com um exemplo comentado quando ainda não há nenhum). Salvar valida o conjunto: nome, script não vazio, `params` que é YAML válido e todo `{{.nome}}` do script correspondendo a um parâmetro. Salvar reescreve no `script.yaml` só o que mudou, mantendo os comentários, grava `notes.md` e renomeia a pasta se o nome mudou (os valores lembrados vão junto).

**Scripts novos** começam com um comentário de 4 linhas explicando `description`, `program`, `run` e `params`, e o conteúdo de verdade logo abaixo. O comentário pode ser desligado no config (`Scripts → hints in new scripts`).

**Formulário de parâmetros.** `Tab`/`↑↓` trocam de campo, `←→` escolhem entre as `choices` (num campo de texto, movem o cursor; as demais teclas de edição estão em [tui.md](tui.md#campos-de-texto)), `Enter` roda e `Esc` cancela. Embaixo aparece a linha que vai rodar, com segredos e valores do ambiente mascarados.

**Execução.** O script roda num terminal de verdade ao lado do painel, com foco nele, então prompts, cores e `Ctrl+C` funcionam. Uma linha de estado embaixo, na cor das dicas e fora da seleção de texto, mostra `running`; quando o script termina, o painel continua aberto com a saída, o título mostra `✓` ou `✗ <código>`, `r` roda de novo e `Enter`/`Esc`/`q` fecham. A saída fica gravada em `~/.local/state/noct/logs/<data>/`, como a dos terminais.

**Pastas na lista.** As pastas vêm sempre primeiro, em ordem alfabética, e começam fechadas (`+`); abertas (`-`), mostram seus scripts logo abaixo, atrás de uma barra vertical `│`. Depois vêm os scripts fora de pastas. A pasta onde a seleção está (ela mesma ou um script dela) aparece, com sua barra, na cor de acento. `Enter`, `Space` ou um clique abrem e fecham uma pasta; `←` fecha (ou, num script dentro de pasta, volta para a pasta) e `→` abre. `n` pergunta o que criar: `directory` (`d`, uma pasta na pasta de scripts) ou `script` (`s`, na pasta selecionada ou na do script selecionado); `a` adiciona uma pasta vinculada e `Ctrl+O` abre a pasta selecionada. `Delete` numa pasta apaga a pasta e seus scripts (depois de confirmar); numa pasta vinculada, só deixa de mostrá-la.

**Ordem da lista.** A ordem vale dentro de cada pasta e para os scripts fora delas. Alfabética por padrão; o config (`Scripts → sort`) troca para a última edição ou a última execução. Cada execução, pela TUI ou por `noct -s`, entra em `~/.local/state/noct/runs.json` (nome e horário), que guarda só as últimas 1000.

**Busca** (`prefixo` + `Space`): busca fuzzy sobre os scripts, as ações do modo noct e as tabs abertas; `Enter` roda o script, executa a ação ou vai para a tab.

## Versão

1. `exec.LookPath(program)`: se não achar, mostra `(not found)`.
2. Se achar, roda `version_cmd` (com `sh -c`), ou tenta `--version` e `-v`, nessa ordem, com timeout de 2 s, lendo stdout e stderr. Um argumento `version` solto não é tentado: para muitos programas ele é um nome de arquivo.
3. Mostra a primeira sequência parecida com versão (`\d+(\.\d+)+`); se não houver, a primeira linha de uma execução que deu certo, ou `?`.
4. A detecção roda em segundo plano (a lista mostra `…` enquanto isso) e o resultado fica em cache até o binário mudar (caminho + data de modificação).

## Ajuda (`prefixo` + `?`)

1. Descobre o programa em primeiro plano no terminal com foco pelo grupo de processos do PTY (`tcgetpgrp`), não pelo processo que o noct iniciou: com o zsh esperando e o `htop` rodando, o que interessa é o `htop`.
2. Abre num painel ao lado, no `$PAGER` (padrão `less`): as notas de um script cujo `program` seja esse, a saída de `programa --help` e a página do `man`, nessa ordem.
3. Se o terminal está só no shell, não há programa para explicar: o popup do modo noct continua aberto, com um aviso na barra de tabs.

## CLI

`noct -h` (ou `--help`) mostra os comandos e flags.

Rodam no terminal atual, sem o daemon, a partir do diretório atual (que decide os scripts do projeto):

| Comando | O que faz |
|---|---|
| `noct ls` | Lista o ID, a versão do programa e a descrição; quebrados com `✗` |
| `noct check` | Valida todos os scripts; sai com erro e lista os problemas se algum não carregar |
| `noct -s <script> [nome=valor ...]` (ou `--script`, ou `noct run`) | Roda o script com `$SHELL -c`, saindo com o código dele. `<script>` é o ID (`pasta/nome`) ou só o nome, quando um único script o tem. Se não existir, diz `Script not found at <locais>`. Parâmetros não passados são perguntados se a entrada é um terminal (Enter mantém o valor mostrado, segredos sem eco); senão ficam com o valor inicial |

## Segredos

`secret: true` mascara o valor no formulário e na prévia, e ele nunca vai para `params.json`. O valor entra na linha de comando renderizada, então aparece para quem lista os processos da máquina (`ps`), e a saída do script é gravada como o programa a imprimir. Para não expor um segredo, use a variável de ambiente direto em `run` (`curl -u "$USER:$TEST_PASSWORD"`), que o shell expande sem passar pelo noct, em vez de um parâmetro.
