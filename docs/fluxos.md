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

- Cada passo é um cartão de cantos arredondados com o `id` no título e, abaixo, o que ele roda, ligado ao seguinte por uma seta de três linhas (`│`, `│`, `↓`), com os traços de caixa e as setas que fontes como a Fira Code desenham inteiros; o `if` se divide em `yes` (then) e `no` (else) lado a lado e se junta de novo; o `repeat` emoldura os passos que repete, com a condição no título e a seta de volta na lateral. Uma lista vazia aparece como `∅ a adds`, onde `a` coloca o primeiro passo.
- Depois de rodar, cada bloco mostra como foi na última execução: `✓`, `✗` (com a moldura na cor de atenção) ou `–` quando não rodou.
- A lista de fluxos e o fluxograma ficam cada um numa moldura, a que tem o foco na cor de destaque. `Tab` esconde a lista, dando a largura toda ao fluxograma, e a mostra de novo.
- Lista: `↑↓`/`jk` escolhe o fluxo, `Enter` (ou `→`/`l`) vai para o fluxograma, `a` cria um fluxo, `Delete` apaga.
- Fluxograma: `↑↓`/`jk` passa pelos blocos, `←→`/`hl` vai ao bloco mais próximo ao lado (de um ramo do `if` para o outro), `Esc` volta à lista. Com um passo selecionado, `a` adiciona logo **abaixo** dele (script, linha de comando, se/senão, repetição, espera, prompt ao usuário, `set` ou `unset` de variáveis); com um `if` ou `repeat` selecionado, abaixo do bloco inteiro. Para pôr um passo **acima** de todos, seleciona-se a cabeça da lista: o `● start` do fluxo, o `yes` ou o `no` de um `if`, a seta de entrada de um `repeat`. Não há pontos de inserção entre os cartões para atravessar. `Enter` ou **clique duplo** num bloco abre um popup com o que ele faz (o script, sua descrição, linha de comando e notas, os `with`, as capturas e o resultado da última execução); `e` edita o passo selecionado sozinho no editor, como YAML; `Delete` remove (com os passos dentro).
- **Mover:** `J`/`K` levam o passo um lugar para baixo ou para cima pelo fluxo, como uma linha de código: passam pelo vizinho, entram num `if` ou `repeat` vizinho, vão do `yes` para o `no` e saem do bloco no fim dele. `m` liga o modo de mover, em que as setas fazem o mesmo, com `↕` no título do cartão, até `m`, `Enter` ou `Esc`. Com o mouse, arrastar um cartão e soltar sobre outro o põe logo abaixo dele; sobre uma cabeça de lista ou uma lista vazia, no topo dela; o lugar onde ele vai cair fica na cor de atenção. Arrastar fora dos cartões move o fluxograma.
- **Desfazer:** `z` desfaz a última mudança feita no fluxo pelo inspect (adicionar, mover, remover, editar um passo) e `y` refaz, até 100 mudanças por fluxo enquanto o noct roda.
- `+`/`-` (ou `Ctrl` + roda) mudam o zoom: caixas mais estreitas e só o nome, ou mais largas com `with`, descrição e capturas. Arrastar com o mouse, `Ctrl` + setas ou a roda (com `Shift`, na horizontal) movem o fluxograma dentro do painel, para ver qualquer parte em qualquer zoom; `0` centraliza de novo.
- `p` abre o formulário de **params** do fluxo, com o rótulo, o tipo e os últimos valores de cada um: `Enter` confere os valores (números, obrigatórios), lembra e roda; um valor recusado fica no formulário com o motivo na barra. `r` roda com os valores lembrados (ou os padrões) e abre o formulário sozinho quando falta um obrigatório.
- `r` roda o fluxo inteiro num painel ao lado; `t` **testa** só o passo selecionado (com os de dentro, se for `if` ou `repeat`). `Shift+↑↓` estende a seleção pelos passos vizinhos da mesma lista, sempre contínua, e `t` testa a faixa toda, em ordem; qualquer outro movimento ou `Esc` desfaz a faixa. `E` abre o arquivo inteiro no editor.
- `s` mostra ou esconde o painel **scope** à direita do fluxograma (ver [Variáveis e escopo](#variáveis-e-escopo)). Nos prompts de `if`, `repeat` e linha de comando, `Ctrl+T` abre a busca pelas variáveis do escopo e insere a escolhida, como `{{.steps.pick.color}}`.
- Não é um canvas livre de arrastar nós: blocos só se encaixam em sequência, dentro de `se/senão` ou dentro de `repetir`, e todo o fluxo se monta pelo inspect, sem editar o YAML à mão.

## Formato

O fluxo `pedido-completo` fica em `~/.local/share/noct/flows/pedido-completo.yaml`; o nome vem do arquivo. Os `params` são valores com padrão, que `noct flow run <fluxo> nome=valor` troca. Cada um é só o padrão (`env: staging`) ou um mapa com `default`, `type` (`text` ou `number`), `prompt` (o rótulo no formulário), `required`, `from` e `secret`; `noct flow run` e o formulário recusam um `number` que não é número e um obrigatório vazio. Com `from: env:NOME`, o valor vem da variável de ambiente quando ela existe, e o formulário não pergunta; sem ela, o param é perguntado como qualquer outro (ADR-063). `secret: true` mascara no formulário e nunca lembra o valor.

```yaml
description: Cria usuário, faz pedido e valida o e-mail
params:
  env: staging
timeout: 1h                        # a execução inteira; sem ele, não há limite

steps:
  - id: user
    script: criar-usuario
    with: { env: "{{.env}}" }
    timeout: 5m                    # só este passo
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
- **`timeout`** (ADR-061), uma duração do Go (`30s`, `10m`, `1h30m`), no fluxo e nos passos `script` e `run`; sem ele não há limite. Um passo que passa do seu é parado (SIGTERM e, 2 s depois, SIGKILL; numa execução sem terminal, como a de um trigger, o grupo de processos inteiro, então os filhos também; com terminal, só o shell do passo, que precisa ficar no grupo do terminal para ler a entrada) e falha com `timed out after 10m` e exit 124, como o `timeout(1)`; daí segue o `on_fail` como qualquer falha. O do fluxo cobre a execução inteira, `wait` e as pausas do `repeat` também: ao passar, o passo que roda é parado e a execução termina com falha (`flow timed out after 1h`, exit 124), seja qual for o `on_fail`. Um `prompt` ou a pergunta do `on_fail: ask` esperando no terminal não são interrompidos; o tempo é conferido de novo quando a resposta vem. Num passo com `pane: new`, o workspace para o programa do painel no tempo, e a saída continua no painel; `background: true` não tem `timeout`, porque o fluxo não espera por ele.
- **`pane: new`** roda o passo num painel próprio, ao lado do painel da execução e na mesma tab, com o terminal de verdade (cores, entrada, programas interativos); o fluxo espera o programa sair e usa o código de saída. Com **`background: true`** (só junto de `pane: new`), o fluxo não espera: o painel abre e o fluxo segue. Como a saída não passa pelo fluxo, esses passos não têm `capture` nem `expect`. Fora do workspace do noct (um `noct flow run` no shell), não há painel: o passo roda no próprio terminal, e em segundo plano sem que o fluxo espere.
- **Templates:** `text/template` do Go, com `.<param>` (ou variável), `.steps.<id>.<captura>`, `.steps.<id>.ok`, `.steps.<id>.exit`, `.steps.<id>.output` e as funções `match` e `quote` (o valor como uma palavra só do shell, para usar a saída de um passo num `run` sem que ela vire comando).
- **Triggers** ficam desligados até o usuário habilitar o fluxo nesta máquina (`noct enable <fluxo>`, ou `o` no inspect), para que um fluxo recebido via git não comece a rodar sozinho. O que está habilitado fica em `~/.local/state/noct/triggers.json`, fora do arquivo do fluxo. Ver [Triggers](#triggers).
- **Validação:** `noct check` valida schema, templates, referências a scripts, triggers (cron, caminhos, `after`), laços de `after` entre fluxos, a presença de `max` e os `timeout`. Avisa também do fluxo com trigger (`cron`, `watch` ou `after`) sem nenhum `timeout`, nem no fluxo nem num passo.

## Triggers

Quem dispara é o daemon, então os triggers funcionam com a TUI fechada (`prefixo` + `d`), mas não com o workspace encerrado (`noct quit`): execuções perdidas nesse tempo não são recuperadas depois.

- **`cron`**: cinco campos como no crontab (minuto, hora, dia, mês, dia da semana), com `*`, listas, faixas, passos (`*/15`) e nomes (`jan`, `mon`); com dia do mês e dia da semana definidos, vale qualquer um dos dois, como no crontab. O minuto em que o daemon sobe não dispara, para que reiniciar não rode um fluxo duas vezes. O cron segue o relógio local, e no horário de verão (ADR-061): quando o relógio volta uma hora, os minutos dela se repetem, e o cron não olha de novo um minuto do relógio igual ou anterior ao último que já olhou, então `30 1 * * *` roda uma vez só e um `*/15` fica parado até o relógio passar de onde estava; quando o relógio adianta, os minutos pulados não existem, e o que estava marcado neles não roda naquele dia, como no cron clássico.
- **`watch`**: `paths` são globs (`**` para qualquer número de pastas), relativos à pasta onde o workspace começou, ou absolutos, ou com `~`. O daemon compara, a cada 2 s, os caminhos, tamanhos e horários dos arquivos que casam (sem entrar em `.git`, até 50 mil arquivos por caminho vigiado); quando mudam e ficam parados por `debounce` (padrão 1 s), o fluxo roda.
- **`after`**: roda quando outro fluxo termina (`status: ok`, `failed`, ou omitido para qualquer um), venha a execução de onde vier: da TUI, de outro trigger ou de um `noct flow run` no shell, porque toda execução grava o resultado do fluxo. Um fluxo não pode seguir a si mesmo, e fluxos que se disparam em círculo (a → b → a) ficam com problema, sem rodar.
- **`manual`** só diz que o fluxo também é rodado à mão.
- Uma execução por trigger roda sem painel: a saída vai para um log, a execução entra em `runs.json` com o que a disparou (`flow x (cron 0 9 * * 1-5)` na lista de logs), e um aviso aparece na barra de tabs. Enquanto uma execução disparada por trigger roda, o mesmo fluxo não é disparado de novo; por isso uma execução travada segura os triggers do fluxo até terminar, e um `timeout` garante que ela termine.
- No inspect, `T` adiciona um trigger por menus (cron, arquivos ou outro fluxo) e já habilita os triggers do fluxo; `o` os liga e desliga. A lista marca com `⏰` os fluxos com triggers ligados, e o fluxograma mostra quais são.

## Variáveis e escopo

O que um template lê num ponto do fluxo é o **escopo** daquele ponto:

- os parâmetros do fluxo (`.env`) e as variáveis que um `set` ou `prompt` anterior grava, menos as que um `unset` anterior apagou;
- os passos com `id` que rodam antes, na mesma lista e nas listas de fora: `.output`, `.exit`, `.ok`, as capturas e os `outputs` do script;
- passos dentro de um `if` anterior entram como **talvez** (`?`): só existem se aquele ramo rodou;
- dentro de um `repeat`, os passos de dentro só valem da passada atual (`↻`), porque cada passada esquece a anterior; o `until`, avaliado no fim da passada, vê todos eles.

O painel scope (`s` no inspect) lista o escopo do bloco ou do ponto de inserção selecionado, com o valor da última execução ao lado. Referências a variáveis que nada grava, gravadas só depois ou já apagadas, a passos que não existem, que ainda não rodaram naquele ponto, a campos que o passo não tem, ou a passos que talvez não tenham rodado aparecem como **avisos**: `⚠` no bloco, no painel scope e em `noct check`. São avisos e não erros para que o fluxo possa ser montado de cima para baixo (um `until` escrito antes do passo que ele lê).

## Execução e resultado

`noct flow run <fluxo> [nome=valor ...]` roda o fluxo no terminal atual; `r` no inspect roda o mesmo comando num painel ao lado. O código de saída é o do passo que parou o fluxo (ou 1). O resultado da última execução, passo a passo (status, exit code, duração, capturas, a saída e o motivo da falha), fica em `~/.local/state/noct/flows/<fluxo>.json`, e é o que o inspect mostra nos blocos e no painel scope. A saída guardada é a mesma que os templates leem (até 1 MiB por passo); no config (`prefixo` + `,`, **Flows → step output kept**) dá para guardar menos: 256, 64, 16 ou 4 KB.

**Aviso ao fim de uma execução.** Quando um script ou fluxo termina fora de vista, o noct avisa: execuções num painel que não aparece na tela (outra tab, painel dobrado, terminal desconectado) e fluxos disparados por trigger, que nunca têm painel. O texto diz o nome e como terminou (`deploy finished`, `flow nightly (cron) failed with exit 1`). O config (**Flows → tell when runs end**, `notify` no arquivo) escolhe como:

- `system` (padrão): a notificação do sistema, com `notify-send` no Linux e `osascript` no macOS. Sem essa ferramenta, ou com o terminal conectado por ssh (`SSH_CONNECTION`), vai como OSC 9 para o terminal, que kitty, WezTerm, Ghostty, foot, iTerm2 e Windows Terminal mostram como notificação do sistema. Terminais sem OSC 9 ignoram.
- `bell`: o sino do terminal (`BEL`). Cada terminal decide o que faz com ele: som, piscar ou marcar a janela, e alguns não fazem nada.
- `off`: nenhum aviso.

`noct flow run <fluxo> --steps <de>..<até>` testa uma faixa de passos da mesma lista (`--steps 0/steps/1..0/steps/3`, ou um só: `--steps 2`). Os passos leem `.steps` da última execução, como se o fluxo tivesse rodado até ali, e o resultado substitui só esses passos no último resultado, marcados como testados e com a saída **truncada em 4 KB**.

## Logs e calendário

`prefixo` + `v` abre a view de **logs** (ADR-029, ADR-030):

- À esquerda, o **calendário** do mês: dias sem nada em cinza, dias com logs ou execuções na cor do texto. Cada dia com logs fica sobre uma cor que cresce com a quantidade deles (ADR-038): quatro passos do fundo do tema até o destaque, em relação ao dia com mais logs, cobrindo a célula inteira com o número no meio. Hoje aparece entre colchetes (`[30]`) e o dia selecionado é sublinhado. Logo abaixo das semanas, a legenda (`activity less … more`); depois, os totais do mês e de todos os logs, com a barra de quanto do limite eles ocupam.
- À direita, o **dia selecionado**: logs, peso total, linhas e execuções (e quantas falharam), e a lista dos logs em colunas com título (ADR-059): última escrita (`edited`), início (`started`, com a data quando algum log da lista é de outro dia), como a execução terminou (`✓`, `✗`, `●` rodando ou interrompida, nada para um terminal), nome, sessão, uma **barra de peso** que vai de 0% no log mais leve a 100% no mais pesado da lista, tamanho legível (B, KB, MB, GB) e linhas (`12,345`, `300k`, `1.2M`). Quais colunas e em que ordem fica em `[logs] columns`; numa lista estreita as menos importantes saem primeiro. Um log aparece em cada dia em que teve saída: o log anota num arquivo ao lado (`<hora>-<nome>.days`) cada dia novo em que escreve, então um terminal aberto há três dias e usado anteontem e hoje está nesses três dias, e não no dia em que ficou parado; o filtro por data também o acha neles. A lista ordena pela última escrita.
- Teclas: `Tab` alterna calendário e lista; no calendário, setas mudam o dia, `[`/`]` (ou `PgUp`/`PgDn`) o mês, `t` volta a hoje e `Enter` ou `Space` vão à lista; na lista, `PgUp`/`PgDn` passam página, `Home`/`End` vão ao primeiro e ao último log (`[`/`]` continuam mudando o mês), `Esc`, `←` ou `h` voltam ao calendário.
- **O log selecionado**: metade da view, embaixo, mostra sempre o log selecionado, desenhado como o programa o mostrou. `p` o leva para a direita, para cima, para a esquerda e de volta para baixo, `+`/`-` mudam de 10 em 10% quanto da view ele ocupa (20 a 80%), e os dois ficam no config (`[logs] preview`, `preview_percent`); ao lado, sem espaço para o calendário, ele vai para baixo. `z` mostra só o log (e de novo, tudo), `v` esconde o log ou o mostra. Ele acompanha a seleção sem tirar o foco da lista e segue o fim do arquivo, então uma execução que ainda roda aparece ao vivo. `Enter` ou `Space` na lista (ou clique duplo, ou clique no log) dão o foco ao log, no fim; `u` faz o mesmo já uma página acima. Com o foco no log, `↑↓`/`jk` (e `Ctrl+Y`/`Ctrl+E`) rolam linha, `Ctrl+U` meia página, `PgUp`/`PgDn` (e `Ctrl+B`/`Ctrl+F`) página, `g`/`Home` vão ao início do log, `Shift+G`/`End` ao fim, e `Enter`, `Space`, `Esc` (ou `q`) voltam à lista. `/` busca no log, do fim do que aparece para cima, como o `?` do `less`, porque as linhas novas ficam embaixo; `n` segue para cima e `N` volta para baixo, os trechos encontrados ficam em vídeo reverso e, sem mais nenhum, a barra avisa. `f` mostra só as linhas com um texto (o log recomeça por `grep -F`, e continua seguindo o fim), com `filter: …` no título. Nos dois, texto em minúsculas acha qualquer caixa; com uma maiúscula, só a dele. Com uma busca ou um filtro, o primeiro `Esc` os desfaz, e o segundo volta à lista. A busca vale na parte do log que aparece; o filtro, no log inteiro. Trocar de log desfaz os dois. `t` mostra, antes de cada linha, quando ela começou (`2026-11-11 09-30-00`, esmaecido), e `times` no título; de novo, esconde. O log guarda isso num arquivo ao lado (`.log.times`): uma linha `número época` cada vez que uma linha começa num segundo depois do último registro, então uma rajada de saída custa um registro só. Linhas de logs de antes desta versão aparecem com o espaço do horário em branco. O preview com horários é impresso por `noct log-times`, um comando interno; um programa de tela cheia no log fica bagunçado com eles, porque o horário entra antes de cada quebra de linha. Um log longo é mostrado em partes de 5 000 linhas: rolar além do topo ou do fim da parte abre a de antes ou a de depois, e o título diz quais linhas estão à vista (`lines 7,346–12,345 of 12,345`), então toda linha que a lista conta pode ser vista; a roda do mouse rola o log sem tirar o foco da lista. `Ctrl+D` continua fechando o painel. A posição (`↑ 120/2000 rows`) fica à direita da linha do título, em linhas do terminal que desenha o log: linhas longas ocupam mais de uma, e um log que termina num programa de tela cheia soma a tela principal inteira de antes dele, então não bate com as linhas do arquivo. Um log que termina dentro de um programa de tela cheia (a tela alternativa, de TUIs, `nano`, `less`) mostra `▣ ends in a full-screen program` e, subindo, a tela principal de antes do programa e o histórico acima dela; os redesenhos do programa nunca viram histórico.
- **Legendas**: embaixo, só as teclas de andar pela parte com foco (calendário, lista ou log) e de ir à próxima; `?` abre um popup com todas as teclas da view e o que cada uma faz, como estão nas binds (ADR-059). Como as outras, `?` muda em **binds of every screen → logs**.
- `f` **filtra** os logs de todos os dias com uma linha só: palavras buscam no nome, na data (`2026-09-27`, `09-27`, `sep 27`, o dia da semana), na hora (`14:30`) e em como a execução terminou (`ok`, `failed`, `running`, o trigger); comparações filtram por peso (`>1MB`, `size<500k`) ou linhas (`>100`, `lines<=20`). Todas as palavras precisam casar; um filtro vazio volta ao dia.
- `i` numa execução de fluxo abre o **fluxograma daquela execução** no inspect, com o estado de cada bloco nela (e o popup de cada passo com a saída dela), não da última.
- Cada execução de script ou fluxo pela TUI, ou por trigger, vai para `runs.json` com o caminho do log e, ao terminar, o código de saída; é assim que a lista sabe como cada log terminou e o calendário conta execuções por dia. Uma execução de fluxo guarda também o resultado dela ao lado do log (`<hora>-flow-<nome>.result.json`), que conta no limite de tamanho e é apagado junto com o log.
- **Limites** (config, seção Logs): logs sem escrita há mais de 90 dias são apagados, e depois os escritos há mais tempo enquanto todos juntos passam de 200 MB; um terminal aberto há meses e ainda em uso fica. Os dois valores mudam no config (`prefixo` + `,`): `←`/`→` passam pelos valores prontos, e `Enter` ou um dígito abre o valor para digitar qualquer um (`99` dias; `222 MB`, `20000 KB` ou `2 GB`, MB quando não há unidade), que `Enter` salva e `Esc` descarta. Se o limite novo apagaria logs, o config pergunta antes, dizendo quantos e quanto ocupam; embaixo dos limites, uma barra mostra quanto os logs já ocupam do tamanho máximo. No `config.toml`, ficam em `[logs]` (`keep_days`, `max_size_kb`; o antigo `max_size_mb` ainda é lido). A saída de painéis de terminal só entra nos logs com `terminals` ligado (desligado por padrão); scripts, fluxos e comandos da IA sempre são gravados. A limpeza roda ao iniciar, a cada hora, ao abrir a view e ao mudar os limites.
