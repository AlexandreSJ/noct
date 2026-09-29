# Decisões de arquitetura

Formato: contexto → decisão → consequências. Decisões novas são acrescentadas no fim; decisões superadas são marcadas, não apagadas.

## ADR-001 — Go como linguagem

**Contexto.** O noct deve ser leve, rodar em qualquer lugar e ter uma TUI bonita. Os candidatos eram Go e Rust.

**Decisão.** Go.

**Consequências.**
- A favor: desenvolvimento mais rápido, binário estático simples, ecossistema Charm (Bubble Tea, Lip Gloss, Bubbles, Glamour) muito forte em estética, SDK MCP oficial.
- Contra: a emulação de terminal (necessária para rodar TUIs dentro de painéis) é menos madura em Go do que em Rust (`alacritty_terminal`, `wezterm-term`). Mitigação: spike 0a no [roadmap.md](roadmap.md) antes de qualquer outra coisa.

## ADR-002 — Multiplexador próprio, não plugin do Zellij

**Contexto.** Usar o Zellij como base aceleraria a fase 0, mas prenderia o noct à UI, ao layout e às versões dele.

**Decisão.** Construir o multiplexador do zero.

**Consequências.** Controle total de estética e UX, e um binário único. A fase 0 fica mais longa e concentra o risco técnico.

## ADR-003 — Daemon + clientes via socket Unix

**Contexto.** Programas precisam continuar rodando com a TUI fechada, e a IA precisa enxergar os mesmos painéis.

**Decisão.** Um daemon é dono dos processos e do estado da tela; TUI, CLI e MCP são clientes do mesmo protocolo `ipc`.

**Consequências.** Tudo que a TUI faz fica disponível para CLI e IA. Custo: definir e versionar um protocolo, e manter a emulação VT no daemon.

## ADR-004 — Low-code como texto (YAML), editor visual depois

> **Superada em parte pela ADR-007:** o editor visual entrou na fase de fluxos. O YAML continua sendo o formato salvo.

**Contexto.** Um editor visual de fluxos é um projeto à parte e não é necessário para a flexibilidade pedida.

**Decisão.** Comandos e fluxos em YAML com parâmetros, passos, condições e triggers. Um editor visual futuro deve gerar o mesmo formato.

**Consequências.** Comandos e fluxos são versionáveis e fáceis de gerar pela IA. A configuração do noct fica em TOML, que é mais simples para chaves e valores planos.

## ADR-005 — Catálogo delega aos gerenciadores de pacote

**Contexto.** O usuário quer instalar ferramentas com apt, pacman, paru, go, cargo etc.

**Decisão.** O noct mantém manifestos e adaptadores que montam comandos; a instalação roda num painel visível.

**Consequências.** Não há lógica de pacotes para manter; `sudo` e prompts funcionam naturalmente. A busca além do catálogo é só texto na v1.

## ADR-006 — Linux e macOS na v1

**Contexto.** No Windows, PTYs usam ConPTY, com comportamento e bibliotecas diferentes.

**Decisão.** A v1 mira Linux e macOS; o pacote `pty` fica atrás de uma interface para permitir ConPTY depois.

**Consequências.** Quem usa Windows roda via WSL até existir suporte nativo.

## ADR-007 — Editor visual de fluxos na fase 2

**Contexto.** O usuário quer montar fluxos com `se/senão`, repetições e esperas sem editar YAML à mão.

**Decisão.** A fase 2 inclui um editor visual na TUI, no formato de fluxograma vertical estruturado (blocos em sequência, dentro de `se/senão` ou de `repetir`). O YAML continua sendo a fonte da verdade; o editor e a IA leem e escrevem o mesmo arquivo. Não há canvas livre de arrastar nós.

**Consequências.** A fase 2 fica maior, mas os fluxos continuam versionáveis e gerados pela IA. Todo recurso do formato precisa de uma representação no editor, o que ajuda a manter o formato pequeno.

## ADR-008 — Workspace desenhado sobre ultraviolet + charmbracelet/x/vt

**Contexto.** O workspace precisa de emulação VT para cada painel e de renderização rápida com vários painéis de alta vazão. No Bubble Tea, cada evento passa pelo `Update` e pelo `View`, que gera a tela inteira como texto; saída pesada viraria uma inundação de eventos.

**Decisão.** Emulação VT com `github.com/charmbracelet/x/vt` e renderização direta com `github.com/charmbracelet/ultraviolet`, que já compara quadros e envia só as células alteradas. O emulador desenha direto numa área da tela (`Draw(screen, área)`), sem conversão para texto. Views ricas das fases seguintes podem usar componentes Charm por cima.

**Consequências.** Controle total do loop de eventos e do ritmo de quadros. As duas bibliotecas ainda não têm versão estável (commits fixados no `go.mod`), então atualizações podem quebrar a API; o uso fica concentrado nos pacotes `pane` e `app`. Os problemas encontrados nas bibliotecas e como o noct os contorna estão na ADR-011.

## ADR-009 — "Comando" e "fluxo" no lugar de "receita"

**Contexto.** Havia três termos para duas coisas.

**Decisão.** **Comando**: um programa com notas, versão, ajuda e invocações salvas. **Fluxo**: a composição de comandos com lógica. O termo "receita" foi aposentado.

**Consequências.** Pastas `commands/` e `flows/`; subcomandos `noct run <comando|fluxo>`.

*Atualizada pela ADR-018:* "comando" virou **script**, e a pasta, `scripts/`.

## ADR-010 — Instalação de pacotes depois da IA

**Contexto.** A instalação de programas saiu das prioridades imediatas.

**Decisão.** O catálogo com instalação vira a fase 4, depois do MCP. A fase 1 mantém só a detecção de versão ou `(não encontrado)`.

**Consequências.** As fases 1–3 não dependem de adaptadores de gerenciadores de pacote.

## ADR-011 — Resultado da fase 0a: Go serve (aceita)

**Contexto.** A fase 0a precisava responder se Go serve para rodar terminais reais em painéis (critérios em [roadmap.md](roadmap.md)).

**Resultado** (medido em 2026-09-26, Linux, zsh 5.9 com a configuração real do usuário):

| Critério | Resultado |
|---|---|
| zsh em 2 painéis lado a lado, em tabs | ✅ |
| Prompt e título definidos pelo shell | ✅ |
| Sugestão do `zsh-autosuggestions` e menu de completar com descrições | ✅ |
| Redimensionar a janela refaz o layout | ✅ |
| `Ctrl+C` e digitação durante saída infinita (`yes`) | ✅ prompt de volta em ~40 ms |
| Vazão de saída (`cat` de arquivo grande) | ⚠️ ~2 MB/s: lento, mas não trava a entrada |

**Recomendação, a confirmar pelo mantenedor.** Continuar em Go. Todos os critérios funcionais passaram; a vazão é limitada pelas bibliotecas, não pela linguagem, e as causas estão identificadas.

**Problemas encontrados nas bibliotecas e como o noct os contorna:**

- `ultraviolet.Terminal` reaproveita um único buffer de leitura entre goroutines, e teclas que chegam em sequência rápida se corrompem (`echo` virava `ehho`). O noct lê a entrada com `ultraviolet.TerminalReader`, que aloca por leitura (hoje em `internal/app/client.go`, no daemon).
- `vt.Emulator.Draw` desenha só as linhas marcadas como alteradas desde o último redimensionamento, e `CellAt` devolve ponteiros lidos fora da trava. O `pane` usa o emulador sem a versão "Safe", com trava própria, e desenha todas as células sob essa trava.
- `vt.Emulator.Close` tem condição de corrida com um `Read` pendente. O `pane` nunca chama `Close`; ele acorda o leitor com uma escrita vazia.
- `vt.Emulator.SendKey` não codifica teclas de navegação com modificadores (`Ctrl+←`, `Shift+↑`) e descarta texto com `Shift`. O `pane` codifica essas teclas (`internal/pane/keys.go`).
- Fechar o PTY com uma leitura pendente não encerra o programa. `pty.Close` envia SIGHUP ao grupo de processos, como o tmux.
- `TerminalScreen.Flush` do `ultraviolet` põe o movimento do cursor no buffer do renderer depois de esse buffer já ter sido escrito, então o cursor só chega ao terminal no quadro seguinte: ele fica sempre um passo atrás (a seta anda, o cursor não). O noct chama `Render` e `Flush` de novo depois de cada `Display` (`internal/app/draw.go`), o que não desenha nada novo e só envia o movimento pendente. Um teste e2e cobre.

**Vazão.** O perfil mostra dois pontos quentes: `ultraviolet.Buffer.DeleteLineArea` copia a tela inteira célula por célula a cada linha rolada, e `vt.Scrollback.Push` desloca todo o histórico a cada linha quando ele está cheio. Corrigir os dois numa cópia local levou o emulador de 1,9 para 5,9 MB/s. O restante vem de alocar uma string por caractere impresso e exige mudanças mais profundas no emulador.

**Consequências.** Os problemas acima devem ser reportados aos projetos da Charm, com as correções de vazão como pull requests. Se a vazão continuar insuficiente no uso real (logs muito verbosos desaceleram o programa que os produz, por contrapressão do PTY), as opções são: manter um fork com as correções via `replace` no `go.mod`, ou escrever um emulador próprio sobre `charmbracelet/x/ansi`.

**Decisão do mantenedor (fase 1):** Go confirmado, e as correções da Charm ficam no noct. `ultraviolet` e `x/vt` estão copiados em `third_party/` como pacotes do próprio módulo (imports reescritos, sem `go.mod` próprio), com as duas correções marcadas com `noct:`: `Buffer.DeleteLineArea` move linhas inteiras em vez de copiar célula por célula, e `Scrollback.Push` descarta a linha mais antiga sem deslocar o histórico inteiro. A vazão do emulador foi de 2,5 para 7,6 MB/s. Não se usa `replace`, porque `go install …@latest` recusa módulos com `replace` para caminhos locais. [third_party/README.md](../third_party/README.md) diz como atualizar as cópias. As correções continuam candidatas a pull request para a Charm.

**Ghosts na tela (2026-09-28).** Às vezes, células antigas ficavam na tela (restos do painel de config sobre a home, restos no Claude CLI) até algo ser escrito em cima. O renderer do `ultraviolet` só envia o que mudou em relação ao modelo que ele guarda do terminal; quando o modelo erra, o erro fica. Um teste que reproduz quadros aleatórios num emulador `vt` e compara com o esperado (`third_party/ultraviolet/noct_render_test.go`) achou quatro erros, corrigidos na cópia com `noct:`:
- `Line.Set`: um caractere largo sobre a primeira metade de outro deixava a segunda metade como célula de largura 0, que `TerminalScreen.Render` pula; aquela coluna nunca mais era enviada.
- `Buffer.DeleteCellArea` (o `DCH` dos painéis) copiava célula por célula com `SetCell`, que apagava um caractere largo no meio da cópia: apagar antes de um emoji ou CJK o fazia sumir.
- `transformLine` ia para `n+1` depois de um caractere largo em `n`, caindo na segunda metade dele; o `EL` seguinte apagava o caractere. No ramo de inserção, o laço que tentava pular essa metade nunca avançava e travava o daemon.
- `clearBottom` limpava a partir da linha acima da primeira linha em branco, e essa linha não era redesenhada se não tivesse mudado.

O caso da home (sem caracteres largos) não se reproduz no emulador: a sequência enviada está correta. Para esse, `NOCT_TRACE=arquivo noct` grava os bytes que o cliente escreve no terminal, para comparar o que o kitty mostrou com o que os bytes pedem.

**Título com `✳` (2026-09-28).** Com o Claude Code num painel, a borda do painel ficava desalinhada, e o nome dele aparecia dentro do painel. O título `✳ Claude Code` chega por OSC, e em UTF-8 o `✳` é `E2 9C B3`. A tabela do parser do `x/ansi` trata `0x9C` como o terminador C1 (ST) mesmo dentro de uma string, então o título virava `\xe2` e o resto era impresso na tela. Esse byte solto ainda seguia até o terminal real, que o juntava ao `ESC` seguinte e perdia a posição. Isso afeta todo o bloco U+2700–U+273F (`✅`, `✔`, `✨`, `✳`) e qualquer caractere com `0x9C` no UTF-8, também em links OSC 8 e em DCS. Como no xterm e no kitty em UTF-8, a cópia do `vt` passa a tratar `0x9C` em strings OSC e DCS como dado (`init` com `noct:` em `emulator.go`); `ESC \` e `BEL` continuam terminando. O parser não aceita tabela própria, então a mudança vale para a tabela compartilhada, que no noct só o `vt` usa.

## ADR-012 — Modo noct persistente e protocolo de teclado do kitty

**Contexto.** Apertar o prefixo antes de cada movimento torna a navegação entre painéis e tabs cansativa. Além disso, sem o protocolo de teclado do kitty, o terminal não distingue combinações como `Ctrl+Shift+Space` de `Ctrl+Space`, o que limita a tecla prefixo configurável.

**Decisão.**
- O prefixo liga um modo. Navegação (setas, `hjkl`, `Tab`, `Shift+Tab`, `1`–`9`) mantém o modo; ações o encerram; `Esc` ou o prefixo de novo saem; teclas desconhecidas são ignoradas.
- ~~O noct pede ao terminal de fora o nível "disambiguate" do protocolo de teclado do kitty.~~ **Revertido em 2026-09-26:** com o protocolo, o kitty informa NumLock e CapsLock como modificadores, e o `pane` descartava essas teclas (`Ctrl+D`, `Ctrl+C` e `Esc` pararam de chegar aos programas). O noct voltou a receber a codificação legada, a mesma que o shell receberia direto do terminal. O `pane` também passou a ignorar modificadores de trava, e a tecla prefixo recusa combinações que a codificação legada não distingue (`Ctrl+Shift+letra`, `Shift+Space`).
- O noct define a variável de usuário `in_noct` (OSC 1337 `SetUserVar`) enquanto roda, para que o kitty libere atalhos próprios só dentro do noct (`map --when-focus-on var:in_noct ...`).

**Consequências.** Enviar o prefixo literal a um programa do painel deixou de ser possível; se virar necessidade, entra como ação do modo. `Shift` só vale no prefixo como `Alt+Shift+letra` ou com F1–F12.

## ADR-013 — Configuração em TOML com BurntSushi/toml

**Contexto.** As configurações precisam persistir num arquivo legível e editável à mão (princípio "arquivo como fonte da verdade"). A biblioteca padrão do Go não lê TOML.

**Decisão.** `~/.config/noct/config.toml` (via `os.UserConfigDir`), lido e escrito com `github.com/BurntSushi/toml`: madura, sem dependências próprias, API mínima. A gravação é atômica (arquivo temporário + rename). Configurações ausentes mantêm o padrão; uma tecla prefixo inválida volta para `ctrl+space` com aviso na barra.

**Consequências.** Uma dependência a mais, restrita ao pacote `config`.

## ADR-014 — Imagens nos painéis via Unicode placeholders do kitty (antecipado)

**Contexto.** A [tui.md](tui.md) deixava fora da v1 as imagens de programas dentro dos painéis, porque reposicionar e recortar imagens em splits é frágil (tmux e Zellij têm suporte parcial). O mantenedor pediu o recurso agora, a partir do fastfetch, que envia `a=T,f=100,t=f,c=44` com o caminho de um PNG e depois pede a posição do cursor (`ESC[6n`) para alinhar o texto.

**Decisão.** Usar os *Unicode placeholders*, o mecanismo que o kitty criou para multiplexadores:
- `internal/kittygfx` separa os comandos (`ESC _G … ESC \`) da saída, traduz os ids de imagem de cada programa para ids únicos no terminal de fora, junta uploads em partes antes de enviá-los e calcula colunas e linhas como o kitty (pela proporção do PNG e pelo tamanho da célula em pixels do terminal de fora).
- O `pane` escreve as células marcadoras na grade e move o cursor como o kitty, tudo na ordem da saída, para que o `ESC[6n` responda a posição certa.
- O `app` envia os comandos ao terminal de fora antes do quadro que mostra os marcadores, e só liga o recurso quando o terminal de fora é o kitty ou o Ghostty.
- A tabela de 297 diacríticos é a lista fixa da especificação do kitty (marcas de classe 230 do Unicode 6.0.0); só os pontos de código foram copiados.

**Consequências.** A imagem acompanha o texto sem código especial para rolagem, recorte e limpeza. Não há suporte a animações nem a apagar por posição. Imagens de painéis fechados ficam na memória do kitty até a janela fechar ou o limite de armazenamento do kitty descartá-las. Nos painéis, a variável `TERM` continua `xterm-256color`, então programas que detectam o kitty só pelo `TERM` não tentam usar imagens.

## ADR-015 — Home, popup de comandos, process panel e fim do zoom

*O fim do zoom foi revisto pela ADR-023, que traz o fullscreen de volta.*

> **Ajustada pela ADR-016:** o process panel virou scripts panel (`s`), `t` cria tabs vazias e a home deixou de ser configurável.

**Contexto.** O mantenedor pediu uma home vazia com o corvo, os atalhos fora da barra de tabs (muitas tabs cobriam as dicas), um painel onde começam os recursos de automação, mover e renomear painéis e tabs, e a remoção do zoom.

**Decisão.**
- O noct abre na **home** e volta para ela quando a última tab fecha; só `prefixo` + `q` sai. A home mostra o corvo em DIM e, conforme o config, os atalhos (ao apertar uma tecla, sempre ou nunca).
- As dicas do modo noct saem da barra de tabs e viram um **popup** no canto inferior direito. Popup, config e prompt de renomear usam o mesmo componente de painel (`internal/app/panel.go`), e uma única tabela de atalhos alimenta o popup e a linha de atalhos da home.
- Um painel do layout mostra uma **view**: um terminal (`pane.Pane`) ou um painel do noct. O **process panel** é a primeira view do noct e a porta de entrada dos comandos (fase 1) e fluxos (fase 2); hoje mostra só um estado vazio.
- O **zoom** foi removido: o terminal de fora já maximiza a janela.
- **Mover** (`m`, hoje `a` pelo ADR-030) troca o painel de lugar com o vizinho e atravessa para a tab adjacente pela borda esquerda ou direita, criando uma tab nova quando não há.

**Consequências.** Sair do último shell com `Ctrl+D` não fecha mais o noct: leva à home. A interface `view` deixa as fases 1 e 2 entrarem como views, sem mexer no layout nem nas tabs, e permite testar a navegação sem processos reais.

## ADR-016 — Tabs como contêineres, scripts panel e home fixa

*Atualizada pela ADR-019:* tabs não ficam mais vazias; fecham com o último painel.

**Contexto.** O mantenedor reorganizou as teclas: `t` abre uma tab, `p` abre um painel, `s` abre um painel de scripts. Também pediu `noct` acima do corvo, um corvo menor e a remoção das opções da home no config.

**Decisão.**
- Uma tab é um contêiner que pode ficar **vazia**; tab vazia mostra a home. `t` cria uma tab já com um terminal; `p` (terminal) e `s` (scripts) preenchem a tab vazia ou dividem o painel com foco; na home sem tabs, abrem uma tab primeiro.
- Fechar o último painel deixa a tab vazia em vez de fechá-la; `x` numa tab vazia fecha a tab.
- O process panel virou **scripts panel**, a mesma porta de entrada das fases 1 e 2.
- A home mostra o título `noct` em blocos (2 linhas, cor de destaque), duas linhas em branco, o corvo na cor de logo do tema (a mesma das dicas, exceto no `blood`, onde é um vermelho escuro, `#c25a5a`) e, por 3 s depois de uma tecla, só a tecla prefixo: é ela que abre o popup com os comandos. Não há opções no config para isso.
- O popup de comandos fica no centro da tela.
- O corvo da home é `internal/app/raven2.ascii` (13×27): o `raven.ascii` original reduzido pela metade com blocos de quadrante, que preservam a silhueta. O original fica na raiz do repositório.

- Sair do noct pede confirmação (`quit noct?`), exceto com `Ctrl+D` duas vezes na home, onde a segunda tecla já é a confirmação. `Ctrl+D` fecha o scripts panel, como fecha um shell. Menus abertos na home escondem o título e o corvo.

**Consequências.** `Ctrl+D` no último shell de uma tab deixa a tab vazia; um segundo `Ctrl+D` já na home (tab vazia) sai do noct. O config perdeu a seção `[home]`; arquivos antigos com ela continuam lendo, e a seção é ignorada.

## ADR-017 — Daemon que desenha para um cliente fino

**Contexto.** A fase 0b precisava manter os programas rodando com a TUI fechada. A [arquitetura.md](arquitetura.md) previa um cliente que recebe diffs da grade de células por JSON-RPC e desenha sozinho.

**Decisão.**
- O workspace inteiro (`app`) roda no daemon, e o daemon **desenha para o terminal do cliente**, como o tmux. O cliente só põe o terminal em modo raw, repassa bytes de teclado e tamanho, e escreve na tela os bytes que recebe.
- O protocolo é mínimo: mensagens com 1 byte de tipo e 4 de tamanho; `Hello`/`Resize`/`Exit` em JSON e `Input`/`Output` em bytes crus. O `Hello` leva o ambiente e o perfil de cores do cliente, porque o daemon não vê aquele terminal.
- Um cliente por vez; um novo attach desconecta o anterior.
- `noct` inicia o daemon quando ele não está rodando (`noct daemon` numa sessão própria, via `setsid`); `noct quit` o encerra.
- Cada terminal grava sua saída crua em `~/.local/state/noct/logs/<data>/`, com rotação em 10 MiB.
- `prefixo` + `[` rola o histórico do terminal com foco; programas de tela cheia não têm histórico.

**Consequências.** Quase nada do workspace mudou: o `App` só trocou o terminal real por um cliente conectado. O cliente fica pequeno, e a mesma renderização por diff do `ultraviolet` vale pelo socket. O custo é o daemon renderizar para as capacidades de um terminal que ele não vê diretamente (cores e imagens vêm do `Hello`). Novos terminais abrem no diretório em que o daemon foi iniciado, não no do cliente atual; enviar o diretório do cliente no `Hello` é uma melhoria simples para depois (feita na fase 1, ADR-018). Busca e cópia no histórico ficam para depois.

## ADR-018 — Scripts: nome, arquivos, execução em painel e YAML

**Contexto.** A fase 1 pedia uma biblioteca de "comandos". Na interface, o mantenedor já chamava o painel de *scripts panel* e pedia `Ctrl+N` para um novo script. Faltava decidir formato, onde ficam, como criar e editar, como rodar e como mostrar o resultado.

**Decisão.**
- O nome é **script** em tudo: código (`internal/script`), pastas (`scripts/`), UI e docs. "Fluxo" continua sendo a composição de scripts (ADR-009).
- Cada script é uma pasta com `script.yaml` e `notes.md`, em `~/.local/share/noct/scripts/` e no `.noct/scripts/` mais próximo do diretório de trabalho; o do projeto vence. Formato em [scripts.md](scripts.md).
- YAML com `go.yaml.in/yaml/v3`, a biblioteca mantida pela organização YAML (continuação do `gopkg.in/yaml.v3`). A biblioteca padrão não lê YAML, e o formato já estava escolhido (ADR-004). Os templates de `run` usam `text/template`, da biblioteca padrão.
- Criar e editar abrem o editor do usuário num painel (qual editor: ADR-021), em vez de um editor próprio: segue "delegar, não reimplementar", e o usuário ganha realce, desfazer e o editor que já conhece. A biblioteca é relida quando o editor fecha.
- Rodar abre um **terminal ao lado** com `$SHELL -c`, e o painel **fica aberto depois do fim**, com o código de saída, até o usuário fechar ou rodar de novo (`r`). Um teste que falhou é lido antes de sumir.
- Parâmetros são pedidos num formulário centralizado; os lembrados ficam em `~/.local/state/noct/params.json`, sem segredos. Valores `from: env:` vêm do ambiente dos painéis. Só `env:` é aceito; o keyring previsto antes fica para quando houver necessidade.
- A versão do programa é detectada em segundo plano, com timeout, e nunca com um argumento `version` solto, que muitos programas tratam como arquivo.
- `prefixo` + `?` usa o grupo em primeiro plano do PTY para abrir a ajuda (notas, `--help`, `man`) no pager; `prefixo` + `Space` abre uma busca fuzzy sobre scripts, ações e tabs.
- `noct ls`, `noct check` e `noct run` rodam no próprio terminal, sem o daemon: são leitura de arquivos e um processo filho, e funcionam em scripts de CI.
- O `Hello` do cliente passa a levar o diretório de trabalho: novos terminais abrem nele, e ele decide os scripts do projeto.
- O prompt de renomear e a confirmação de sair viraram componentes genéricos (prompt de texto e confirmação), reutilizados para criar e apagar scripts.

**Consequências.** Os painéis do noct passam a iniciar programas por uma função única (`App.launch`), o que também deixa os testes da TUI rodarem sem processos. O valor de um segredo entra na linha de comando renderizada e aparece no `ps`; [scripts.md](scripts.md) recomenda usar a variável de ambiente direto em `run`. Campos previstos antes e não implementados (`tags`, `pane`, `.env_vars`, keyring) saíram do formato até alguém precisar deles.

## ADR-019 — Tab fecha com o último painel

**Contexto.** Pela ADR-016, fechar o último painel deixava a tab vazia, mostrando a home, e era preciso `x` para fechar a tab. O mantenedor pediu que a tab suma quando não tiver mais painéis.

**Decisão.** Uma tab fecha junto com o último painel, seja qual for o motivo: `x`, o programa terminar (`Ctrl+D` no shell), `Ctrl+D` no scripts panel, fechar uma execução ou mover o painel para outra tab. A seleção fica na mesma tab, ou vai para a que ocupa o lugar da fechada. Sem tabs, aparece a home, onde `Ctrl+D` duas vezes sai do noct.

**Consequências.** Não existe mais tab vazia visível: `x` sempre fecha um painel, e `p`/`s` só abrem uma tab nova a partir da home. `Ctrl+D` no último shell da última tab leva à home, onde `Ctrl+D` duas vezes sai.

## ADR-020 — Atalhos diretos, mouse e renomear scripts

**Contexto.** O mantenedor pediu um segundo modificador, por padrão `Ctrl+Alt`, para controlar painéis sem o prefixo; cliques para trocar de tab, dar foco, selecionar scripts e redimensionar painéis arrastando a divisória; e `r` no scripts panel para renomear, no lugar de "rodar de novo".

**Decisão.**
- **Atalhos diretos:** o modificador configurável (`[shortcut]`, `ctrl` e `alt`) com uma tecla do modo noct roda o comando sem o prefixo, e com `Shift` + setas move o painel. Como o noct lê a codificação legada (ADR-012), combinações que o terminal não distingue não são capturadas: `Ctrl+Alt+M` chega igual a `Alt+Enter` (a nova linha do Claude Code, por exemplo) e continua indo para o programa. Por isso mover usa `Shift` + setas e não `m`.
- **Mouse:** com `mouse = true` (padrão), o cliente recebe o modo de arraste (1002) com codificação SGR. O noct usa cliques na barra de tabs e nos painéis, arraste das divisórias e a roda para o histórico; programas que ligam o mouse recebem os eventos na posição dentro do painel. O emulador informa os modos de mouse de cada programa pelos callbacks de modo.
- **Divisórias com proporção:** cada split do layout guarda a fração do primeiro lado (antes era sempre metade), que o arraste ajusta respeitando o tamanho mínimo do painel e que se mantém ao redimensionar a janela.
- **Renomear script:** `r` no scripts panel renomeia a pasta, e os valores lembrados vão junto. "Rodar de novo" fica só no painel da execução (`r` depois do fim); a memória de valores da sessão, que servia a ele, saiu.

**Consequências.** Capturar o mouse tira do terminal de fora a seleção de texto sem `Shift`; quem prefere a nativa desliga o mouse no config. Algumas combinações de `Ctrl+Alt` são capturadas pelo ambiente gráfico antes de chegar ao terminal; o modificador pode ser trocado ou desligado. Soltar o botão fora da janela durante um arraste pode não chegar ao noct; o próximo clique termina o arraste.

## ADR-021 — Editor escolhido no config

**Contexto.** Scripts eram editados no `$VISUAL`/`$EDITOR`, com `vi` como último recurso. O mantenedor pediu que o editor fosse escolhido no config, com `vim` como padrão, `nano` como alternativa, os outros editores instalados detectados e um comando livre.

**Decisão.** O config ganhou `editor` (padrão `vim`). No painel, `←→` passa por `vim`, `nano`, os editores conhecidos encontrados no `PATH` quando o config abre (`nvim`, `hx`, `micro`, `emacs`, `kak`, `vi`, `joe`, `mcedit`) e `custom…`, que pede o comando num prompt; `Esc` volta ao config sem mudar nada. O comando roda pelo `sh`, então aceita argumentos (`code -w`). Antes de abrir, o noct confere se o programa existe e, se não, avisa em vez de abrir um painel que fecharia na hora.

**Consequências.** `$VISUAL` e `$EDITOR` deixam de ser lidos: o editor é um só, o do config, também para quem os tem definidos. Um editor gráfico só funciona se esperar o arquivo fechar (`code -w`), porque a biblioteca é relida quando o comando termina.

## ADR-022 — Tiling como o i3, seleção de texto e scripts pelo formulário

**Contexto.** O mantenedor pediu: dispor e mover painéis como o i3 dele (com autotiling), selecionar texto com o mouse, editar scripts num formulário do noct (nome, script, descrição) em vez de só no editor, abrir a pasta dos scripts num gerenciador de arquivos, escolher o local dos scripts e rodar scripts com `noct -s`.

**Decisão.**
- **Layout n-ário:** a árvore de uma tab tem containers com dois ou mais filhos, cada um com uma fração do container, como os containers do i3. Um container nunca tem um filho da mesma orientação: esse filho é dissolvido nele.
- **Painel novo (autotiling):** a direção segue a proporção do painel com foco (mais largo que alto: lado a lado). Se o container dele já corre nessa direção, o painel novo entra como irmão, tirando uma parte igual de cada um; senão o painel com foco vira um container dos dois. Se algum painel ficaria menor que o mínimo, o painel não abre.
- **Mover** segue o `move` do i3: dentro de um container na direção do movimento, troca com o vizinho, ou entra nele se o vizinho for um container; na borda, sai e fica ao lado do grupo, no container de fora que corre nessa direção; sem nenhum, a tab inteira passa a correr nessa direção com o painel na ponta. Na borda da tab, `←`/`→` continuam levando o painel para a tab vizinha. Outras lógicas de tiling ficam para quando alguém precisar.
- **Seleção de texto:** com o mouse capturado, o terminal de fora não seleciona. Arrastar sobre o texto de um painel seleciona (vídeo reverso) e soltar copia para a área de transferência pelo OSC 52; um clique ou tecla limpa. Bordas, tabs, divisórias e a lista do scripts panel continuam clicáveis, e programas que pedem o mouse o recebem. `Shift` ainda dá a seleção nativa.
- **Formulário do script:** `e` abre nome, script (a linha `run`) e descrição; salvar reescreve só essas chaves do `script.yaml`, preservando comentários e parâmetros, e renomeia a pasta se o nome mudou (os valores lembrados vão junto). `n` abre o mesmo formulário vazio. `Ctrl+E` salva e abre YAML e notas no editor, para parâmetros e notas. Um `run` de várias linhas só é editado no editor. O `r` de renomear saiu.
- **Gerenciador de arquivos:** `Ctrl+O` no scripts panel abre a pasta dos scripts. O config escolhe `terminal` (padrão: um painel de terminal nessa pasta), um gerenciador instalado (de terminal num painel: yazi, ranger, lf, nnn, vifm, mc; gráfico numa janela própria: thunar, nautilus, dolphin, nemo, pcmanfm, caja) ou um comando custom.
- **Custom no config:** editor e gerenciador de arquivos têm a escolha `custom`, e o comando é digitado na linha de baixo, no próprio painel, com o aviso `command not found` quando o programa não existe. O comando é salvo ao sair da linha ou com `Enter`; sair com ele vazio mantém a escolha anterior.
- **Local dos scripts** (`scripts_dir`) é configurável, com `~`; vazio é `~/.local/share/noct/scripts`. A TUI e a CLI usam o mesmo.
- **CLI:** `noct -s <script>` (ou `--script`, ou `noct run`) roda o script; se não existir, diz `Script not found at <local>`.

**Consequências.** Proporções ajustadas com o mouse sobrevivem a painéis novos (cada irmão cede uma parte proporcional) e a remoções. A cópia depende de o terminal de fora aceitar OSC 52 (kitty e Ghostty aceitam por padrão). Supersede a parte de mover da ADR-015 e o `r` de renomear da ADR-020.

## ADR-023 — Resize, fullscreen, painéis flutuantes, foco anterior e scripts quebrados

**Contexto.** O mantenedor pediu: redimensionar painéis pelo teclado (`prefixo` + `h`), um fullscreen só visual (`f`), painéis flutuantes (`g`) movidos e redimensionados com o mouse, a seleção de texto presa a um painel, o foco voltando ao painel anterior quando um fecha, e scripts que o editor quebrou continuando visíveis, com o erro e um backup.

**Decisão.**
- **Resize** segue o modo resize do i3: `→`/`↓` aumentam, `←`/`↑` diminuem, movendo a fronteira com o próximo vizinho no container mais próximo daquela direção, ou com o anterior quando o painel é o último. `h` deixou de ser atalho de navegação (as letras `hjkl` saíram do modo noct; as setas continuam). Revisto no ADR-030: `hjkl` voltaram como setas.
- **Fullscreen** é por tab e só visual: o painel com foco ocupa a tab e recebe esse tamanho; os outros guardam o tamanho do layout. As setas navegam pelo layout como antes.
- **Painéis flutuantes** ficam numa camada sobre os tiles, cada um com seu retângulo, o último por cima. Setas chegam a um flutuante a partir do tile embaixo do meio dele e saem pelo painel mais próximo naquela direção. Esquerdo na borda arrasta, direito na borda redimensiona pelos lados agarrados, e clique dentro dá foco, traz para a frente e seleciona texto. Quando o último tile fecha, o primeiro flutuante vira o tile da tab.
- **Foco anterior:** cada tab guarda o histórico de foco; fechar o painel com foco devolve o foco ao usado antes dele, como o i3.
- **Seleção** começa num painel e fica presa à área de conteúdo dele (e é desenhada antes dos flutuantes, que a cobrem).
- **Scripts quebrados** entram na biblioteca com o erro (`Problem`) em vez de sumir: `✗` na lista, erro nos detalhes, `e` abre no editor, `Enter` não roda. Antes de editar, `script.yaml` é copiado para `script.yaml.bak`, só quando carrega, para a cópia ser sempre a última versão que funcionava.
- O cursor e o bug do `ultraviolet` que o atrasava estão na ADR-011.

**Consequências.** A ADR-015 dizia que não haveria zoom; o fullscreen volta, mas sem mexer no layout. `noct check` falha com scripts quebrados, como antes, e `noct ls` os mostra.

## ADR-024 — Formulário do script por partes, dicas em scripts novos e Alt como atalho direto

**Contexto.** O mantenedor pediu: editar cada parte do script separadamente (a descrição sem mexer no script), com script e descrição lado a lado em caixas próprias; `Enter` abrindo o editor da parte, sem o `Ctrl+E` que abria o arquivo inteiro; um comentário explicando os campos no topo de scripts novos, desligável; a location dos scripts já preenchida e copiável; e `Alt` como modificador padrão dos atalhos diretos.

**Decisão.**
- O formulário tem `name` e `program` digitados na linha e quatro caixas, duas a duas com metade da largura cada: `script`, `description`, `params` e `notes`. `Enter` abre a parte selecionada sozinha num arquivo temporário (`script.sh`, `description.txt`, `params.yaml`, `notes.md`, para o editor realçar cada um) no editor do config; ao fechar, o conteúdo volta para a caixa e o formulário reaparece. `Ctrl+S` salva tudo; `Esc` descarta e apaga os temporários. Um `run` de várias linhas deixou de ser um caso à parte.
- Salvar valida o conjunto, inclusive que todo `{{.nome}}` do `run` é um parâmetro; antes, isso só falhava ao rodar. No `script.yaml`, só as chaves que mudaram são reescritas, e chaves vazias são removidas.
- Scripts novos são gerados a partir dos campos, com um comentário de 4 linhas sobre `description`, `program`, `run` e `params` no topo, quando `script_hints` está ligado (padrão).
- A location dos scripts aparece preenchida com a pasta em uso; `Ctrl+C` copia o caminho (OSC 52). Guardar o caminho padrão grava vazio, para o config continuar valendo em outra máquina.
- O modificador padrão dos atalhos diretos passa a ser só `Alt`. Com ele, `Alt+M` funciona (com `Ctrl+Alt` não funcionava). Em troca, `Alt+F` e `Alt+D` deixam de chegar ao shell como edição por palavra; `Ctrl+Alt` continua disponível no config.

**Consequências.** Parâmetros e notas passam a ser editados pelo formulário também; o editor com o `script.yaml` inteiro fica para scripts quebrados. Configs antigos que já gravaram `ctrl = true` no `[shortcut]` continuam com `Ctrl+Alt`.

## ADR-025 — Intro do título e `q` confirmando a saída

**Contexto.** O mantenedor pediu uma animação no título ao abrir o noct, com o texto alternando como binário até revelar o ASCII da esquerda para a direita em 500 ms, e `q` confirmando a saída além de `Enter`.

**Decisão.**
- Ao conectar com a home à vista, cada célula das letras do título mostra `0` ou `1`, sorteado a cada quadro (~30 ms); células em branco continuam em branco, então os dígitos já têm o formato de "noct". Cada coluna se revela num momento que combina uma varredura da esquerda para a direita (700 ms) com um atraso aleatório (até 300 ms): o jitter. Tudo termina em 1 s (eram 500 ms; o mantenedor pediu mais lento). O raven fica parado.
- Uma confirmação pode aceitar como "sim" a mesma tecla que a abriu: `prefixo` + `q` e depois `q` sai. Outras confirmações (apagar script) continuam só com `Enter`/`y`, para uma tecla repetida por engano não apagar nada.

**Consequências.** A animação só redesenha enquanto dura; não há opção para desligá-la, por ser curta e só na abertura.

## ADR-026 — Seleção que acompanha a rolagem, avisos no tema e ordem dos scripts

**Contexto.** O mantenedor notou que selecionar e rolar ao mesmo tempo perdia a seleção, que não dava para trocar de painel no meio de uma rolagem, que a cópia de uma execução incluía a linha de estado, que `copied to the clipboard` não combinava com o tema, e pediu para ordenar os scripts por nome, edição ou última execução.

**Decisão.**
- A seleção guarda posições como linhas do histórico (contadas da mais antiga), não como linhas da tela. Assim, rolar com a roda (que, durante a seleção, age sempre no painel selecionado) ou arrastar para fora do painel, que rola uma linha por vez, mantém o começo e move só a ponta. O texto copiado lê as linhas fora da tela no histórico, em blocos do tamanho da tela.
- Views podem declarar linhas de rodapé que não são texto (`FooterRows`) e a view de onde vem o texto (`Terminal`): a execução de um script exclui a linha de estado, que passou a usar a cor das dicas.
- No modo rolagem, o prefixo, os atalhos diretos e cliques ou a roda em outro painel encerram a rolagem.
- A barra de tabs separa problemas (cor de atenção) de avisos (cor de destaque do tema).
- `scripts_sort` escolhe a ordem: `name` (padrão), `edited` (data do `script.yaml`) ou `run`. Execuções da TUI e da CLI vão para `runs.json` no diretório de estado, limitado às últimas 1000; a fase 2 acrescenta a esse arquivo os fluxos, o código de saída e o log de cada execução (ADR-029).

**Consequências.** Com saída chegando durante a seleção, o texto sobe e a seleção sobe junto, como num terminal. `noct ls` continua em ordem alfabética.

## ADR-027 — Pastas de scripts em vez de criar scripts no projeto, e `noct -h`

**Contexto.** Com a fase 1 concluída, o mantenedor preferiu pastas de scripts a criar scripts no projeto: agrupar os próprios scripts em pastas e mostrar pastas de scripts de qualquer lugar pelo caminho. Pediu também `noct -h`.

**Decisão.**
- Dentro da pasta de scripts, uma subpasta sem `script.yaml` é uma **pasta de scripts** (um nível). Pastas em outros lugares são **vinculadas** pelo caminho (`script_folders` no config). O `.noct/scripts` do projeto vira a pasta `project`, e deixa de sobrepor scripts pessoais de mesmo nome: os dois aparecem.
- Cada script tem um **ID**, `pasta/nome` ou `nome`, usado nos valores lembrados, no histórico de execuções e em `noct -s`, que também aceita só o nome quando um único script o tem.
- No scripts panel, as pastas vêm antes dos scripts, qualquer que seja a ordem escolhida, e a ordem vale dentro de cada pasta. Pastas começam fechadas e abrem e fecham com `+`/`-` (o estado vale para a sessão); os scripts de uma pasta aberta ficam atrás de uma barra vertical, e a pasta onde a seleção está aparece com a barra na cor de acento. `n` pergunta se cria um `directory` ou um `script` (este na pasta selecionada), `a` vincula uma pasta, e `Delete` apaga uma pasta própria com seus scripts ou desvincula uma pasta vinculada.
- `noct -h`/`--help` lista comandos e flags.

**Consequências.** Scripts já existentes fora de pastas continuam iguais, com o mesmo ID. Valores lembrados e execuções gravados pelo nome valem para o script de mesmo ID; ao mover um script para uma pasta, eles precisam ser renomeados (foi feito para os scripts de exemplo em `tests-noct/`).

## ADR-028 — Fluxos: runner no CLI, saída por pipe e inspect em fluxograma

**Contexto.** A fase 2 começa pelos fluxos. O mantenedor pediu `prefixo` + `i` como "inspect", um modo fluxograma para criar e ver fluxos, arrastável dentro do painel e com zoom, com um popup no clique duplo que mostra o que um passo faz; e um exemplo de ponta a ponta (`pwd`, `ls` do caminho, mostrar as entradas, sortear um arquivo e uma cor, um dado no vermelho, e recomeçar num 6) montado pelo mesmo código do inspect.

**Decisão.**
- Pacote `flow` com o modelo, a validação, a edição por caminho do passo (`2/then/0`) e a execução; não há pacote `runner` separado. O nome do fluxo vem do arquivo em `~/.local/share/noct/flows/`.
- Quem roda é o CLI: `noct flow run <fluxo>`. A TUI roda esse comando num painel de execução, como um script, e o mesmo comando serve no terminal e, depois, aos triggers. O resultado da última execução fica em `flows/<fluxo>.json` no diretório de estado.
- Cada passo roda com a entrada do terminal e a saída passando por um pipe, que a mostra e a guarda para capturas e templates. Um PTY por passo manteria as cores de todos os programas, mas exigiria repassar tamanho e modo raw do terminal; o pipe basta para testes e fica para rever se atrapalhar.
- Cada passada de um `repeat` esquece os resultados dos passos de dentro da passada anterior, para que "recomeçar num 6" pare quando o dado não roda.
- O inspect desenha o fluxograma numa tela própria, do tamanho do fluxo, e mostra a parte que cabe no painel a partir de um deslocamento: arrastar, `Shift` + setas (hoje `Ctrl` + setas, pelo ADR-031) e a roda mudam o deslocamento; `+`/`-` mudam a largura e o detalhe das caixas. Só se encaixam blocos em sequência, dentro de `if` ou dentro de `repeat` (sem canvas livre).
- Passos são adicionados por teclas (o script é escolhido pela busca) e editados um a um no editor, como YAML. Salvar regrava o arquivo inteiro sem comentários: para fluxos, que a TUI edita, isso vale menos que para scripts, editados à mão. Uma edição que traria um problema novo não é salva, e o texto fica num arquivo temporário para não se perder.

**Consequências.** O exemplo `explore` usa o `pwd` do usuário e os scripts `tests-noct/ls-path`, `show-entries`, `pick` e `dice`; o teste `TestInspectBuildsAFlowStepByStep` monta um fluxo do mesmo formato só pelas teclas. Ficam para depois triggers, `on_fail: ask`, passos em painel próprio (`pane`, `background`) e parâmetros perguntados antes de rodar.

## ADR-029 — Logs com limites, calendário em `prefixo` + `h` e histórico em `runs.json`

**Contexto.** O mantenedor pediu um histórico com os logs: guardar por 90 dias e no máximo 200 MB, os dois configuráveis; ver os logs com barras de peso, ordenar por nome, data e peso; e trocar `prefixo` + `h` do resize para os logs por data, com um calendário navegável, quantas execuções houve em cada dia, peso (de cada log e do dia), linhas, e limpar um log, o dia ou tudo, com confirmação. O resize passa para `prefixo` + `n`.

**Decisão.**
- `[logs]` no config: `keep_days` (90) e `max_size_mb` (200). A limpeza apaga os logs mais velhos que o limite e depois os mais antigos até caber no tamanho; roda ao iniciar, de hora em hora, ao abrir a view e ao mudar os limites.
- A view de logs lê os próprios arquivos (`logs/<dia>/<hora>-<nome>.log`, com a parte rotacionada somada) e o `runs.json`. As linhas são contadas só do dia aberto e guardadas por tamanho do arquivo.
- A barra de peso vai do log mais leve (0%) ao mais pesado (100%) da lista mostrada, como foi pedido: ela compara os logs entre si, e o tamanho escrito ao lado dá o valor absoluto.
- O histórico fica em `runs.json` (as últimas 1000 execuções), agora com fluxos, o caminho do log e o código de saída, em vez do SQLite previsto: o que o calendário precisa (execuções por dia, falhas e o log de cada uma) cabe nele sem dependência nova. Se consultas maiores aparecerem (a IA na fase 3, por exemplo), o SQLite volta à mesa.
- `prefixo` + `i` é o inspect, que tira do catálogo (fase 4) a tecla prevista; o catálogo fica com `k` (hoje `u`, e os logs em `prefixo` + `v`, pelo ADR-030).

**Consequências.** Logs de terminais comuns também aparecem na view e contam no limite. Apagar um log tira o caminho dele das execuções. Ao ver um log, ele é redesenhado num terminal sem gerar log novo.

## ADR-030 — Teclas `hjkl` em todo lugar, pilhas de painéis e inserção exata no inspect

**Contexto.** O mantenedor pediu: mover em `prefixo` + `a` em vez de `m`; `hjkl` valendo como setas em todo lugar; os logs em `prefixo` + `v` em vez de `h`; `prefixo` + `w` para alternar o layout, com um layout empilhado como o do i3 ou do zellij; `prefixo` + `e` para renomear o painel. No inspect: `Tab` esconde ou mostra a lista de flows, lista e fluxograma com moldura, e adicionar um passo exatamente onde se está, antes ou depois de laços, `if` e passos, sem editar o YAML (no exemplo `explore`, o fluxo inteiro é um `repeat` e não havia como pôr algo antes dele).

**Decisão.**
- `hjkl` voltam ao modo noct como setas (desfazendo a saída delas no ADR-023), no modo mover, no resize, nos painéis e no inspect; `J`/`K` continuam movendo passos. Nos atalhos diretos com `Ctrl`, `h` e `j` ficam com o programa: o terminal manda `Ctrl+H` no lugar de Backspace (em muitos, no `Ctrl+Backspace`) e `Ctrl+J` é nova linha.
- Uma pilha é um estado do container da árvore do `layout` (`stacked`), não uma nova direção: o container guarda a direção para quando volta a ser dividido e qual filho mostra. O filho mostrado ocupa a altura menos uma linha por irmão; os outros viram uma linha de título. A tab mostra na pilha o painel com foco ao calcular os retângulos, então qualquer mudança de foco (teclas, mouse, fechar um painel) abre o painel certo. Painéis dobrados não são redimensionados, para não encolher o programa a uma linha. Um painel novo aberto numa pilha entra nela, já mostrado. A pilha não tem divisórias para arrastar, e o resize passa para o container de fora.
- O nome dado a um painel fica na tab (e vai com ele quando muda de tab); vazio volta ao título da view.
- No fluxograma, as setas entre os passos são pontos de inserção selecionáveis (lista e índice), inclusive antes do primeiro e depois do último passo de cada lista; `a` adiciona ali. `n` sai do inspect, e `a` cria um fluxo na lista, para que "adicionar" seja uma tecla só.
- As teclas reservadas para views futuras mudam: Execuções `o`, Aprovações `y`, Catálogo `u` (antes `e`, `a` e `k`, este último do ADR-029).

**Consequências.** Com o atalho direto em `Alt`, `Alt+H/J/K/L`, `Alt+A`, `Alt+W`, `Alt+E` e `Alt+V` passam a ser do noct e deixam de chegar ao shell (por exemplo `Alt+H`, o `run-help` do zsh); o modificador pode ser trocado ou desligado no config. Cada ramo de um `if` ganhou uma linha no fluxograma para a seta depois do último passo.

## ADR-031 — Variáveis dos fluxos: escopo, saída nos resultados, `outputs` e testes de trechos

**Contexto.** No inspect era difícil saber que variáveis existem em cada ponto de um fluxo e que valores tiveram. O mantenedor aprovou: um painel de escopo com os valores da última execução, avisos para referências erradas, inserir variáveis pela busca ao digitar, `outputs` declarados no script como retorno, e testar passos isolados (`t`), com `Shift` selecionando uma faixa contínua. Pediu também guardar a saída inteira dos passos nos resultados, com a opção de truncar no config, e truncar sempre nos testes.

**Decisão.**
- Continua o `text/template` do Go. O problema era descobrir as variáveis, não expressá-las; uma linguagem embutida (Lua, expr) seria dependência nova e outra sintaxe para aprender.
- O escopo é calculado pelo pacote `flow` a partir da árvore de passos (não do resultado), com três graus: garantido, talvez (dentro de um `if` anterior) e desta passada (dentro do `repeat` que contém o ponto).
- Referências erradas são avisos, não erros: a TUI recusa edições que criam erros de validação, e um erro aqui impediria montar o fluxo de cima para baixo.
- `StepResult` ganha `output` e `tried`. A saída guardada é a que os templates leem (até 1 MiB); `[flows] output_kb` no config limita (0 guarda tudo). Guardar tudo é seguro em espaço porque cada fluxo só mantém o resultado da última execução, substituído a cada run.
- Testar é `noct flow run <fluxo> --steps de..até`: roda uma faixa contínua de passos irmãos com `.steps` da última execução e mistura o resultado no último, com a saída truncada em 4 KB. Faixas com buracos ou atravessando listas ficam de fora: o trecho precisa ser algo que o fluxo rodaria em sequência.
- No inspect, `Shift` + `↑↓` passa a estender a faixa, e o pan pelo teclado vai para `Ctrl` + setas (revê o ADR-028).

**Consequências.** Um teste de um passo que lê algo de um passo que nunca rodou lê vazio, como no fluxo inteiro. Os scripts de exemplo `tests-noct/pick` e `dice` podem trocar seus `capture` por `outputs`, mas os existentes continuam funcionando.

## ADR-032 — Triggers no daemon, `on_fail: ask`, passos em painel e execuções no calendário

**Contexto.** Para fechar a fase 2, o mantenedor pediu o cron e o resto do que faltava (triggers de arquivo e de outro fluxo, `on_fail: ask`, passos em painel próprio e em segundo plano, abrir uma execução do calendário com o estado de cada bloco) e um filtro nos logs (`f`) que busca data, nome, peso ou linhas numa linha só.

**Decisão.**
- **Pacote `schedule`** no lugar do `trigger` previsto: só decide quando cada fluxo dispara (`Tick`); o `app`, dentro do daemon, roda uma goroutine com um tick a cada 2 s e dispara as execuções. Sem dependência nova: o cron é um parser de cinco campos próprio, e o `watch` compara, por varredura, caminho, tamanho e horário dos arquivos que casam (fsnotify seria dependência e não segue `**`).
- **`after` pelo resultado:** toda execução regrava `flows/<fluxo>.json` no fim, venha de onde vier, então o scheduler só observa quando o fim dali muda. Conta como novo o fim posterior ao último visto ou, a primeira vez, ao início do scheduler. Laços de `after` entre fluxos viram problema ao carregar.
- **Habilitar fica fora do arquivo** (`triggers.json` no estado): um fluxo recebido via git não dispara sozinho (visão original em fluxos.md).
- **Execução por trigger sem painel:** `noct flow run` como processo do daemon, com a saída num log, registrada em `runs.json` com o trigger (`Run.Trigger`). Uma segunda disparada do mesmo fluxo enquanto a primeira roda é ignorada.
- **`on_fail: ask` no terminal da execução:** o runner pergunta no próprio terminal (o painel da execução, na TUI), sem popup do noct; sem terminal, para. Assim a pergunta funciona igual na TUI e no shell, e o runner continua sem falar com o daemon para isso.
- **`pane: new` pelo socket:** o `noct flow run` de uma execução da TUI recebe `NOCT_PANE` e pede ao daemon um painel ao lado (`Purpose: pane` no hello, com o pedido; `ipc.Version` passa a 2). O daemon responde quando o programa sai, com o código, ou logo que o painel abre, para `background`. O painel abre na tab da execução sem trocar a tab na tela; em segundo plano, o foco fica onde estava. Fora do workspace, o passo roda no terminal.
- **Resultado por execução ao lado do log** (`.result.json`), pedido por `NOCT_FLOW_RESULT`: conta no limite dos logs e é apagado com eles, e o calendário (`i`) abre a execução no inspect com o estado de cada bloco.
- **Filtro dos logs** numa linha: palavras em nome, data, hora e status; comparações de peso (com unidade ou `size`) e de linhas (número sem unidade ou `lines`). Filtrar lista todos os dias.

**Consequências.** Triggers só disparam com o workspace no ar; execuções perdidas não são recuperadas. `watch` custa uma varredura a cada 2 s, limitada a 50 mil arquivos por caminho vigiado; padrões sobre pastas enormes devem ser mais estreitos. Um `noct` antigo e um daemon novo (ou o contrário) se recusam a conversar até `noct quit`.

## ADR-033 — Servidor MCP sem SDK, pedidos ao daemon, aprovações e rascunhos

**Contexto.** O mantenedor pediu o MCP com todas as ferramentas possíveis antes de confirmar a fase 2 (ver [mcp.md](mcp.md)): listar, ler e conferir scripts e fluxos, rascunhar, rodar e acompanhar execuções com resultado estruturado, buscar nos logs, ver e usar painéis, com a política `allow`/`ask`/`deny`, aprovações na TUI, auditoria e rascunhos que só rodam depois de promovidos.

**Decisão.**
- **Sem SDK MCP.** O que o noct usa do protocolo (JSON-RPC por stdio: `initialize`, `ping`, `tools/list`, `tools/call`) cabe em um arquivo; o SDK oficial traria dependências para isso. Recursos e prompts do MCP não entram: as ferramentas cobrem o que a IA precisa.
- **Pacote `mcp` lê arquivos direto e pede o resto ao daemon.** Como a CLI, lê scripts, fluxos, `runs.json`, resultados e logs dos arquivos, que são a fonte da verdade. Painéis, execuções que o usuário vê e aprovações vão ao daemon por um pedido no hello (`Purpose: request`, `ipc.Request`, resposta com `Exit.Data`), iniciando o daemon quando preciso. Revê a regra de que só `ipc` sai dos clientes: ler arquivos, a CLI já fazia.
- **A política decide no `noct mcp`, a aprovação no daemon.** Cada ferramenta tem um padrão; `[mcp.policy]` o troca; palavra desconhecida vale `ask`. `ask` pede ao daemon (`approve`), que guarda o pedido, avisa na barra de tabs e espera `prefixo` + `y` ou o fim de `approval_seconds`. A tecla `y` era a reservada para Aprovações; virou o popup do MCP, com as aprovações no topo, os clientes conectados e o comando para adicionar o noct a cada cliente, para quem configura a IA achar tudo num lugar só.
- **Presença sem protocolo novo.** O cliente conectado é um pedido `client` que o daemon nunca responde: dura o que dura a conexão, e fechar (o `noct mcp` saiu) o tira da lista. Não inicia o daemon, para `noct quit` não ser desfeito por uma IA aberta; reconecta a cada 3 s.
- **Rascunhos** (`draft: true` em fluxos e scripts): escritos pela IA com `draft_flow` e `draft_script`; não rodam à mão, por trigger, pela IA nem como passo de outro fluxo. O usuário promove com `P` (com confirmação) ou `noct promote`. Substituir o que não era rascunho guarda uma cópia `.bak`.
- **`run_id` é o caminho do log** da execução, que já identifica cada uma no calendário e guarda o resultado ao lado; `get_run` espera o fim com `wait_seconds` lendo `runs.json`.
- **Auditoria** em `logs/mcp.log`, com rotação como os logs dos painéis.

**Consequências.** `noct mcp` num terminal sem daemon o inicia na primeira ferramenta que precisa do workspace. Segredos não saem pelas ferramentas de scripts, mas o que os programas imprimem nos logs vai como está. `install_program` espera a fase 4.

## ADR-034 — Params com tipo, formulário `p`, passos `prompt`, `set` e `unset`

**Contexto.** O mantenedor pediu, para testes que precisam de valores ao começar, uma tela `p` de params no inspect; um passo `prompt` que exige intervenção imediata do usuário e grava a resposta numa variável, nova ou existente; um jeito de passos de comando criarem variáveis; e um garbage collection de variáveis. Pediu também um fluxo de exemplo (`explore2`) que usa tudo.

**Decisão.**
- **Params com tipo sem quebrar o formato:** cada param continua podendo ser só o padrão (`env: dev`) e passa a aceitar um mapa com `default`, `type` (`text`, `number`), `prompt` e `required`. O `flow.Param` decodifica os dois e grava o mapa só quando precisa, então fluxos antigos são salvos iguais. `CheckParams` confere nomes, números e obrigatórios em `noct flow run` e no formulário.
- **Um namespace de variáveis:** params, `set` e `prompt` gravam em `{{.nome}}`; os resultados dos passos continuam em `.steps`. Um `prompt` pode trocar um param, e isso é o "editar variáveis que já existem".
- **`set` e `unset` como passos próprios**, e não campos do `run`: um tipo por passo, como os outros, com bloco próprio no fluxograma. `set` renderiza todos os valores antes de gravar. `unset` aceita nomes e `steps.<id>`; o que foi apagado lê vazio (e não `<no value>`: o render dá vazio a todo campo de topo que falta).
- **`prompt` no terminal da execução**, como o `on_fail: ask` (ADR-032): o runner pergunta e lê a linha. Para a intervenção ser imediata, o `noct flow run` de uma execução do workspace pede ao daemon `attention` pelo pedido do MCP (`Purpose: request`, sem mudar `ipc.Version`): o daemon mostra a tab da execução, dá o foco ao painel e avisa na barra; o runner toca o sino. Sem terminal (trigger, headless), o passo falha na hora em vez de esperar, e `type: number` desiste depois de 3 respostas, para uma entrada fechada não virar laço.
- **Escopo:** `set` e `prompt` entram no escopo com o caminho do passo (o painel scope e o MCP mostram o último valor gravado); `unset` tira. Ler uma variável que nada grava, gravada só depois, apagada antes ou gravada dentro de um `if` anterior gera aviso, como já acontecia com `.steps`. Testar uma faixa (`t`) recupera as variáveis gravadas antes dela na última execução.
- **Formulário `p`:** o formulário de parâmetros dos scripts passou a servir aos dois (título, prévia e envio por quem abre). Os valores de um fluxo ficam no `params.json` com a chave `flow:<nome>`; `r` roda com eles e abre o formulário quando falta um obrigatório.

**Consequências.** Um fluxo com `prompt` não serve para triggers, porque ninguém responde. Uma variável com o mesmo nome de um param o esconde até o fim da execução. O `explore2` usa `awk` para as contas (aceita decimais, que `$(( ))` não aceita) e um texto com `'` quebra a linha de shell do `check`, porque templates entram na linha sem aspas próprias.
