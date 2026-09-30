# TUI

## Views

| View | O que mostra | Fase |
|---|---|---|
| **Home** | Aparece quando não há tabs. Ao conectar, o título `noct` surge de `0`s e `1`s com o formato das letras, revelados da esquerda para a direita com um leve atraso aleatório por coluna, em 1 s. Passar o mouse sobre um caractere do título ou do corvo faz só esse caractere piscar entre `0` e `1` por 0,4 s; espaços vazios não reagem. Só na home o noct pede ao terminal todo movimento do mouse (modo 1003); fora dela, só o arrasto com botão pressionado. Depois: o título `noct` em blocos no topo, o corvo logo abaixo na cor das dicas e, embaixo, por 3 s depois de qualquer tecla, a tecla prefixo que abre os comandos. Some enquanto um menu está aberto. `Ctrl+D` duas vezes seguidas sai do noct. | 0 |
| **Workspace** | Tabs no topo, painéis em splits. Cada painel mostra uma *view*: um terminal real (shell, Claude, vim) ou um painel do próprio noct. | 0 |
| **Scripts panel** | Painel do noct onde terminam os recursos de multiplexador e começam os de automação. Lista os scripts (pessoais e do projeto) com a versão do programa ou `(not found)`; ao lado, descrição, linha `run` e notas. `Enter` abre o formulário de parâmetros e roda num painel ao lado (ver [scripts.md](scripts.md)). Os fluxos (fase 2) vêm para cá também. | 1 |
| **Busca** | `prefixo` + `Space`: busca fuzzy sobre scripts, ações do modo noct e tabs. | 1 |
| **Ajuda** | `prefixo` + `?`: a ajuda do programa em primeiro plano no terminal com foco (notas, `--help`, `man`), num painel com o pager. | 1 |
| **Inspect** | `prefixo` + `i`: a lista de fluxos e o fluxograma do selecionado, que se monta passo a passo, se arrasta e tem zoom; roda o fluxo e mostra em cada bloco como foi a última execução; `p` abre o formulário de params (tipos, obrigatórios, últimos valores) e roda; `T` adiciona triggers e `o` os liga e desliga (ver [fluxos.md](fluxos.md)). | 2 |
| **Execuções** | O que está rodando agora, com status, duração e passo atual. Enter salta para o painel. | 2 |
| **Logs** | `prefixo` + `v`: o calendário do mês, cada dia marcado pelo que rodou nele, e os logs do dia com barra de peso, tamanho, linhas e como a execução terminou; abrir, ordenar, filtrar (`f`: data, nome, status, peso ou linhas numa linha só), abrir uma execução de fluxo no fluxograma (`i`) e limpar (ver [fluxos.md](fluxos.md#logs-e-calendário)). | 2 |
| **Aprovações** | Pedidos pendentes da IA ou de triggers, com o comando completo; aprovar/negar. | 3 |
| **Catálogo** | Programas instalados, faltando e recomendados; instalar abre um painel com o gerenciador escolhido. | 4 |

## Tabs e painéis

- O noct abre na **home**. `prefixo` + `t` cria uma tab já com um painel de terminal; `p` (terminal) e `s` (scripts) abrem mais painéis na tab atual; na home, abrem uma tab com o painel.
- Os painéis se organizam como no i3 com autotiling (ADR-022): um painel novo abre ao lado do painel com foco se ele é mais largo que alto (em proporção de célula), senão embaixo. Se o grupo do painel com foco já está nessa direção, o novo entra no grupo e todos dividem o espaço.
- Uma tab fecha junto com o último painel, seja com `x`, saindo do shell com `Ctrl+D` ou movendo o painel para outra tab; a seleção vai para a tab que ocupa o lugar dela. Sem tabs, aparece a home (ADR-019).
- `Ctrl+D` fecha o scripts panel, como sai de um shell.
- Para sair do noct: `prefixo` + `q` e confirmar com `Enter` ou `q`, ou `Ctrl+D` duas vezes na home (o segundo enquanto a dica `ctrl+d again to quit` está visível).
- Na home sem tabs, `p` e `s` abrem uma tab com o painel.
- Tabs e painéis ilimitados; um terminal novo abre o shell do usuário (`$SHELL`) com a configuração dele, no diretório em que o `noct` foi aberto.
- Mudar o foco pelas setas vai para o painel mais próximo naquela direção; entre painéis igualmente próximos (ex.: 2 sobre 3 à direita do 1), vai para o que teve o foco por último, como no i3, e só sem histórico para o mais alinhado. Assim, ir e voltar retorna ao painel de onde se saiu.
- `prefixo` + `a` (ou `Alt+Shift` + setas ou `hjkl`) move o painel com foco como o `move` do i3: dentro de um grupo na direção da seta, troca de lugar com o vizinho ou entra nele, se o vizinho for um grupo; na borda do grupo, sai e fica ao lado dele. Ex.: com o painel 1 à esquerda e 2 sobre 3 à direita, mover o 2 para a esquerda deixa 1 | 2 | 3; mover de novo troca 2 e 1. Na borda da tab, `←`/`→` levam o painel para a tab anterior ou seguinte, ou para uma tab nova.
- `prefixo` + `r` renomeia a tab; um nome vazio volta a seguir o título do painel com foco.
- Quando as tabs não cabem na barra, os nomes longos são cortados com `…` numa mesma largura, a maior que deixa todas visíveis; nomes curtos ficam inteiros. Se nem com nomes de 6 colunas cabem, a barra rola e mostra só um trecho de tabs, sempre com a tab selecionada visível; `‹` e `›` nas pontas indicam tabs escondidas, e clicar neles seleciona a tab escondida mais próxima. A barra só rola quando a tab selecionada sairia dela, e deixa espaço à direita para o selo do modo e as mensagens.
- `prefixo` + `e` renomeia o painel com foco; um nome vazio volta ao título do programa.
- `prefixo` + `w` alterna o grupo do painel com foco entre dividido e **empilhado** (stacked), como no zellij e no i3: o painel com foco ocupa quase toda a altura e cada um dos outros vira uma linha com o título (`▸ nome`). Mudar o foco pela pilha (`↑↓`/`jk`, ou clicar num título) abre o painel que recebe o foco; um painel novo aberto no grupo entra na pilha. Os painéis dobrados mantêm o tamanho, sem redimensionar o programa (ADR-030).
- Em todo lugar onde o noct move algo sem que se esteja digitando texto, `hjkl` valem como as setas: foco, mover, resize, rolagem, painéis e o inspect.
- Cada painel tem título (definido pelo programa, ou o nome do shell) e, no futuro, indicador de estado: rodando, terminou ok, falhou, atividade não vista. Painéis de script já mostram `●`, `✓` ou `✗ <código>`.
- Rolagem do histórico com `prefixo` + `[` ou com a roda do mouse; a posição aparece no título do painel (`↑ 120/2000`), e rolar de volta até o fim com a roda volta ao vivo. Busca e cópia no histórico ficam para depois.
- Durante a rolagem, o prefixo, os atalhos diretos e um clique em outro painel encerram a rolagem e funcionam como sempre: dá para trocar de painel sem voltar ao fim antes.
- Layouts nomeados podem ser salvos e reabertos (ex.: "claude à esquerda, shell à direita, logs embaixo").

- Fechar um painel devolve o foco ao painel usado antes dele, como no i3: rodar um script (que abre à direita) e fechar a execução volta para o scripts panel, não para o primeiro painel da tab.
- `prefixo` + `n` liga o modo resize: `→`/`↓` aumentam o painel com foco e `←`/`↑` diminuem, movendo a borda com o vizinho (ou, se ele é o último do grupo, com o anterior); `Esc` ou `Enter` terminam.
- `prefixo` + `f` alterna o fullscreen: só o painel com foco aparece, ocupando a tab, e o título mostra `⛶ fullscreen`. É só visual: o layout continua o mesmo, e as setas navegam por ele, mostrando em tela cheia o painel que recebe o foco.
- `prefixo` + `g` abre um terminal num **painel flutuante**, centralizado sobre os outros. Setas chegam nele a partir do painel embaixo do meio dele, e saem dele para o painel mais próximo naquela direção. Mover (`a`) desloca o painel flutuante; resize (`n`) muda o tamanho. Com o mouse: botão esquerdo na borda arrasta, botão direito na borda redimensiona por aquele lado, e clique dentro dá foco e o traz para a frente. Se o último painel fixo fecha, um flutuante vira o painel da tab (ADR-023).

## Sessões

Uma sessão é um conjunto de tabs com nome (ADR-037). O daemon guarda várias; o terminal conectado mostra uma, e as outras continuam rodando por trás.

- `noct` volta à última sessão usada; `noct attach dev` (ou `a`, `att`, `-a`) entra na sessão `dev`, e `noct new [nome]` cria uma. Uma sessão que não existe é recusada, com a dica de `noct new`. `noct sessions` lista e `noct kill <nome>` apaga, com ou sem o noct rodando.
- Nomes não têm espaços, porque são digitados depois de `noct attach`; sem nome, a sessão ganha o primeiro número livre.
- Com mais de uma sessão, a barra de tabs mostra à direita a sessão na tela (`◆ dev`). A busca (`prefixo` + `Space`) também troca de sessão.
- **Fechar** uma sessão para os programas dela e guarda o layout; **reabrir** inicia de novo os shells, cada um no diretório em que estava, e os painéis do noct (scripts, inspect, logs), nos mesmos lugares, com os nomes de tabs e painéis. Execuções, editores e programas que rodavam dentro dos shells não voltam; o layout se fecha sobre eles. A sessão na tela, ao fechar, dá lugar à usada antes dela; a única sessão não fecha.
- `noct quit` (ou `prefixo` + `q`) grava todas as sessões antes de parar: o próximo `noct` reabre a última como estava. As sessões também são gravadas ao trocar, desconectar e a cada 30 s.
- **Apagar** para os programas e esquece a sessão; apagar a da tela leva à usada antes dela, ou a uma nova.

## Atalhos: modo noct

A **tecla prefixo** (padrão `Ctrl+Space`, configurável) liga o **modo noct**. Um popup `commands` no centro da tela lista os comandos disponíveis, e o selo `commands` aparece na barra de tabs. Fora do modo, toda tecla vai direto para o painel, então programas como vim e Claude recebem todas as teclas.

No modo noct:

- **Navegação mantém o modo ligado**, para andar por painéis e tabs com um único prefixo. Segurar `→` percorre os painéis e, na borda da tab, passa para a próxima.
- **Ações encerram o modo** depois de executadas.
- `Esc` ou o prefixo de novo cancelam o modo. Teclas desconhecidas são ignoradas.
- Na home, o popup mostra só os comandos que não precisam de uma tab.

| Tecla | Popup | O que faz | Modo depois | Fase |
|---|---|---|---|---|
| `←↑↓→` / `hjkl` | focus | Painel ao lado; em `←`/`→`, passa da borda para a tab anterior/seguinte | continua | 0 |
| `Tab` / `Shift+Tab` | tab: change | Próxima tab / tab anterior (circular) | continua | 0 |
| `1`–`9` | go to tab | Ir para a tab N | continua | 0 |
| `t` | new tab | Nova tab com um painel de terminal | termina | 0 |
| `p` | new panel | Painel de terminal: divide o painel com foco; na home, abre uma tab | termina | 0 |
| `s` | scripts panel | Scripts panel, do mesmo jeito que `p` | termina | 0 |
| `i` | inspect flows | Inspect: fluxos e o fluxograma, do mesmo jeito que `p` | termina | 2 |
| `v` | logs and calendar | Logs por dia com o calendário, do mesmo jeito que `p` | termina | 2 |
| `a` | move | Modo mover: setas ou `hjkl` movem o painel; `Esc` ou `Enter` terminam | modo mover | 0 |
| `n` | resize | Modo resize: setas aumentam ou diminuem o painel | modo resize | 1 |
| `w` | stacked or split | Empilhar os painéis do grupo com foco, ou dividi-los de novo | termina | 2 |
| `f` | fullscreen | Mostrar só o painel com foco, ou o layout de novo | termina | 1 |
| `g` | floating panel | Terminal num painel flutuante | termina | 1 |
| `r` | rename tab | Renomear a tab: `Enter` salva, `Esc` cancela | prompt | 0 |
| `e` | rename panel | Renomear o painel com foco; vazio volta ao nome do programa | prompt | 2 |
| `x` | close | Fechar o painel; a tab fecha junto com o último | termina | 0 |
| `c` | config | Abrir o config | config | 0 |
| `q` | quit | Pergunta `quit noct?`: `Enter` ou `q` de novo saem, `Esc` cancela | pergunta | 0a |
| `Esc` | cancel | Cancelar o modo noct | termina | 0 |
| `[` | scroll history | Rolar o histórico do terminal com foco: `↑↓`/`jk` linha, `PgUp`/`PgDn` página, `g`/`Home` topo, `Shift+G`/`End` fim, `Esc` volta ao vivo | rolagem | 0b |
| `d` | detach | Desconectar: o terminal volta ao shell e os programas continuam; `noct` reconecta à mesma sessão | — | 0b |
| `o` | sessions | Popup das sessões, da mais recente à mais antiga: aberta (`●`) ou fechada (`○`), tabs e último uso. `Enter` troca de sessão, `n` cria (vazio dá um número), `r` renomeia, `x` fecha (para os programas e guarda o layout), `d` apaga; ver [Sessões](#sessões) | sessões | 5 |
| `?` | program help | Ajuda do programa em primeiro plano no terminal com foco; só com o shell, avisa e mantém o popup | termina | 1 |
| `Space` | search | Busca fuzzy sobre scripts, ações e tabs | busca | 1 |
| `o` | | Execuções | | 2 |
| `y` | MCP | Popup do MCP: no topo as aprovações (o que uma IA pediu e espera a decisão; `y` aprova, `n` nega), depois os clientes de IA conectados agora, e o comando para adicionar o noct a cada cliente conhecido (Claude Code, Codex, Gemini CLI, VS Code, opencode, Cursor), com o caminho deste binário; `Enter` copia e fecha (ver [mcp.md](mcp.md#como-a-ia-se-conecta)) | MCP | 3 |
| `u` | | Catálogo (instalar) | | 4 |

Scripts são abertos pelo scripts panel (`s`) ou pela busca (`Space`), e fluxos pelo inspect (`i`), não por teclas próprias no modo noct.

O scripts panel e o inspect acompanham o disco enquanto aparecem na tab atual: a cada segundo, o noct compara nomes, tamanhos e datas dos arquivos de scripts e fluxos e só os relê quando algo mudou. Um rascunho que a IA escreve pelo MCP, ou uma edição fora do noct, aparece sem fechar e abrir o painel (ADR-036).

## Atalhos diretos

Um segundo modificador, por padrão `Alt` (ADR-024), roda os comandos do modo noct sem o prefixo (ADR-020):

- `Alt` + uma tecla da tabela acima roda aquele comando e deixa o modo noct desligado: `Alt+→` muda o foco (e atravessa a borda da tab), `Alt+T` abre uma tab, `Alt+M` liga o modo mover, `Alt+X` fecha o painel.
- `Alt+Shift` + setas move o painel com foco direto, sem o modo mover.
- Com `Alt`, os comandos tiram do shell alguns atalhos de edição: `Alt+F` (avançar palavra) vira fullscreen, `Alt+D` (apagar palavra) vira detach, `Alt+.` não é afetado. Quem usa muito esses atalhos pode trocar para `Ctrl+Alt` no config.
- Com `Ctrl+Alt`, só valem combinações que o terminal distingue na codificação legada: `Ctrl+Alt+M` chega como `Alt+Enter`, `Ctrl+Alt+[` como `Alt+Esc` e `Ctrl+Alt`+dígito como `Alt`+dígito, e essas teclas continuam indo para o programa. Por isso mover direto usa `Shift` + setas.
- Teclas que não são comandos, com ou sem o modificador, vão para o programa.
- O ambiente gráfico pode capturar algumas combinações antes do terminal (`Ctrl+Alt+T` no GNOME, `Ctrl+Alt+L` no KDE). Nesse caso, troque o modificador no config ou desligue os dois.

## Mouse

Com `mouse` ligado no config (padrão), o noct pede ao terminal os relatórios de mouse:

- Clique na barra de tabs seleciona a tab; clique num painel dá foco a ele. A roda sobre a barra passa para a tab anterior (para cima ou para a esquerda) ou a seguinte (para baixo ou para a direita), parando na primeira e na última.
- Arrastar a linha entre dois painéis (as duas bordas que se encontram) redimensiona os dois; o tamanho mínimo de um painel é respeitado.
- Mensagens na barra de tabs seguem o tema: problemas na cor de atenção, avisos (`copied to the clipboard`, `opened …`) na cor de destaque.
- A roda rola o histórico do terminal sob o mouse, entrando e saindo da rolagem sozinha. Dá para selecionar e copiar texto também durante a rolagem.
- Programas que ligam o mouse (vim, htop, less com `--mouse`) recebem cliques, arrastes e a roda na posição dentro do painel; o noct só dá o foco.
- No scripts panel, clique seleciona um script e a roda move a seleção.
- No popup de comandos do prefixo, clique numa linha roda o comando, como a tecla dela: `tab` troca de tab e mantém o popup, `esc` fecha. As linhas de foco (`←↓↑→ hjkl`) e de número de tab (`1-9`) valem por várias teclas e não fazem nada. Clique fora do popup o fecha. Setas e `hjkl` continuam movendo o foco, não uma seleção no popup.
- Arrastar sobre o texto de um painel (terminal ou painel do noct) seleciona, e soltar copia para a área de transferência (OSC 52; kitty e Ghostty aceitam). A seleção nunca passa para outro painel: arrastar para fora a prende na borda do painel onde começou. Um clique ou uma tecla limpa a seleção. Só bordas, tabs, divisórias e itens clicáveis, como a lista do scripts panel, recebem cliques.
- A seleção acompanha a rolagem, como num terminal: com o botão segurado, a roda rola o histórico e a seleção continua de onde começou, e arrastar para cima ou para baixo do painel rola uma linha por vez. O texto copiado inclui as linhas que saíram da tela.
- No painel de uma execução de script, a linha de estado (`✓ done  r run again  enter close`) não entra na seleção: só a saída do script é copiada.
- `Shift` + arrastar ainda usa a seleção do próprio terminal de fora. Quem prefere só ela desliga o mouse no config.

## Config

`prefixo` + `c` abre o painel **config**, gravado em `~/.config/noct/config.toml` (TOML legível, editável à mão). Cada mudança vale na hora e é salva.

| Seção | Item | Valores |
|---|---|---|
| **Prefix** | `ctrl`, `alt`, `shift`, `key` | Modificadores ligados ou desligados + uma tecla (caractere, `space` ou `F1`–`F12`). Precisa ter `ctrl` ou `alt`, senão seria digitado como texto. |
| **Shortcut** | `ctrl`, `alt` | Modificadores dos atalhos diretos; padrão os dois. Com os dois desligados, os atalhos diretos ficam desligados. |
| **Appearance** | `theme` | `noct`, `aurora`, `ember`, `rose`, `blood`, `frost`, `mono`. |
| | `background` | `terminal` mantém o fundo do terminal; `theme` pinta o fundo e o texto padrão com as cores do tema, inclusive dentro dos painéis. |
| **Mouse** | `clicks and drag` (`mouse` no arquivo) | Liga os cliques, o arraste das divisórias e a roda; padrão ligado. |
| **Programs** | `editor` | O editor que abre cada parte do script (`Enter` no formulário do script) e scripts quebrados: `vim` (padrão), `nano`, os outros editores conhecidos instalados (`nvim`, `hx`, `micro`, `emacs`, `kak`, `vi`, `joe`, `mcedit`) e `custom`. |
| | `file manager` | Abre a pasta dos scripts (`Ctrl+O`): `terminal` (padrão, um painel de terminal na pasta), os gerenciadores instalados (yazi, ranger, lf, nnn, vifm, mc num painel; thunar, nautilus, dolphin, nemo, pcmanfm, caja numa janela) e `custom`. |
| | `command` | Aparece embaixo de `editor` ou `file manager` quando `custom` está escolhido: o comando digitado ali mesmo (pode ter argumentos, como `code -w`), com o aviso `command not found` se o programa não existir. Salva ao sair da linha ou com `Enter`. |
| **Scripts** | `location` | A pasta dos scripts pessoais, já preenchida com a que está em uso (padrão `~/.local/share/noct/scripts`); aceita `~`. Com a linha selecionada, `Ctrl+C` (ou `Ctrl+Shift+C`) copia o caminho para a área de transferência, e a dica embaixo muda de `ctrl+c copy` para `copied to clipboard`. Vale para a TUI e para `noct -s`. |
| | `sort` | Ordem da lista de scripts: `alphabetical` (padrão), `last edited` (o `script.yaml` alterado por último primeiro) ou `last run` (o rodado por último primeiro; os nunca rodados vêm depois, em ordem alfabética). A seleção fica no mesmo script quando a ordem muda. |
| | `hints in new scripts` | Liga (padrão) ou desliga o comentário de 4 linhas no topo de cada script novo, explicando `description`, `program`, `run` e `params`. |
| **Flows** | `step output kept` (`[flows] output_kb` no arquivo) | Quanto da saída de cada passo o último resultado de um fluxo guarda: `all` (padrão, até 1 MB, o que os templates leem), 256, 64, 16 ou 4 KB. Testes de passos (`t` no inspect) guardam sempre 4 KB (ADR-031). |

O noct recebe as teclas na codificação legada, a mesma que o shell receberia direto do terminal. Nela, `Ctrl+Shift+A` chega igual a `Ctrl+A` e `Shift+Space` igual a `Space`, então `shift` só vale no prefixo como `Alt+Shift+letra` ou com F1–F12; o config recusa as demais combinações (ADR-012).

### Atalhos do terminal de fora

Atalhos do próprio terminal têm prioridade: se o kitty usa `Ctrl+L` para limpar a tela, a tecla nunca chega ao noct. Para esses casos, o noct define a variável de usuário `in_noct` enquanto roda, e o kitty pode deixar a tecla passar só quando o noct está em foco:

```conf
# kitty.conf: dentro do noct, Ctrl+L vai para o painel
map --when-focus-on var:in_noct ctrl+l
```

## Estética

- Tema definido por tokens (fundo, superfície, texto, acento, sucesso, erro, aviso) em `config.toml`; um tema escuro padrão com identidade própria e um claro.
- Bordas retas e discretas; o painel com foco destacado pela cor de acento.
- Home com o título `noct` em blocos (cor de destaque) e o corvo de `internal/app/raven2.ascii` na cor das dicas, centralizados; o `raven.ascii` original, maior, fica na raiz do repositório.
- Suporte a truecolor com fallback para 256 cores.
- Animações só onde informam algo (spinner de execução, transição de view), nunca bloqueando input.

## Imagens

**Programas dentro dos painéis** (fastfetch, e qualquer programa que use o protocolo gráfico do kitty) mostram imagens quando o terminal de fora desenha os *Unicode placeholders* do kitty: kitty 0.28+ e Ghostty (ADR-014). O noct:

1. separa os comandos de imagem da saída do programa, na ordem em que chegam;
2. envia a imagem ao terminal de fora como posicionamento virtual (`U=1`), com um id único no noct;
3. escreve na grade do painel as células marcadoras (`U+10EEEE` + diacríticos), que o terminal troca pela imagem;
4. move o cursor como o kitty faria e responde às consultas do programa.

Como a imagem vira texto na grade, ela rola, é recortada pelas bordas do painel e some com `clear` junto com o resto. Em outros terminais os comandos de imagem são descartados, como antes. Não há suporte a animações, nem a apagar imagens por posição.

**As próprias views do noct** (notas com imagens, previews) continuam na fase 5, usando o mesmo mecanismo no kitty e Sixel/iTerm2/meios-blocos nos demais terminais.

## Qualidade de vida (lista viva)

- Notificação (sino do terminal e/ou notificação do sistema) quando uma execução em segundo plano termina.
- Re-executar o último fluxo com os mesmos parâmetros em uma tecla (scripts já têm: `r`).
- Clicar/abrir URLs e caminhos de arquivo na saída.
- Timestamps opcionais por linha nos logs.
- Filtrar a saída de um painel por regex sem parar o processo.
