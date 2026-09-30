# Fluxos

Um **fluxo** encadeia scripts (ver [scripts.md](scripts.md)) com condições, repetições, esperas e triggers. O usuário monta fluxos no **editor visual** da TUI; o formato salvo é YAML, que também é o que a IA gera via MCP (ver ADR-007 em [decisoes.md](decisoes.md)).

## Inspect: o editor visual

`prefixo` + `i` abre o **inspect**: a lista de fluxos à esquerda e, à direita, o fluxograma do fluxo selecionado, de cima para baixo, entre `● start` e `◉ end`:

```
            ● start
               ▼
 ┌─ ↻ explore until ne .steps.dice.number "6" (max 5) ─┐
 │                         ▼                           │
 │             ┌─ ✓ where ────────────────┐            ▲
 │             │ pwd                      │            ▲
 │             └──────────────────────────┘            ▲
 │                         ▼                           ▲
 │             ┌─ ✓ ◇ if ─────────────────┐            ▲
 │             │ eq .steps.pick.color "re │            ▲
 │             └──────────────────────────┘            ▲
 │               ┌─yes───────┴────────no─┐             ▲
 │               ▼                       ▼             ▲
 │ ┌─ ✓ dice ─────────────────┐   ┌────────────┐       ▲
 │ │ tests-noct/dice          │   │  ∅ a adds  │       ▲
 │ └──────────────────────────┘   └────────────┘       ▲
 │                └──────────┬──────────┘              │
 └─────────────────────────────────────────────────────┘
```

- Cada passo é uma caixa com o `id` no título e, abaixo, o que ele roda; o `if` se divide em `yes` (then) e `no` (else) lado a lado e se junta de novo; o `repeat` emoldura os passos que repete, com a condição no título e a seta de volta na lateral. Uma lista vazia aparece como `∅ a adds`, onde `a` coloca o primeiro passo.
- Depois de rodar, cada bloco mostra como foi na última execução: `✓`, `✗` (com a moldura na cor de atenção) ou `–` quando não rodou.
- A lista de fluxos e o fluxograma ficam cada um numa moldura, a que tem o foco na cor de destaque. `Tab` esconde a lista, dando a largura toda ao fluxograma, e a mostra de novo.
- Lista: `↑↓`/`jk` escolhe o fluxo, `Enter` (ou `→`/`l`) vai para o fluxograma, `a` cria um fluxo, `Delete` apaga.
- Fluxograma: `↑↓`/`jk` passa pelos blocos e pelas setas entre eles, `←→`/`hl` vai ao bloco mais próximo ao lado (de um ramo do `if` para o outro), `Esc` volta à lista. As setas antes, entre e depois dos passos de cada lista (a do fluxo, a de um `repeat`, cada ramo de um `if`) são pontos de inserção: selecionada, a seta vira `+`, e `a` adiciona o passo **exatamente ali** (script, linha de comando, se/senão, repetição, espera, prompt ao usuário, `set` ou `unset` de variáveis). Assim dá para pôr um passo antes ou depois de um `repeat` que ocupa o fluxo inteiro. Com um passo selecionado, `a` adiciona logo depois dele. `Enter` ou **clique duplo** num bloco abre um popup com o que ele faz (o script, sua descrição, linha de comando e notas, os `with`, as capturas e o resultado da última execução); `e` edita o passo selecionado sozinho no editor, como YAML; `Delete` remove (com os passos dentro); `J`/`K` movem na mesma lista.
- `+`/`-` (ou `Ctrl` + roda) mudam o zoom: caixas mais estreitas e só o nome, ou mais largas com `with`, descrição e capturas. Arrastar com o mouse, `Ctrl` + setas ou a roda (com `Shift`, na horizontal) movem o fluxograma dentro do painel, para ver qualquer parte em qualquer zoom; `0` centraliza de novo.
- `p` abre o formulário de **params** do fluxo, com o rótulo, o tipo e os últimos valores de cada um: `Enter` confere os valores (números, obrigatórios), lembra e roda; um valor recusado fica no formulário com o motivo na barra. `r` roda com os valores lembrados (ou os padrões) e abre o formulário sozinho quando falta um obrigatório.
- `r` roda o fluxo inteiro num painel ao lado; `t` **testa** só o passo selecionado (com os de dentro, se for `if` ou `repeat`). `Shift+↑↓` estende a seleção pelos passos vizinhos da mesma lista, sempre contínua, e `t` testa a faixa toda, em ordem; qualquer outro movimento ou `Esc` desfaz a faixa. `E` abre o arquivo inteiro no editor.
- `s` mostra ou esconde o painel **scope** à direita do fluxograma (ver [Variáveis e escopo](#variáveis-e-escopo)). Nos prompts de `if`, `repeat` e linha de comando, `Ctrl+V` abre a busca pelas variáveis do escopo e insere a escolhida, como `{{.steps.pick.color}}`.
- Não é um canvas livre de arrastar nós: blocos só se encaixam em sequência, dentro de `se/senão` ou dentro de `repetir`, e todo o fluxo se monta pelo inspect, sem editar o YAML à mão.

## Formato

O fluxo `pedido-completo` fica em `~/.local/share/noct/flows/pedido-completo.yaml`; o nome vem do arquivo. Os `params` são valores com padrão, que `noct flow run <fluxo> nome=valor` troca. Cada um é só o padrão (`env: staging`) ou um mapa com `default`, `type` (`text` ou `number`), `prompt` (o rótulo no formulário) e `required`; `noct flow run` e o formulário recusam um `number` que não é número e um obrigatório vazio.

```yaml
description: Cria usuário, faz pedido e valida o e-mail
params:
  env: staging

steps:
  - id: user
    script: criar-usuario
    with: { env: "{{.env}}" }
    capture:
      user_id: 'id=(\d+)'          # regex sobre a saída; grupo 1 vira variável

  - if: '{{.steps.user.ok}}'
    then:
      - id: order
        script: fazer-pedido
        with: { user: "{{.steps.user.user_id}}" }
        expect:
          exit: 0
          output_matches: 'status=CREATED'

      - id: mail
        repeat:
          every: 10s
          until: '{{match .steps.mail.output "enviado"}}'
          max: 30                  # obrigatório: limite de segurança
        steps:
          - script: checar-email
            with: { user: "{{.steps.user.user_id}}" }
    else:
      - script: notify
        with: { msg: "falha ao criar usuário" }

  - wait: 5s

  - id: logs
    run: kubectl logs -f deploy/api -n {{.env}}
    pane: new
    background: true               # fica rodando; o fluxo não espera

triggers:
  - manual
  - cron: "0 9 * * 1-5"
  - watch: { paths: ["./src/**/*.go"], debounce: 2s }
  - after: { flow: deploy-staging, status: ok }
```

## Semântica

- **Passos** rodam em sequência. Um passo é `script` (script cadastrado), `run` (linha de shell avulsa), `if`, `repeat`, `wait`, `prompt`, `set` ou `unset`.
- **Variáveis:** params, `set` e `prompt` dividem um namespace só, lido como `{{.nome}}` (`steps` não vale como nome). `set: {soma: '{{.steps.add.output}}'}` cria ou troca variáveis, renderizando todos os valores antes de gravar. `unset: [soma, steps.add]` é o garbage collection: apaga variáveis e resultados de passos, que passam a ler vazio.
- **`prompt`** exige intervenção imediata do usuário: pergunta no terminal da execução (`prompt`, um template) e grava a resposta em `var`, nova ou existente (um param também). Resposta vazia fica com o `default` (template); `type: number` pergunta de novo até vir um número, no máximo 3 vezes, e depois falha. No workspace, antes de perguntar, o noct traz o painel da execução para a tela (troca de tab e dá o foco, com um aviso na barra) e toca o sino. Sem ninguém para responder, como numa execução por trigger, o passo falha na hora em vez de esperar.
- **`expect`** define sucesso (exit code e/ou regex na saída). Sem `expect`, sucesso = exit 0.
- **`capture`** extrai valores da saída por regex, linha a linha (`^` e `$` casam em cada linha); a primeira ocorrência vence, e o primeiro grupo, se houver, é o valor. A saída é lida sem cores.
- **Saída:** cada passo roda no shell do usuário e mostra a saída enquanto roda, com um cabeçalho por passo. Para capturá-la, a saída passa por um pipe, então o programa não a vê como terminal (alguns tiram as cores); a entrada continua sendo o terminal (ADR-028).
- **`if`** avalia um template que resulta em `true`/`false`; `else` é opcional.
- **`repeat`** aceita `every` (intervalo entre execuções), `until` (condição de parada, avaliada depois de cada passada) e/ou `times` (número fixo). `max` é obrigatório quando não há `times`, para que nenhum laço rode para sempre; chegar ao `max` sem o `until` ficar verdadeiro é falha. Cada passada começa sem os resultados dos passos de dentro da passada anterior: um passo que não rodou nesta passada lê como vazio.
- **`on_fail`** por passo: `stop` (padrão), `continue` ou `ask`. Com `ask`, a execução pausa no próprio terminal dela (o painel da execução, na TUI) e pergunta `Retry, continue or stop? [r/c/s]`: `r` roda o passo de novo, `c` segue como `continue`, e qualquer outra resposta para. Sem terminal para perguntar, como numa execução por trigger, `ask` para o fluxo.
- **`pane: new`** roda o passo num painel próprio, ao lado do painel da execução e na mesma tab, com o terminal de verdade (cores, entrada, programas interativos); o fluxo espera o programa sair e usa o código de saída. Com **`background: true`** (só junto de `pane: new`), o fluxo não espera: o painel abre e o fluxo segue. Como a saída não passa pelo fluxo, esses passos não têm `capture` nem `expect`. Fora do workspace do noct (um `noct flow run` no shell), não há painel: o passo roda no próprio terminal, e em segundo plano sem que o fluxo espere.
- **Templates:** `text/template` do Go, com `.<param>` (ou variável), `.steps.<id>.<captura>`, `.steps.<id>.ok`, `.steps.<id>.exit`, `.steps.<id>.output` e a função `match`.
- **Triggers** ficam desligados até o usuário habilitar o fluxo nesta máquina (`noct enable <fluxo>`, ou `o` no inspect), para que um fluxo recebido via git não comece a rodar sozinho. O que está habilitado fica em `~/.local/state/noct/triggers.json`, fora do arquivo do fluxo. Ver [Triggers](#triggers).
- **Validação:** `noct check` valida schema, templates, referências a scripts, triggers (cron, caminhos, `after`), laços de `after` entre fluxos e a presença de `max`.

## Triggers

Quem dispara é o daemon, então os triggers funcionam com a TUI fechada (`prefixo` + `d`), mas não com o workspace encerrado (`noct quit`): execuções perdidas nesse tempo não são recuperadas depois.

- **`cron`**: cinco campos como no crontab (minuto, hora, dia, mês, dia da semana), com `*`, listas, faixas, passos (`*/15`) e nomes (`jan`, `mon`); com dia do mês e dia da semana definidos, vale qualquer um dos dois, como no crontab. O minuto em que o daemon sobe não dispara, para que reiniciar não rode um fluxo duas vezes.
- **`watch`**: `paths` são globs (`**` para qualquer número de pastas), relativos à pasta onde o workspace começou, ou absolutos, ou com `~`. O daemon compara, a cada 2 s, os caminhos, tamanhos e horários dos arquivos que casam (sem entrar em `.git`, até 50 mil arquivos por caminho vigiado); quando mudam e ficam parados por `debounce` (padrão 1 s), o fluxo roda.
- **`after`**: roda quando outro fluxo termina (`status: ok`, `failed`, ou omitido para qualquer um), venha a execução de onde vier: da TUI, de outro trigger ou de um `noct flow run` no shell, porque toda execução grava o resultado do fluxo. Um fluxo não pode seguir a si mesmo, e fluxos que se disparam em círculo (a → b → a) ficam com problema, sem rodar.
- **`manual`** só diz que o fluxo também é rodado à mão.
- Uma execução por trigger roda sem painel: a saída vai para um log, a execução entra em `runs.json` com o que a disparou (`flow x (cron 0 9 * * 1-5)` na lista de logs), e um aviso aparece na barra de tabs. Enquanto uma execução disparada por trigger roda, o mesmo fluxo não é disparado de novo.
- No inspect, `T` adiciona um trigger por menus (cron, arquivos ou outro fluxo) e já habilita os triggers do fluxo; `o` os liga e desliga. A lista marca com `⏰` os fluxos com triggers ligados, e o fluxograma mostra quais são.

## Variáveis e escopo

O que um template lê num ponto do fluxo é o **escopo** daquele ponto:

- os parâmetros do fluxo (`.env`) e as variáveis que um `set` ou `prompt` anterior grava, menos as que um `unset` anterior apagou;
- os passos com `id` que rodam antes, na mesma lista e nas listas de fora: `.output`, `.exit`, `.ok`, as capturas e os `outputs` do script;
- passos dentro de um `if` anterior entram como **talvez** (`?`): só existem se aquele ramo rodou;
- dentro de um `repeat`, os passos de dentro só valem da passada atual (`↻`), porque cada passada esquece a anterior; o `until`, avaliado no fim da passada, vê todos eles.

O painel scope (`s` no inspect) lista o escopo do bloco ou do ponto de inserção selecionado, com o valor da última execução ao lado. Referências a variáveis que nada grava, gravadas só depois ou já apagadas, a passos que não existem, que ainda não rodaram naquele ponto, a campos que o passo não tem, ou a passos que talvez não tenham rodado aparecem como **avisos**: `⚠` no bloco, no painel scope e em `noct check`. São avisos e não erros para que o fluxo possa ser montado de cima para baixo (um `until` escrito antes do passo que ele lê).

## Execução e resultado

`noct flow run <fluxo> [nome=valor ...]` roda o fluxo no terminal atual; `r` no inspect roda o mesmo comando num painel ao lado. O código de saída é o do passo que parou o fluxo (ou 1). O resultado da última execução, passo a passo (status, exit code, duração, capturas, a saída e o motivo da falha), fica em `~/.local/state/noct/flows/<fluxo>.json`, e é o que o inspect mostra nos blocos e no painel scope. A saída guardada é a mesma que os templates leem (até 1 MiB por passo); no config (`prefixo` + `c`, **Flows → step output kept**) dá para guardar menos: 256, 64, 16 ou 4 KB.

`noct flow run <fluxo> --steps <de>..<até>` testa uma faixa de passos da mesma lista (`--steps 0/steps/1..0/steps/3`, ou um só: `--steps 2`). Os passos leem `.steps` da última execução, como se o fluxo tivesse rodado até ali, e o resultado substitui só esses passos no último resultado, marcados como testados e com a saída **truncada em 4 KB**.

## Logs e calendário

`prefixo` + `v` abre a view de **logs** (ADR-029, ADR-030):

- À esquerda, o **calendário** do mês: dias sem nada em cinza, dias com logs ou execuções na cor do texto. Cada dia com logs fica sobre uma cor que cresce com a quantidade deles (ADR-038): quatro passos do fundo do tema até o destaque, em relação ao dia com mais logs, cobrindo a célula inteira com o número no meio. Hoje aparece entre colchetes (`[30]`) e o dia selecionado é sublinhado. Logo abaixo das semanas, a legenda (`activity less … more`); depois, os totais do mês e de todos os logs, com a barra de quanto do limite eles ocupam.
- À direita, o **dia selecionado**: logs, peso total, linhas e execuções (e quantas falharam), e a lista dos logs com hora, como a execução terminou (`✓`, `✗`, `●` rodando ou interrompida, nada para um terminal), nome, uma **barra de peso** que vai de 0% no log mais leve a 100% no mais pesado da lista, tamanho legível (B, KB, MB, GB) e linhas.
- Teclas: `Tab` alterna calendário e lista; no calendário, setas mudam o dia, `[`/`]` (ou `PgUp`/`PgDn`) o mês e `t` volta a hoje; na lista, `Enter` (ou clique duplo) abre o log num painel ao lado, desenhado como o programa o mostrou. `s` ordena por data, nome ou peso; `a` alterna entre o dia e todos os dias. `Delete` pergunta o que limpar: o log selecionado, os logs do dia ou todos, sempre com confirmação.
- `f` **filtra** os logs de todos os dias com uma linha só: palavras buscam no nome, na data (`2026-09-27`, `09-27`, `sep 27`, o dia da semana), na hora (`14:30`) e em como a execução terminou (`ok`, `failed`, `running`, o trigger); comparações filtram por peso (`>1MB`, `size<500k`) ou linhas (`>100`, `lines<=20`). Todas as palavras precisam casar; um filtro vazio volta ao dia.
- `i` numa execução de fluxo abre o **fluxograma daquela execução** no inspect, com o estado de cada bloco nela (e o popup de cada passo com a saída dela), não da última.
- Cada execução de script ou fluxo pela TUI, ou por trigger, vai para `runs.json` com o caminho do log e, ao terminar, o código de saída; é assim que a lista sabe como cada log terminou e o calendário conta execuções por dia. Uma execução de fluxo guarda também o resultado dela ao lado do log (`<hora>-flow-<nome>.result.json`), que conta no limite de tamanho e é apagado junto com o log.
- **Limites** (config, seção Logs): logs com mais de 90 dias são apagados, e depois os mais antigos enquanto todos juntos passam de 200 MB. Os dois valores mudam no config (`prefixo` + `c`) ou em `[logs]` no `config.toml` (`keep_days`, `max_size_mb`). A limpeza roda ao iniciar, a cada hora, ao abrir a view e ao mudar os limites.
