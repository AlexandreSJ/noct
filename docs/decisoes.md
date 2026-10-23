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
- `TerminalScreen.Flush` do `ultraviolet` põe o movimento do cursor no buffer do renderer depois de esse buffer já ter sido escrito, então o cursor só chega ao terminal no quadro seguinte: ele fica sempre um passo atrás (a seta anda, o cursor não). O noct chamava `Render` e `Flush` de novo depois de cada `Display`; desde a ADR-035 a cópia em `third_party/` escreve o movimento junto com o quadro. Um teste e2e cobre.

**Vazão.** O perfil mostra dois pontos quentes: `ultraviolet.Buffer.DeleteLineArea` copia a tela inteira célula por célula a cada linha rolada, e `vt.Scrollback.Push` desloca todo o histórico a cada linha quando ele está cheio. Corrigir os dois numa cópia local levou o emulador de 1,9 para 5,9 MB/s. O restante vem de alocar uma string por caractere impresso e exige mudanças mais profundas no emulador.

**Consequências.** Os problemas acima devem ser reportados aos projetos da Charm, com as correções de vazão como pull requests. Se a vazão continuar insuficiente no uso real (logs muito verbosos desaceleram o programa que os produz, por contrapressão do PTY), as opções são: manter um fork com as correções via `replace` no `go.mod`, ou escrever um emulador próprio sobre `charmbracelet/x/ansi`.

**Decisão do mantenedor (fase 1):** Go confirmado, e as correções da Charm ficam no noct. `ultraviolet` e `x/vt` estão copiados em `third_party/` como pacotes do próprio módulo (imports reescritos, sem `go.mod` próprio), com as duas correções marcadas com `noct:`: `Buffer.DeleteLineArea` move linhas inteiras em vez de copiar célula por célula, e `Scrollback.Push` descarta a linha mais antiga sem deslocar o histórico inteiro. A vazão do emulador foi de 2,5 para 7,6 MB/s. Não se usa `replace`, porque `go install …@latest` recusa módulos com `replace` para caminhos locais. [third_party/README.md](../third_party/README.md) diz como atualizar as cópias. As correções continuam candidatas a pull request para a Charm.

**Ghosts na tela (2026-09-28).** Às vezes, células antigas ficavam na tela (restos do painel de config sobre a home, restos no Claude CLI) até algo ser escrito em cima. O renderer do `ultraviolet` só envia o que mudou em relação ao modelo que ele guarda do terminal; quando o modelo erra, o erro fica. Um teste que reproduz quadros aleatórios num emulador `vt` e compara com o esperado (`third_party/ultraviolet/noct_render_test.go`) achou quatro erros, corrigidos na cópia com `noct:`:
- `Line.Set`: um caractere largo sobre a primeira metade de outro deixava a segunda metade como célula de largura 0, que `TerminalScreen.Render` pula; aquela coluna nunca mais era enviada.
- `Buffer.DeleteCellArea` (o `DCH` dos painéis) copiava célula por célula com `SetCell`, que apagava um caractere largo no meio da cópia: apagar antes de um emoji ou CJK o fazia sumir.
- `transformLine` ia para `n+1` depois de um caractere largo em `n`, caindo na segunda metade dele; o `EL` seguinte apagava o caractere. No ramo de inserção, o laço que tentava pular essa metade nunca avançava e travava o daemon.
- `clearBottom` limpava a partir da linha acima da primeira linha em branco, e essa linha não era redesenhada se não tivesse mudado.

**Atalhos atrasados com o mouse se mexendo (2026-10-15).** Na home, com o mouse passando pela arte (que pede todo movimento ao terminal), `Alt` + letra (`Alt+T`, `Alt+P`) só agia quando o mouse parava. O `TerminalReader` segura um evento de dois bytes que começa com `ESC` (`ESC t` é `Alt+T`) esperando mais bytes, porque podia ser o começo de outra sequência, e cada leitura nova reinicia essa espera; ele esperava mesmo com o próximo relatório do mouse já no buffer. A cópia (`noct:`) só espera quando nada vem depois do evento; com bytes depois, o evento está completo. `Alt` + setas não sofria disso, por serem sequências longas. `third_party/ultraviolet/noct_reader_test.go` cobre.

O caso da home (sem caracteres largos) não se reproduz no emulador: a sequência enviada está correta. Para esse, `NOCT_TRACE=arquivo noct` grava os bytes que o cliente escreve no terminal, para comparar o que o kitty mostrou com o que os bytes pedem.

**Título com `✳` (2026-09-28).** Com o Claude Code num painel, a borda do painel ficava desalinhada, e o nome dele aparecia dentro do painel. O título `✳ Claude Code` chega por OSC, e em UTF-8 o `✳` é `E2 9C B3`. A tabela do parser do `x/ansi` trata `0x9C` como o terminador C1 (ST) mesmo dentro de uma string, então o título virava `\xe2` e o resto era impresso na tela. Esse byte solto ainda seguia até o terminal real, que o juntava ao `ESC` seguinte e perdia a posição. Isso afeta todo o bloco U+2700–U+273F (`✅`, `✔`, `✨`, `✳`) e qualquer caractere com `0x9C` no UTF-8, também em links OSC 8 e em DCS. Como no xterm e no kitty em UTF-8, a cópia do `vt` passa a tratar `0x9C` em strings OSC e DCS como dado (`init` com `noct:` em `emulator.go`); `ESC \` e `BEL` continuam terminando. O parser não aceita tabela própria, então a mudança vale para a tabela compartilhada, que no noct só o `vt` usa.

**Texto que some ao estreitar (2026-10-20).** O mantenedor notou que, ao diminuir a largura do terminal, o fim das linhas sumia e não voltava ao aumentar de novo, o que perde logs e valores ainda por ler. O `Buffer.Resize` do `ultraviolet` cortava cada linha na largura nova e, ao crescer, acrescentava células em branco. A cópia (`noct:` em `buffer.go`) mantém o fim cortado na capacidade da própria linha e o mostra de novo ao alargar; como o fim é da linha, ele acompanha a linha quando a tela rola. Limpar uma linha até o fim (`ED`, `EL`, `clear`) limpa também o fim escondido, para nada apagado voltar. Linhas que viraram histórico enquanto o terminal estava estreito ficam como estavam; não há reflow. `third_party/vt/noct_resize_test.go` cobre. Depois: a linha que vai para o histórico enquanto o terminal está estreito leva o fim escondido junto (`Scrollback.Push` copia até a capacidade), e a linha que uma rolagem reaproveita embaixo perde o fim escondido da linha que ela era, que senão reapareceria ao alargar.

**Emoji com seletor de variação (2026-10-20).** Uma linha com `➡️` (o Claude Code usa em respostas) puxava a borda direita uma coluna para a esquerda, só naquela linha. `➡️` é `U+27A1` + `U+FE0F`: pelas regras de grapheme do Unicode são duas colunas, mas pelo wcwidth, que ignora o seletor, só uma. O programa no painel e o emulador `vt` contam duas; o terminal de fora pode desenhar em uma, e o renderer do `ultraviolet` supõe que o cursor andou a largura da célula e escreve o resto da linha em sequência. A cópia (`noct:` em `terminal_renderer.go`) trata como de largura disputada a célula larga cuja largura pelo wcwidth difere: preenche as colunas dela com espaços, volta, escreve o grapheme e leva o cursor com `CHA` para a coluna depois da célula. Num terminal que desenha estreito, o emoji fica com um espaço ao lado e o resto da linha no lugar. Na última coluna isso não é feito, porque o cursor ficaria esperando a quebra e nenhum movimento diz isso; ali o `CR` do movimento seguinte já realinha. `third_party/ultraviolet/noct_render_test.go` cobre, reproduzindo o quadro no `vt` com e sem o seletor.

**Log que termina em tela cheia (2026-10-23).** O mantenedor notou logs de milhares de linhas no `prefixo` + `v` que não rolavam nada. O preview reproduz o log num terminal emulado, e um log que termina dentro de um programa de tela cheia (`?1049h` sem o `?1049l`, porque o painel fechou com a TUI aberta) deixa o emulador na tela alternativa, que não tem histórico. As milhares de "linhas" eram redesenhos da TUI, não texto que rola. A cópia do `x/vt` ganhou `Emulator.MainScreenCellAt` (`noct:` em `emulator.go`, testado em `noct_altscreen_test.go`), que lê a tela principal com a alternativa à mostra; com `HistoryUnderFullScreen`, só no preview, o histórico do painel termina na tela principal, embaixo do último quadro do programa, e o título avisa `▣ ends in a full-screen program`.

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

*Atualizada pela ADR-019:* tabs não ficam mais vazias; fecham com o último painel. *E pela ADR-039:* uma tab pode abrir ou ficar vazia, se o config pedir.

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
- `prefixo` + `g` (era `[`, que no atalho direto `Alt+[` se confunde com `Alt+Esc`, e depois `u`; desde 2026-10-23 `u` abre o painel flutuante) rola o histórico do terminal com foco, com as teclas do vim e as setas; qualquer outra tecla volta ao vivo. Programas de tela cheia não têm histórico, a não ser no preview dos logs (ADR-011).

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

**Consequências.** Não existe mais tab vazia visível: `x` sempre fecha um painel, e `p`/`s` só abrem uma tab nova a partir da home. `Ctrl+D` no último shell da última tab leva à home, onde `Ctrl+D` duas vezes sai. *Atualizada pela ADR-039:* uma tab nova pode abrir vazia, mostrando a home, quando o config escolhe `empty`, e uma tab pode ficar vazia com `close_empty_tabs` desligado.

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

*Atualizada pela ADR-045:* mover pensa em colunas, e empilhar vale para o painel e os de baixo dele na coluna.

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

*Atualizada pela ADR-043:* o config `animate intro` decide quando a intro roda (só na home, sempre ou nunca), e uma tab vazia conta como home.

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

*Atualizada pela ADR-044:* o tamanho vai em `max_size_kb`, os limites aceitam valores digitados, e terminais só são gravados com `[logs] terminals` ligado.

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

## ADR-035 — Synchronized output (mode 2026) nos painéis e na saída

**Contexto.** Com o opencode num painel, o cursor piscava enquanto a IA pensava. Cada quadro do spinner chega como `ESC[?2026h ESC[?25l …desenho… CUP ESC[?25h ESC[?2026l`. O emulador ignora o mode 2026, e o noct desenha até 60 vezes por segundo quando chega saída: pegando o quadro no meio, via o cursor escondido ou na posição do spinner. Na saída do próprio noct, cada flush ia entre `ESC[?25l` e `ESC[?25h`, e o movimento do cursor ia num segundo write (o contorno da ADR-011), com o cursor visível no último ponto desenhado entre os dois. O caminho WSL → ConPTY → Alacritty deixava esses estados intermediários à vista.

**Decisão.**
- **Painéis:** o `pane` segura a saída de um quadro sincronizado, de `ESC[?2026h` a `ESC[?2026l`, e a passa inteira ao emulador (`internal/pane/sync.go`). Um marcador cortado entre duas leituras também é reconhecido. Um quadro que não termina em 150 ms (o tempo do Alacritty) ou passa de 1 MiB vai como está. A saída continua indo para o log na hora. Só a forma literal dos marcadores conta, que é a que os programas usam.
- **Saída do noct:** cada quadro vai como uma atualização sincronizada (`SetSynchronizedUpdates`), no lugar de esconder e mostrar o cursor. Mostrar ou esconder só é enviado quando muda. A cópia do `ultraviolet` escreve o movimento do cursor no mesmo write do quadro, e o `render` deixou de fazer o segundo `Render`/`Flush`.

**Consequências.** Respostas a consultas feitas no meio de um quadro (`ESC[6n`) saem quando ele termina. Num terminal sem o mode 2026, o cursor pode ser visto andando durante o desenho de um quadro, já que não é mais escondido.

**Cursor andando nas animações (2026-10-21).** Com as animações, essa consequência apareceu: ao trocar o foco com `fade`, o cursor piscava no painel e em pontos da borda, porque cada quadro da animação muda células longe dele e o terminal mostrava o cursor indo até cada uma. O mesmo vale para qualquer saída fora do painel com foco. Agora o noct pergunta ao terminal, ao conectar, se ele tem o mode 2026 (`DECRQM`, `ESC[?2026$p`). Só com a resposta "reconhecido" (ligado, desligado ou sempre ligado) os quadros vão como atualização sincronizada; até a resposta, e num terminal que não responde ou não tem o mode, o cursor é escondido durante o quadro e mostrado no fim, no mesmo write, como o `ultraviolet` faz sem o mode. Um terminal intermediário (como o ConPTY) que diga ter o mode sem passá-lo adiante ainda mostraria o cursor andando.

## ADR-036 — Painéis de scripts e fluxos acompanham o disco

**Contexto.** O inspect e o scripts panel liam os arquivos ao abrir. Um fluxo que a IA criava ou transformava em rascunho pelo MCP (que escreve os arquivos direto, ADR-033) só aparecia depois de fechar e abrir o painel.

**Decisão.** Enquanto um inspect ou scripts panel aparece na tab atual, com um cliente conectado, o laço do daemon confere a cada segundo uma assinatura das pastas: nome, tamanho e data de cada arquivo, sem ler o conteúdo (até três níveis nas pastas de scripts, um na de fluxos). Só quando ela muda os scripts ou fluxos são relidos; scripts novos releem também os fluxos, que são conferidos contra eles. Um tick sem mudança não redesenha a tela. Sem `fsnotify`: seria uma dependência nova, precisaria de um watch por pasta de script e não pega mudanças em pastas ligadas que estejam em sistemas de arquivos como o 9p do WSL (`/mnt/c`), e a varredura custa poucos `stat`.

**Consequências.** Uma mudança aparece em até um segundo. Painéis em outras tabs e o daemon sem cliente não olham o disco; ao voltar para eles, a próxima conferência os atualiza.

## ADR-037 — Sessões com nome, reabertas depois de fechadas

*Atualizada pela ADR-046:* o comando que rodava em cada shell é guardado e oferecido de novo ao reabrir, e o shell continua o próprio log.

**Contexto.** O mantenedor pediu sessões com attach e detach: várias, com nome, que continuem existindo mesmo fechadas e possam ser reabertas; um popup em `prefixo` + `o` com o último acesso, renomear e apagar; `noct` voltando à última sessão e um comando para entrar numa sessão pelo nome. Configurações por sessão ficam para depois. Era também o item "retomar a sessão ao reconectar" da lista de qualidade de vida.

**Decisão.**
- **Um daemon, várias sessões**, como no tmux: o `App` guarda as sessões e aponta para a que está na tela; as tabs saíram do `App` para a sessão. Triggers, MCP e aprovações continuam do daemon. Os ids de painel são únicos entre as sessões, então pedidos por painel (`NOCT_PANE`, MCP) acham o painel em qualquer sessão, e um shell que sai numa sessão de fundo deixa a tab dela. `attention` traz a sessão do painel para a tela. Revê o "um cliente por vez" do ADR-017 só no que ele mostra: continua um cliente, agora numa das sessões.
- **Protocolo:** o `Hello` do attach leva `Session` e `NewSession`; sem nome, a última usada. Uma sessão que não existe é recusada com `Exit.Refused`, e o cliente sai com o motivo. `ipc.Version` passa a 3. `noct sessions` e `noct kill` usam o pedido do MCP (`sessions`, `delete_session`) com o noct rodando, e leem ou editam `sessions.json` sem ele, para não iniciar o daemon só para listar.
- **CLI:** `noct attach <nome>` com os apelidos `a`, `att` e `-a`/`--attach` (como `-s` para scripts); `noct new [nome]`; `noct sessions`; `noct kill <nome>`. Nomes sem espaços, porque são digitados na linha de comando; sem nome, o primeiro número livre.
- **Reabrir é refazer o layout, não o processo.** Fechar uma sessão (ou `noct quit`) grava, por tab, a árvore do layout (`layout.Shape`) e os painéis que sabem recomeçar: shells, com o diretório atual lido de `/proc/<pid>/cwd` (fora do Linux, o diretório do noct), e os painéis scripts, inspect e logs, com nomes e painéis flutuantes. Execuções, editores e ajuda terminam com o que faziam e ficam de fora; a árvore se fecha sobre eles ao reabrir. Programas dentro dos shells não voltam: guardar processos seria outro projeto (CRIU), fora do escopo.
- **Novo pacote `session`** para o arquivo, porque a CLI o lê sem o daemon; o `app` monta e desmonta as sessões. O arquivo é gravado inteiro por um temporário, ao trocar, desconectar, fechar, apagar e renomear, no quit e a cada 30 s (só quando mudou), para os diretórios sobreviverem a uma queda.
- **Popup `o`** da mais recente à mais antiga, com aberta/fechada, tabs e último uso; `x` fecha e `d` apaga, os dois com confirmação, porque param programas. A barra de tabs só mostra o nome da sessão quando há mais de uma.

**Consequências.** Sessões abertas no fundo continuam gastando o que seus programas gastam; fechar é o jeito de liberar sem perder o layout. Um shell reaberto começa limpo: histórico da tela, variáveis e o programa que rodava nele se perdem. Um `noct` antigo e um daemon novo (ou o contrário) se recusam a conversar até `noct quit`. A única sessão não fecha (fechar seria sair); `noct quit` faz isso guardando o layout.

## ADR-038 — Calendário dos logs colorido pela atividade

**Contexto.** O calendário dos logs marcava cada dia com um de poucos estados (nada, logs, execuções, falha): um dia com um log e um com trezentos pareciam iguais. Um heatmap à parte, no estilo do GitHub, embaixo dos totais, foi testado e descartado pelo mantenedor: repetia o calendário. O pedido ficou em pôr as cores de calor no próprio calendário, sem a cor de atenção nos dias com falha, que destoava do tema.

**Decisão.**
- **Cor no dia:** cada dia com logs fica sobre uma cor pela quantidade deles, em quatro passos pela raiz quadrada da fração do dia com mais logs entre todos os guardados (a mesma escala em todos os meses). As cores vêm de `mix` entre o `background` e o `accent` do tema (30% a 100%). A cor usa o `background` do tema mesmo com o fundo do terminal, porque a paleta não tem outra base.
- **Célula centralizada:** o dia é ` 28 `, quatro colunas com a cor em todas e os dígitos nas mesmas colunas do nome do dia da semana, em vez de `%3d` com o fundo rente ao número, que parecia alinhado à direita. Dias vizinhos com logs encostam as cores; um espaço entre eles pediria cinco colunas por dia, mais que a coluna do calendário.
- **Legível em qualquer passo:** nos dois primeiros o número vai na cor do texto; nos dois últimos, na cor que o tema usa sobre o destaque.
- **Sem decorações:** nenhum dia fica em negrito. O dia selecionado é só sublinhado, mantendo a cor dos seus logs, em vez do fundo de destaque; hoje aparece entre colchetes nas margens da célula (`[30]`, `[ 1]`), que se leem sobre qualquer cor. O `•` dos dias com execuções saiu.
- **Sem cor de atenção:** um dia com execução que falhou não muda de cor; as falhas aparecem na lista do dia (`✗`) e nos totais do mês.
- **Legenda** logo abaixo das seis semanas, antes dos totais: `activity less` e os quatro passos. Os totais descem uma linha, e o calendário do layout estreito passa de 17 para 18 linhas.

**Consequências.** A cor mede logs, e terminais também gravam logs: um dia de muitos shells aparece quente sem ter rodado scripts. Um dia com falha, ou só com execuções sem log, não se destaca mais no calendário. Em terminais sem true color, as cores descem para a paleta de 256 cores e os passos podem ficar próximos.

## ADR-039 — Painel de uma tab nova e tabs vazias escolhidos no config

**Contexto.** `t` abria sempre uma tab com um terminal. O mantenedor pediu que o config escolha com o que uma tab nova abre: vazia, terminal (o padrão), scripts, fluxos, logs e qualquer tipo de painel que venha depois, de preferência sem precisar lembrar de pôr o tipo novo na lista.

**Decisão.**
- **Uma lista de tipos de painel** (`panelKinds`, em `internal/app/panel_kinds.go`): nome, tecla do modo noct e como o painel começa. `runAction` abre `p`, `s`, `i` e `v` por ela, e as opções do config (`empty` e os nomes, na ordem da lista) também saem dela. Um tipo novo entra numa linha; a tecla ainda precisa de uma linha em `commandShortcuts` e de um lugar em `actionKeys`, e um teste falha quando falta. O painel flutuante (`g`) não entra: é um lugar, não um tipo, e abre um terminal.
- **Config:** `new_tab` guarda o nome (`"terminal"`, `"flows"`), não a tecla, para o arquivo continuar valendo se uma tecla mudar. Vazio no arquivo vira `terminal`; um nome que o noct não conhece abre um terminal, sem erro. No painel, cada opção aparece com as teclas que abrem aquele painel, pelo atalho direto configurado (`terminal (alt+p)`) ou pelo prefixo quando os atalhos estão desligados.
- **Fechar quando vazia:** `close_empty_tabs` (padrão ligado, como a ADR-019). Desligado, uma tab cujo último painel fecha (`x`, o programa terminar, `Ctrl+D` num painel do noct) fica, vazia. Vale só para painéis que fecham, no `detachView`: mover o último painel para outra tab fecha a de origem de qualquer jeito, porque o painel não fechou e aquele caminho reorganiza as tabs em seguida; e uma tab que falha ao abrir também some.
- **Tab vazia** (revê a ADR-019): mostra a home na tab. Um painel aberto nela a ocupa; `x` e `Ctrl+D` a fecham, em vez de `Ctrl+D` duas vezes sair do noct como na home sem tabs. Os comandos que agem sobre painéis (mover, redimensionar, empilhar, tela cheia, renomear painel, histórico) não fazem nada nela; renomear a tab funciona. Uma tab vazia não é guardada ao fechar a sessão.

**Consequências.** Uma tab nova de scripts, fluxos ou logs não tem shell; `p` abre um ao lado. Os outros caminhos que criam tabs (abrir um painel a partir da home, pedidos de painel pelo MCP e pelo `NOCT_PANE`, mover um painel para depois da última tab) continuam abrindo o que pediram.

## ADR-040 — Config com mouse, prefixo capturado e volta ao padrão

*Atualizada pela ADR-044:* os resets só aceitam `Enter`, e o segundo precisa vir em até 1000 ms.

**Contexto.** O mantenedor pediu o config clicável (hover, clique nas setas e em `on`/`off`), um prefixo com quantos modificadores o usuário quiser, pego apertando a combinação e com aviso do que ela já usa, e uma última linha para voltar tudo ao padrão. É a primeira de quatro partes; as outras são o mapa de teclas (todas as teclas trocáveis, por tela) e o mesmo "voltar ao padrão" nele. Personalizar teclas é polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- **Mouse:** `configLayout` põe o painel, as linhas e a rolagem num lugar só, que o desenho, o clique e o hover leem. Com o config aberto, o noct pede ao terminal todo movimento do mouse (modo 1003), como na home, para marcar a linha sob ele. Clique no rótulo seleciona; no valor, `◂` volta e o resto avança, como `←`/`→`, o que cobre as setas, `on`/`off` e valores sem setas (`background`, o prefixo); numa linha de texto, só seleciona. A roda move a seleção; um clique fora fecha, como `esc`.
- **Prefixo capturado:** uma linha `prefix` no lugar de `ctrl`, `alt`, `shift` e `key`. A combinação é a que o terminal mandou, sem uma tabela do que cada terminal distingue: `ctrl+shift+a` chega e aparece como `ctrl+a`. Duas fases, para `Enter` também poder ser apertado como combinação: a primeira tecla é a combinação, `Enter` depois a usa; `esc` cancela, e por isso não pode ser o prefixo. Continua exigindo `ctrl` ou `alt` e recusando o `shift` que o terminal não manda (`shiftIsVisible`).
- **Avisos, não proibição:** a linha lista o que a combinação tomaria: o comando de um atalho direto (o prefixo é conferido antes, então o comando só sairia com o prefixo) e teclas comuns de programas nos painéis (`terminalKeys`: `ctrl+c`, `ctrl+d`, `ctrl+r`, `ctrl+b` do tmux…). Quem quer mesmo aquela combinação a usa.
- **Voltar ao padrão:** a última linha pede `Enter` duas vezes, sem sair do config (a confirmação comum fecharia o painel); mover a seleção desarma. Volta `config.Default()` em tudo o que o painel mostra, inclusive a pasta dos scripts e as teclas trocadas (ADR-041); as pastas extras (`script_folders`) e a política da IA, editadas fora dele, ficam.

**Consequências.** O arquivo continua com `ctrl`, `alt`, `shift` e `key` em `[prefix]`; só o painel mudou. Os atalhos diretos continuam como modificadores ligados e desligados. A lista de teclas de programas é curta e não sabe o que o usuário usa de fato.

## ADR-041 — Teclas trocáveis por contexto: registro e tradução

**Contexto.** O mantenedor pediu para trocar todas as teclas, uma a uma, em cada tipo de tela, com aviso quando a tecla já faz outra coisa e uma volta ao padrão. As teclas estavam em 22 funções, com cerca de 170 comparações de texto (`case "enter"`), e as dicas de rodapé eram textos fixos. Personalizar teclas é polimento da fase 5, antecipado pelo mantenedor. Das três formas avaliadas (trocar os handlers por ações, só uma camada de apelidos, ou um registro com tradução), ele escolheu a última.

**Decisão.**
- **Registro** (`keyContexts`, em `internal/app/keymap.go`): cada contexto (o modo depois do prefixo, cada painel e popup do noct) lista suas ações com nome, rótulo e teclas padrão. Uma ação agrupa só teclas que o handler trata igual em todo o contexto (`up` e `k`); onde a mesma tecla faz coisas diferentes em partes do painel (`enter` na lista e no gráfico do inspect), ela é uma ação só, com um rótulo que diz as duas.
- **Tradução, handlers intactos:** antes do handler, `keymap.resolve` troca uma tecla trocada pela primeira tecla padrão da ação, que o handler conhece, e barra as teclas padrão que a ação perdeu. Teclas não trocadas passam como estão, então sem mudanças no config tudo funciona como antes. O contexto vem do modo, ou do painel com foco no modo normal; terminais, execuções rodando, o prefixo e as capturas de tecla não passam pela tradução. Os atalhos diretos traduzem a tecla pelo contexto `commands`.
- **Limites:** `esc` nunca muda (cancela em todo lugar); onde se digita (contextos `typing`), caracteres sem `ctrl` ou `alt` não são tomados nem barrados. Números de tab, as letras das opções de uma escolha e o que vai para programas ficam fora do registro.
- **Dicas pelo registro:** as dicas marcam a tecla padrão entre chaves (`{enter} run`), e `hint` escreve a tecla atual na mesma posição da ação, ou nada; chaves vizinhas formam grupos (`↑↓`, `shift+↑↓`, `ctrl+p/↓`). O popup de comandos, o de mover e redimensionar, as perguntas, a busca, os prompts e rótulos como `terminal (alt+p)` passam por ela.
- **Config:** `[keys.<contexto>] <ação> = [teclas]`; lista vazia deixa a ação sem teclas, e ações iguais ao padrão saem do arquivo. Nomes desconhecidos são ignorados.
- **Página de binds** no config, reaproveitando as linhas, o mouse e a rolagem do painel: `Enter` troca uma tecla (com várias, primeiro escolhe qual com `←→`, entre colchetes; um clique numa tecla a escolhe direto), `a` acrescenta, `Delete` volta ao padrão. As teclas são contadas como aparecem, uma por tecla apertada (`G` e `shift+g` são uma), com a captura em duas fases do prefixo (ADR-040). Recusa o prefixo, teclas que não se leem de volta e caracteres onde se digita; avisa a outra ação do contexto que já tem a tecla, e usá-la tira a tecla de lá. A última linha volta todas as teclas ao padrão, com `Enter` duas vezes.

**Consequências.** Um handler novo precisa entrar no registro e ter suas dicas com chaves para as teclas dele poderem mudar; um teste confere que toda tecla padrão se lê de volta e não se repete no contexto. O registro achou um erro antigo: no histórico, `G` não ia ao fim porque o handler esperava `shift+g`, que o terminal manda como `G`. Uma ação trocada manda a mesma tecla padrão em todas as partes do painel, então os grupos do registro precisam continuar fiéis aos handlers.

## ADR-042 — Um campo de texto para todo lugar onde se digita

**Contexto.** O mantenedor notou que os campos de texto (prompts de renomear tab, painel e sessão, nova sessão e pasta, o formulário de parâmetros, a busca, o nome e o programa do formulário do script e as linhas de texto do config) só escreviam no fim e apagavam o último caractere: sem setas, sem colar, sem `Ctrl+W`. Cada tela repetia o mesmo código de acrescentar e apagar. É polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- **Um componente, `textField`** (`internal/app/textfield.go`): texto e cursor, com as teclas de edição de uma linha do shell. `←`/`→` (`Ctrl+B`/`Ctrl+F`), `Ctrl`/`Alt` + setas e `Alt+B`/`Alt+F` por palavra, `Home`/`End` (`Ctrl+A`/`Ctrl+E`), `Backspace`, `Delete` (`Ctrl+D`), `Ctrl+W`, `Ctrl+Backspace` e `Alt+Backspace` apagam a palavra anterior (a maioria dos terminais manda `Ctrl+Backspace` como `Ctrl+H`, que vale igual), `Alt+D` a seguinte, `Ctrl+U` até o começo e `Ctrl+K` até o fim. O cursor aparece como `▏` no meio do texto, e o texto rola para ele continuar à vista.
- **As telas só dizem qual campo está sendo digitado** (`typedField`) e o que fazer quando ele muda (filtrar a busca, gravar o valor do formulário). Teclas que a tela usa vêm antes: `↑`/`↓` e `Tab` trocam de campo; `←`/`→` escolhem nas `choices` e, num campo de texto, movem o cursor; na busca e no config, `Home`/`End` continuam indo ao começo e ao fim da lista, e o cursor vai com `Ctrl+A`/`Ctrl+E`.
- **Colar:** o terminal que cola em `Ctrl+V`/`Ctrl+Shift+V` manda o texto como bracketed paste, que agora vai para o campo em vez de só para o painel com foco. Se a tecla chega crua, o noct pede a área de transferência ao terminal (OSC 52) e digita a resposta. Quebras de linha e tabs viram espaços. `Ctrl+C` copia o texto do campo (OSC 52).
- **Dicas:** o `←→ choose` do formulário de parâmetros só aparece num campo com `choices`.
- **O seletor de variáveis dos prompts de template sai de `Ctrl+V` para `Ctrl+T`**, para `Ctrl+V` colar em todos os campos.

**Consequências.** Um campo novo usa `textField` e entra em `typedField`, e ganha as mesmas teclas e o colar. `Ctrl+U` passa a apagar até o cursor, como no shell; com o cursor no fim, continua limpando tudo. Ler a área de transferência por OSC 52 depende do terminal (kitty pede permissão; outros ignoram), mas os terminais que colam sozinhos não precisam dele.

## ADR-043 — Quando a intro roda: `animate intro`

**Contexto.** O mantenedor pediu para escolher quando a intro do título (ADR-025) roda: como hoje, só ao abrir na home; sempre, mesmo com a sessão cheia de painéis; ou nunca. Também notou que a intro não rodava quando a tab à vista era uma tab vazia (a home aparece, mas só se conferia se a sessão não tinha tabs). É polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- `Appearance → animate intro` (`animate_intro` no arquivo): `empty` (padrão), `always` ou `never`. Valores desconhecidos viram `empty`.
- **`empty`:** a intro roda ao conectar quando a home está à vista: sessão sem tabs ou tab à vista sem painéis. Uma pergunta só (`showsHome`) decide isso para o desenho, o mouse e a intro, para não divergirem de novo.
- **`always`:** com painéis à vista, a home fica na tela por cima deles durante a intro (1 s) e mais uma pausa de 1000 ms, e então a sessão aparece. Uma tecla ou um clique nesse meio pula para a sessão e não faz mais nada, para não agir num painel que não estava à vista. Com a home já à vista, é igual a `empty`.
- **`never`:** nunca roda, nem com a home à vista.

**Consequências.** `always` atrasa a sessão em 2 s a cada conexão, o que é o pedido; quem tem pressa aperta uma tecla. Os programas nos painéis continuam rodando e recebendo o tamanho da tela enquanto a home os cobre.

## ADR-044 — Limites de log digitados, terminais fora dos logs e reset só com `Enter`

**Contexto.** O mantenedor pediu valores livres para os limites dos logs (99 dias, 222 MB, 20000 KB), uma opção para não gravar os terminais, cujos logs enchiam com o histórico do shell sem uso, e notou que o reset do config armava e confirmava com `←`/`→`, o que podia apagar tudo por acidente. É polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- **Limites digitados:** em `keep` e `max size`, `←`/`→` continuam nos valores prontos; `Enter` abre o valor atual para editar e um dígito começa um novo, no campo de texto de sempre (ADR-042). `Enter` salva e `Esc` descarta, sem fechar o config; sair da linha descarta. Dias vão de 1 a 36500; o tamanho aceita `KB`, `MB` e `GB` (sem unidade, MB; decimais com ponto ou vírgula), de 1 KB a 1 TB. Um valor que não se lê fica aberto com o problema embaixo.
- **Tamanho em KB:** para `20000 KB` caber sem arredondar, o arquivo passa de `max_size_mb` para `max_size_kb`. Um `max_size_mb` de arquivos antigos ainda é lido e vira KB; o noct não o grava mais.
- **Confirmação antes de apagar:** mudar `keep` ou `max size` (pelas setas, digitando ou pelo reset do config) conta antes, com `logstore.Expired`, os logs que os limites novos apagariam. Se há algum, uma pergunta diz quantos e quanto ocupam; `n`/`Esc` mantém os limites e volta ao config (com o texto digitado, se havia). Sem nenhum a apagar, aplica direto. A limpeza de hora em hora e ao iniciar segue sem perguntar, porque usa os limites já aceitos.
- **Barra de uso:** embaixo de `max size`, a barra da view de logs (`drawBar`) mostra quanto todos os logs ocupam do tamanho máximo, com a porcentagem, o total e quantos logs são. O total é lido ao abrir o config e depois de cada mudança de limite, não a cada quadro.
- **`[logs] terminals`**, desligado por padrão: painéis de terminal (o shell e o terminal do file manager) não gravam log. Scripts, fluxos, passos, comandos da IA, ajudas e editores continuam gravados, porque o histórico de execuções e a IA leem esses logs. Vale para terminais abertos depois da mudança.
- **Reset só com `Enter`:** nas duas linhas de reset (config e teclas), setas, espaço e cliques não fazem nada. O primeiro `Enter` arma por 1000 ms (`resetWindow`), e só um segundo `Enter` nesse tempo reseta; depois disso o aviso some e o próximo `Enter` arma de novo.

**Consequências.** Quem tinha `max_size_mb` no arquivo passa a ver `max_size_kb` ao salvar algo no config. Logs de terminais deixam de existir por padrão, então a view de logs e o calendário mostram só execuções, a não ser que `terminals` seja ligado. O reset ficou mais difícil de disparar sem querer, inclusive com o mouse.

## ADR-045 — Mover por colunas, empilhar o painel e os de baixo, e o foco no painel que a pilha mostra

**Contexto.** O mantenedor notou três coisas nos painéis. Mover (`prefixo` + `a`) para o lado levava o painel para o topo da coluna vizinha, ou o deixava como coluna própria entre duas, ou só o trocava de lugar com o vizinho, quando ele queria que o painel saísse da coluna e fosse para a vizinha, com as colunas tendo prioridade sobre o resto. `prefixo` + `w` empilhava o container do painel, que com colunas lado a lado era a tab inteira, quando ele queria empilhar só o painel e os de baixo dele. E, depois de mover um painel, as setas às vezes iam para um painel dobrado de uma pilha em vez do que ela mostrava, abrindo outro. Testamos antes entrar na coluna vizinha na altura do painel (como o i3, que entra junto ao último filho focado) e trocar de lugar com o vizinho (como o `move` do zellij); o mantenedor preferiu manter o lugar do painel na coluna, nenhuma troca e o fim da coluna para um painel sozinho. É polimento do layout (ADR-022), antecipado pelo mantenedor.

**Decisão.**
- **Colunas primeiro:** uma coluna é um container vertical ou um painel sozinho num container lado a lado. Painéis lado a lado dentro de uma coluna contam como colunas próprias.
- **`←`/`→`:** o painel sai da coluna dele e vai para o mesmo lugar (índice) da coluna vizinha. Se estava sozinho na coluna, ou a vizinha tem menos lugares, vai para o fim dela; um painel sozinho do outro lado vira uma coluna de dois. Numa pilha, entra do mesmo jeito e fica aberto. Não há troca: com `1 | 2`, mover o 1 para a direita deixa 2 sobre 1.
- **`↑`/`↓`:** o painel anda pela coluna; uma fileira de painéis lado a lado conta como um lugar, e numa pilha ele entra pela ponta e fica aberto. No primeiro ou no último lugar, sai da coluna e fica ao lado dela no container de fora que corre na vertical, com a largura dele; sem nenhum, a tab passa a correr na vertical, e o painel ocupa a largura toda (o `move` do i3).
- **Borda da tab:** com `←`/`→`, um painel que divide a coluna vira uma coluna nova na borda, entrando no container lado a lado que já existe, e cada coluna fica com uma parte igual da largura (um painel de largura inteira sobre `2 | 3` vira a terceira coluna de 1/3); só um painel que já é uma coluna sozinho na borda vai para a tab vizinha.
- **Tamanhos:** cada movimento redivide por igual o container de onde o painel saiu e o em que ele entrou (1/2, 1/3...), desfazendo ajustes feitos nesses dois. As fronteiras arredondam frações que a conta em ponto flutuante deixa um fio abaixo do valor (0,4999…), para painéis iguais terem o mesmo tamanho e as bordas de colunas divididas igual se alinharem.
- **Empilhar** (`prefixo` + `w`) junta numa pilha o painel com foco e os painéis abaixo dele na coluna; os de cima ficam divididos. Se ele é o primeiro, a coluna inteira empilha; se é o último, ou está sozinho na coluna, nada empilha (`nothing below this panel to stack`). Em um painel da pilha, desfaz a pilha e devolve os painéis à coluna. A pilha dentro de uma coluna é o único container com a mesma direção do pai.
- **Foco nas pilhas:** entre painéis igualmente próximos, as setas preferem o que a pilha mostra a um dobrado, antes do histórico de foco. Quando o painel aberto sai da pilha, ela mostra o que ficou no lugar dele, não o primeiro.

**Consequências.** Não há como deixar um painel como coluna própria entre duas colunas de uma vez: ele vai para o fim da vizinha, e só vira coluna própria na borda da tab. Pilhas de painéis lado a lado deixam de ser criadas; sessões salvas com uma ainda abrem como estavam, e `prefixo` + `w` nelas desfaz a pilha.

## ADR-046 — Rodar de novo o que os shells rodavam, e o log do painel ao reabrir

**Contexto.** O mantenedor pediu que, ao voltar ao noct depois de `prefixo` + `q` (ou de fechar a sessão), um painel que rodava, por exemplo, o Claude Code ao menos dissesse isso e oferecesse rodar o mesmo comando de novo com `Enter`/`Esc`, como o zellij faz ao ressuscitar uma sessão; sem retomar a conversa do programa nem a senha de um ssh. Com o log dos terminais ligado, que o painel mostrasse mais do que tinha antes. Ele também cogitou inverter `q` e `d`; ficou como está, porque `d` (desconectar com tudo rodando) e `q` (parar os programas e guardar o layout) são o que tmux e zellij fazem, e `noct attach` sem nome passou a listar as sessões. É polimento das sessões (ADR-037), antecipado pelo mantenedor.

**Decisão.**
- **O comando vem do terminal, não do shell:** o grupo de processos em primeiro plano do pty (`TIOCGPGRP`) e a linha de comando dele em `/proc/<pid>/cmdline`, com aspas de shell onde precisa. Fora do Linux nada é oferecido: o `ps` junta os argumentos com espaços e perde as aspas, e `foo 'a; b'` voltaria como `foo a; b`. Um argumento com caractere de controle também não é oferecido, nem um comando assim lido de `sessions.json`, porque digitado no shell ele viraria uma tecla do editor de linha. Funciona com qualquer shell, sem ganchos no `.zshrc`; um pipeline guarda o primeiro programa, e um `sh -c` cujo último comando o shell executa no lugar guarda esse comando.
- **Gravado com a sessão** em `sessions.json` (`command` no painel), a cada gravação (troca, desconexão, 30 s, quit). No quit, a gravação acontece antes de os programas pararem: o cliente se desconecta antes de os painéis fecharem, porque desconectar grava de novo e, depois, os shells já não diriam o comando.
- **Ao reabrir**, o shell começa normal, e uma caixa no centro do painel diz o programa e a linha. `Enter` digita a linha no shell e aperta enter, em vez de rodá-la como programa do painel: ela entra no histórico do shell, e o shell fica quando o programa termina. `Esc` esquece. Outras teclas esperam, como no zellij. A tecla `enter` é trocável no contexto `rerun` (ADR-041). Sem resposta, o comando continua guardado.
- **Log contínuo:** com `[logs] terminals` ligado (ADR-044), o painel guarda o caminho do log (`log`), e o shell reaberto escreve depois do que ele tem (`logstore.Append`) em vez de começar outro. Os últimos 256 KB aparecem acima do prompt. Eles passam antes por um emulador à parte, com o que ele responderia descartado, e chegam ao painel só como texto e cores: repetidos crus, as perguntas da saída antiga (posição do cursor, cores) seriam respondidas ao shell novo como se o usuário as digitasse, e um programa de tela cheia deixaria a tela alternativa ligada.

**Consequências.** Rodar de novo é rodar a mesma linha: programas que retomam o próprio estado (`claude --resume`) precisam que a linha diga isso. Um comando com segredos na linha de comando fica gravado em `sessions.json`, como já fica no histórico do shell. O log de um terminal pode atravessar dias e reaberturas, numa só entrada com a data em que começou.


## ADR-047 — Releases, versão na barra de tabs e atualização pelo próprio noct

**Contexto.** O mantenedor quer que, assim que uma tag de versão subir no GitHub, todo usuário veja que há uma atualização e consiga instalá-la sem digitar comandos para baixar ou compilar o noct de novo, sempre com o aceite dele; quer ver a versão do noct antes do nome do workspace, um changelog por versão e não gosta de atalhos com letra maiúscula. Até aqui não havia tags nem releases: a instalação era só `go install`. O daemon é dono de todos os painéis (ADR-017), então trocar o binário em disco não basta: o daemon segue com o código antigo até reiniciar, e reiniciar fecha os programas. A primeira release é a v0.1.0, o primeiro beta. É polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- **Releases por tag:** um push de tag `vX.Y.Z` roda `.github/workflows/release.yml`: `make check`, `make dist` (linux e darwin, amd64 e arm64, `noct_<versão>_<os>_<arch>.tar.gz` com o binário, LICENSE, NOTICE e README, e `checksums.txt` com os sha256) e `gh release create` com a seção da versão no `CHANGELOG.md` como texto (`make release-notes`). Uma tag com `-` (`v0.2.0-rc.1`) vira prerelease. Sem GoReleaser: o Makefile e um workflow curto bastam e qualquer um roda `make dist` localmente.
- **Versão no binário:** `-ldflags -X main.version=<tag>` no build de release; sem ela, a versão que o `go install` grava (`debug.ReadBuildInfo`) quando baixou o módulo, com checksum, ou `dev`. Um `go build` num checkout também grava uma pseudo-versão do commit (`v0.0.0-…+dirty`), mas sem checksum: é `dev`, para a atualização não trocar um build local por uma release. `noct version` imprime.
- **Changelog:** o `CHANGELOG.md` (só em inglês, porque é texto de UI) segue o Keep a Changelog: uma seção `## Unreleased` no topo, que cada mudança percebida pelo usuário preenche no próprio commit, e uma seção `## vX.Y.Z - AAAA-MM-DD` por versão, com os grupos `### Added`, `### Changed`, `### Fixed`, `### Removed` e `### Security`, escrita para quem usa o noct (regras no CONTRIBUTING.md §5); o popup mostra os grupos sem os `###`; ela é o texto da release no GitHub, e o noct lê os textos das releases pela API para o popup.
- **Quem procura é o daemon,** não o cliente: lista as últimas 30 releases (`api.github.com/repos/AlexandreSJ/noct/releases`), sem rascunhos e sem tags que não são versão. Build `dev` não procura sozinho.
- **Cache:** o que a última procura achou fica em `~/.local/state/noct/updates.json` (as releases com o changelog, quando foi, o `ETag` da resposta e até quando o GitHub pediu para esperar). Ao iniciar, o daemon e o popup mostram o cache, e o daemon só procura sozinho quando ele tem mais de 24 h, conferindo a cada hora (um computador que dormiu procura logo depois de acordar). Toda procura manda o `ETag` em `If-None-Match`: sem releases novas, o GitHub responde 304, que não conta no limite de 60 por hora. O popup diz quando foi a última procura (`checked 2 h ago`). `noct update` sempre pergunta de novo, sem o cache. Procuras ficam a pelo menos 1 minuto uma da outra, para segurar `c` não gastar o limite, e quando o GitHub recusa por limite (403 com `X-RateLimit-Remaining: 0`, ou 429) a próxima espera o `X-RateLimit-Reset` (ou o `Retry-After`) que ele informa; enquanto isso, `c check` some das dicas.
- **Config:** `[update] check` (`Updates → check for new versions`), ligado por padrão; desligado, só o popup procura. Não há opção de instalar sozinho: a versão nova só valeria depois de um reinício, que também precisa do aceite, e instalar sem o usuário pedir é o que o mantenedor não quer.
- **Barra de tabs:** à direita, depois da mensagem e antes do nome da sessão, `noct v0.1.0`, e na cor de destaque `↑ v0.2.0` quando há uma mais nova, ou `v0.2.0 ready: restart` depois de instalada.
- **Teclas:** o config sai de `prefixo` + `c` para `prefixo` + `,` (`Alt+,`), como o `Cmd+,` de configurações; `prefixo` + `c` (`Alt+C`) passa a abrir o popup **updates and changelog**. Ações `config` e `updates` no contexto `commands`, e um contexto `updates` com as teclas do popup (ADR-041), todas minúsculas: `c` procura, `u` instala, `y` copia o comando, `r` reinicia, `q`/`Esc` fecha, setas rolam.
- **Instalar** (`u`, ou `noct update`, que pergunta `[y/N]`): baixa o `.tar.gz` do sistema e o `checksums.txt`, confere o sha256, extrai o `noct` para um arquivo temporário na pasta do binário (com os links seguidos) e o renomeia por cima, então o binário nunca fica pela metade e o daemon que roda continua com o dele. Sem permissão de escrita na pasta (`/usr/local/bin`), nada é baixado: o popup mostra o comando que instala com `sudo` (`curl … | tar -xzf - -C /tmp noct && sudo install -m 755 /tmp/noct <caminho>`), que `y` copia, e `noct update` o imprime.
- **Reiniciar** (`r`, depois de instalar, com confirmação): o daemon para como no quit, gravando as sessões, mas manda `restart` ao cliente (`ipc.Restart`). O cliente restaura o terminal e se troca (`exec`) pelo binário em disco, que inicia o daemon novo e abre a sessão usada por último, a que estava na tela. Sem reiniciar, a versão nova vale no próximo `noct` depois de um quit.

**Consequências.** Quem instalou com `go install` também atualiza pelo noct: o binário da release substitui o de `~/go/bin`. Um gerenciador de pacotes que venha a empacotar o noct teria o binário trocado por fora dele; se isso acontecer, o noct deve deixar a atualização com o gerenciador. Reiniciar fecha os programas dos painéis, como o quit, e oferece rodá-los de novo (ADR-046). A comparação de versões segue o semver (pseudo-versões do `go install` de um commit ficam entre as releases), sem dependência nova. Quem trocou a tecla do config no arquivo continua com a dele; quem usava `prefixo` + `c` passa a cair no popup de atualizações.

## ADR-048 — Terminal novo no diretório do painel com foco

**Contexto.** Um terminal novo abria no diretório do terminal de fora que fez o último attach. Como o daemon sobrevive aos attaches, esse lugar mudava conforme de onde se reconectou, e um `cd` num painel não valia para o terminal seguinte. Kitty, WezTerm, Ghostty e tmux (`-c "#{pane_current_path}"`) abrem no diretório do painel atual. É polimento da fase 5, antecipado pelo mantenedor (ver [polimento.md](polimento.md)).

**Decisão.**
- `p`, `g` e o terminal de uma tab nova (`t`) abrem o shell no diretório do painel com foco (`terminalDir`); a tab nova herda do painel com foco da tab que se deixa, lido antes de ela existir, o primeiro que der resposta: o diretório atual do shell, lido de `/proc/<pid>/cwd`; numa execução de script ou passo de fluxo, o diretório em que ela começou (o `runView` o guarda); ao lado de painéis sem diretório (scripts, inspect, logs, ajuda, editor) e na home, `~`; num shell cujo diretório não se lê, o diretório do attach, como antes.
- Um diretório que não existe mais (o Linux o mostra como `/caminho (deleted)`) dá `~`. Sem `$HOME`, o diretório do attach.
- Sem opção no config: é o comportamento padrão.

**Consequências.** No macOS o diretório de um shell não se lê (não há `/proc`), então lá os terminais continuam abrindo no diretório do attach, como ao reabrir sessões. Um shell rodando `ssh` dá o diretório local, não o do servidor. A execução dá o diretório em que começou, não o atual do programa dela.

## ADR-049 — Temas em arquivo, com todos os componentes

**Contexto.** Os temas eram sete, fixos, de sete cores cada, das quais o noct derivava todos os estilos: não dava para mudar só o nome da tab inativa ou a borda sem foco, nem criar um tema. O mantenedor quer cada componente trocável e temas criados pelo usuário, editados no próprio noct ([temas.md](temas.md)). É polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- **Pacote `theme`**, sem depender do `app`: os 21 componentes (fundo, texto, nome e fundo das tabs ativa e inativa, bordas e títulos com e sem foco, texto, apagado, cabeçalho, linha selecionada, fundo dos popups, mensagem, sucesso, erro, corvo, calendário), as cores, os temas embutidos e os arquivos.
- **Cores:** `#rrggbb`, um índice de 0 a 255 (com ou sem aspas no arquivo), ou vazio, que segue o fundo do tema (com `background = terminal`, o do terminal). Só os fundos de tabs, linhas e popups podem ficar vazios: um fundo, um texto ou um calendário vazio é erro do arquivo, porque não haveria cor para desenhar nem para misturar. Os índices 0–15 continuam índices, porque são as cores do terminal de fora; os outros viram hex.
- **Arquivo:** `~/.config/noct/themes/<nome>.toml`, com `base` (outro tema, embutido ou do usuário, até 8 níveis) e só os componentes que diferem dela, em tabelas (`[tabs]`, `[panels]`, `[noct]`, `[state]`, `[other]`, `[background]`). Os embutidos não podem ser sobrescritos por nome.
- **Leitura:** ao iniciar e a cada segundo, junto do ticker da biblioteca, sempre. Um arquivo com erro (cor inválida, chave desconhecida, base que não existe ou em ciclo) fica de fora, e a mensagem diz qual. O tema em uso que some volta ao padrão.
- Os embutidos mantêm as cores de antes, e ganham sucesso e erro.

- **Editor no config:** abaixo de `theme`, `new theme` (nome, depois o tema de onde começa, escolhido na busca, que mostra cada um na tela) e `edit theme` (o tema da sessão; num embutido, uma cópia com outro nome). O editor é uma página do config, como a dos binds, com a lista dos componentes por grupo à esquerda e o palco à direita (uma mini-workspace desenhada pelo noct, com um pouco de cada componente), num painel de 84 colunas. O componente selecionado pisca 400 ms no palco.
- **Seletor de cores:** uma grade 16×16 com as 256 cores, que rola quando não cabe, a cor selecionada (índice e hex) e um campo `#rrggbb` aberto com `#`. O palco mostra a cor sob a seleção antes do `Enter`; `Enter` grava o arquivo na hora, `Esc` volta sem mudar.

**Consequências.** O `✓` e o `✗` das execuções passam a ter as cores de sucesso e de erro. O fundo dos popups pode ser diferente do fundo do tema. As teclas do editor e do seletor são as do config, com `#` fixo. O tema por sessão está na ADR-050.

## ADR-050 — Tema por sessão

**Contexto.** Com várias sessões, nada mostrava em qual se estava além do nome na barra de tabs. O mantenedor quer que o tema seja da sessão, e que escolher uma sessão já a mostre nas cores dela, antes de abri-la, como ajuda visual. A ADR-037 deixou configurações por sessão para depois; o tema é a primeira. É polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- O tema vai em cada sessão do `sessions.json` (`theme`). Sem tema, ou com um que não existe mais, a sessão usa o `Appearance → theme` do config.
- `Appearance → theme` muda o tema da sessão na tela e também o do config, que vale para as sessões sem tema.
- Uma sessão nova nasce com o tema da sessão na tela.
- Trocar de sessão (pelo popup, pela busca, pelo attach, ao fechar ou apagar) troca o palette.
- **Prévia:** no popup de sessões, a seleção pinta a tela com o tema da sessão selecionada; na busca, quando a seleção está num item de sessão. Fechar sem trocar volta ao tema da sessão na tela.

**Consequências.** As outras opções do config continuam valendo para o noct inteiro. A prévia só muda o desenho; nada é gravado até a sessão ser aberta.

## ADR-051 — Cores por painel e por tab

**Contexto.** O mantenedor quer marcar um painel (o shell de produção em vermelho) e uma tab por cor, por cima do tema da sessão, e que as cores fiquem com a sessão e com os templates ([temas.md](temas.md#cores-por-painel-e-por-tab)). Pediu as teclas com shift (`prefixo` + `P`), embora a ADR-047 registre que ele não gosta de maiúsculas: só `m` e `z` estavam livres. É polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- **Componentes:** um painel muda borda e título, com e sem foco, fundo e texto; uma tab muda nome e fundo, ativa e inativa. A ausência quer dizer a cor do tema. Prioridade: painel e tab > tema da sessão.
- **Fundo e texto do painel** funcionam como o `background = theme`, só naquele painel (o `themedScreen` por painel): mudam o padrão dos programas, e as cores que eles escolhem ficam.
- **Popup `prefixo` + `P`:** a lista do painel e da tab à esquerda, o palco à direita com as cores em escolha, o seletor de cores dos temas com `t` para voltar à do tema.
- **Aviso de contraste**, sem bloquear: abaixo de 4,5:1 para texto e de 3:1 para bordas e títulos, sobre o fundo do painel (WCAG 2), e só nos pares em que uma das cores é do painel; os do tema são do tema.
- **Persistência:** as cores vão no `sessions.json`, por painel e por tab, com as chaves do arquivo de tema (`panels.focused_border`); passam com o painel quando ele muda de tab e somem quando ele fecha.
- **`Alt+Shift` + letra** vale como a maiúscula no atalho direto (com o atalho só em `alt`): `Alt+Shift+P` abre o popup, mesmo que `p` não seja um comando nos binds. `Alt+Shift+hjkl` continuam movendo o painel. `Shift` com um símbolo não é comando: com o protocolo de teclado do kitty, `Alt+Shift+,` chega como `,` com `Shift` e iria para o config. O popup anda e escolhe com as teclas do contexto `config` dos binds, as mesmas das dicas, e fecha quando o painel dele fecha.

**Consequências.** Com o atalho direto em `ctrl`, não há atalho para `P`: na codificação legada `Ctrl+Shift+P` chega como `Ctrl+P` (ADR-012). Os templates gravam as mesmas cores (ADR-052).

## ADR-052 — Templates de tab

**Contexto.** O mantenedor quer montar uma tab uma vez (layout, painéis, cores, programas) e abri-la de novo onde quiser, escolhendo por painel se o comando roda na hora (um `fastfetch`) ou pergunta antes (um `claude`). Era o item "layouts nomeados" de [tui.md](tui.md). É polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- **Um template é uma tab**, num arquivo YAML em `~/.config/noct/templates/<nome>.yaml` (pacote `templates`): o nome da tab, as cores dela, e o layout como `columns`/`rows` aninhados, com `share` só quando a divisão não é igual e `stacked` para pilhas. Cada painel é `terminal`, `scripts`, `inspect` ou `logs`, com `name`, cores e, num terminal, `run` e `start`. As cores usam as chaves do arquivo de tema, como na sessão.
- **Gravar (`prefixo` + `R`)** monta o template a partir da tab, como a sessão faz: execuções, editores, ajuda e painéis flutuantes ficam de fora, e o layout se fecha sobre eles. Cada terminal com comando escolhe `run`, `ask` (padrão) ou `none`. Um nome que existe pergunta antes de sobrescrever.
- **Abrir (`prefixo` + `T`)** lista os templates com a miniatura do layout, nas cores salvas, e abre uma tab nova no fim, com o foco no primeiro painel. Os terminais abrem no diretório que um terminal novo teria (ADR-048). `run` digita o comando no shell; `ask` o oferece na caixa `run?` (ADR-046). Um comando com quebra de linha não é digitado nem oferecido, e um painel que não abre fica de fora, com a mensagem. O arquivo é relido no `Enter`.
- `e` edita o arquivo no editor do config, `r` renomeia (sem sobrescrever) e `d` apaga, com confirmação.
- **Arquivo estrito e fiel.** Um campo que o noct não conhece no topo ou num painel (`comand`) é erro, não é ignorado; só um tipo de painel desconhecido continua aceito, de um noct mais novo. Nomes e comandos são gravados como texto, então `null`, `~` ou `true` voltam como foram escritos. `shown` grava o painel que uma pilha mostra. Quando um painel fica de fora, os outros do grupo mantêm a proporção entre si. Os painéis abrem na ordem do layout.
- **Um template que não se lê** aparece na lista com `cannot read` e o erro no lugar da miniatura: `Enter` mostra o erro, e `e`, `r` e `d` continuam valendo para consertá-lo ou apagá-lo. Renomear para o mesmo nome não faz nada.

**Consequências.** Templates de um projeto (vindos por `git pull`) e sessões a partir de templates ficam para depois: com `run`, um template de outra pessoa rodaria comandos ao abrir. Os diretórios de cada terminal não são gravados.

## ADR-053 — Animações por momento

**Contexto.** Havia só a intro do título e o `01` ao passar o mouse nele. O mantenedor quer animações ao abrir e fechar painel e tab, no foco, ao mover, na tela inteira, nos popups, no hover e na atividade, com efeitos à escolha (`01`, mola, fade, jitter, flicker), uma prévia ao escolher e um aviso de epilepsia no flicker ([animacoes.md](animacoes.md)). É polimento da fase 5, antecipado pelo mantenedor.

**Decisão.**
- **Momento × efeito:** `Appearance → animations` liga ou desliga tudo, e `[animations]` no config dá o efeito de cada momento e `[animation_ms]` o tempo dele, de 50 a 2000 ms. Os efeitos são `none`, `01`, `fade` e `jitter`. Padrões: fade para painéis, tabs, mover, tela inteira, foco e atividade, jitter nos popups, `01` no hover. Um nome desconhecido, ou um tempo fora dos limites, volta ao padrão.
- **Retrato do estado no lugar de ganchos:** antes de cada quadro, o que mudou desde o anterior (painéis, tabs, foco, quadro do painel com foco, tela inteira, modo) começa a animação do momento. Fechar usa as células do quadro anterior, guardadas só enquanto fechar painéis ou tabs anima. Fechar um popup toca o efeito ao contrário sobre as células que ele tinha no último quadro, que só são copiadas enquanto há popup aberto com o efeito ligado.
- **Só o desenho muda:** a camada de animações corre depois dos painéis e, para popups, sobre as células que o popup desenhou. Os programas são redimensionados uma vez, como antes. Foco, mover, hover e atividade só tocam a moldura e a barra.
- **Foco:** só o painel que ganha o foco anima. Uma tecla não termina essas animações, que só tocam as bordas: trocar o foco rápido deixa um rastro, e o painel que perdeu o foco antes do fim volta à cor dele. Trocar de tab as termina.
- **Teclas:** qualquer outra tecla termina as animações e é tratada normalmente. Trocar de sessão ou o tamanho do terminal também termina.
- **Prévia:** com uma linha de momento selecionada, o config mostra o palco ao lado, tocando o efeito uma vez com a duração real; `space` repete; sem animações, o palco fica parado com `animations off`.
- **Hover** liga o modo de movimento do mouse em todo lugar; os programas só recebem movimento se o pediram, e um movimento que não muda nada na tela (fora de nomes de tabs e títulos, sem programa que o queira) não pede quadro. **Atividade:** a primeira saída de um shell fora de vista pulsa o título (ou o nome da tab) uma vez, até o painel ser visto.
- Código próprio (easing, ruído), sem dependência nova. O relógio das animações (`App.clock`) é trocável nos testes.

**Consequências.** O hover anima nomes de tabs e títulos de painéis; as listas dos painéis não. Execuções fora de vista não pulsam: têm o próprio status.



**Sem mola e sem flicker (2026-10-21).** Na mola, o conteúdo do painel aparecia de uma vez e só as bordas chegavam atrasadas, e o flicker era lento, pedia o aviso a cada vez que se passava por ele e atrapalhava escolher. O mantenedor preferiu tirar os dois e deixar cada momento com o tempo próprio. Os arquivos com `spring` ou `flicker` voltam ao padrão do momento.

**Animações seguem o alvo (2026-10-21).** O mantenedor viu na barra de tabs uma célula com a cor antiga quando o nome de uma tab mudava de tamanho, e o fechamento do config animava uma área que não era a do popup. As animações guardavam a posição na tela de quando começavam, e a área de um popup saía da comparação da tela antes e depois dele. Agora cada animação tem um alvo (o quadro de um painel, o título dele, o nome de uma tab, o popup) e a posição é recalculada a cada quadro; um alvo que saiu da tela deixa de ser desenhado. O popup registra o próprio retângulo ao ser desenhado (`drawPanel`), e abrir e fechar animam só ele, sem copiar a tela inteira. Só o que já fechou (um painel, uma tab, um popup) anima onde estava, com as células que tinha.

## ADR-054 — Lip Gloss no seletor de cores

**Contexto.** O seletor de cores (editor de temas e `prefix` + `P`) era uma grade de 16×16 com as 256 cores em ordem de índice: as 16 do sistema, o cubo 6×6×6 e os cinzas se misturavam, e era difícil achar uma cor. O mantenedor pediu o seletor mais organizado, ainda com todas as cores, usando o Lip Gloss.

**Decisão.**
- Dependência nova: `charm.land/lipgloss/v2` **v2.0.0**, a última versão que não pede Go 1.25 nem atualiza outras dependências; ela traz o `github.com/charmbracelet/ultraviolet` upstream como indireta, separado da cópia em `third_party/`. O Lip Gloss monta cada bloco de cores (estilo de cada célula, `JoinHorizontal`/`JoinVertical`) e a string vai para a tela com o `StyledString` da cópia em `third_party/`.
- O seletor tem três partes com título e faixa de índices: **system** (0-15, normais em cima e brilhantes embaixo), **cube** (16-231, uma fatia por vez, em triângulo) e **grays** (232-255, duas linhas de 12). Cabe nas 39 colunas da lista e rola quando falta altura.
- **O cubo em fatias:** a fatia `k` tem as cores cujos níveis de vermelho, verde e azul (0 a 5) somam `k`, de 0 (preto) a 15 (branco); juntas, as 16 fatias têm as 216 cores, cada uma uma vez. Cada fatia é desenhada como um triângulo: uma linha por nível de vermelho, o mais forte em cima, o azul crescendo para a direita, e cada linha meia célula à esquerda da de cima. Na fatia 5 as pontas são vermelho, verde e azul puros; nas claras, amarelo, magenta e ciano; no meio, hexágonos. Quanto mais perto de uma ponta, mais a cor puxa para ela. `,` e `.` escurecem e clareiam a fatia, e a seleção vai para a cor da nova fatia mais parecida em proporção de vermelho, verde e azul; uma régua ao lado do título mostra a fatia. As alternativas comparadas foram a tabela usual em blocos, cores ordenadas por matiz e claridade (sobravam buracos e tons acinzentados no meio dos vivos), uma paleta reduzida por matiz e claridade (não tinha todas as cores) e o triângulo sem deslocamento, em escada (subia reto, mas ficava torto). A ideia do triângulo é do Ryan.
- As setas andam pela posição na tela: para os lados na mesma linha, para cima e para baixo até a linha mais próxima com células, na célula mais próxima da coluna de onde se veio. Como as linhas do triângulo andam meia célula, nenhuma célula fica exatamente em cima da outra: guardar a coluna faz a seleção alternar entre as duas vizinhas e descer reto, em vez de escorregar na diagonal.
- A célula selecionada mostra `[]` em preto ou branco, o que se lê sobre ela, sem esconder a cor.

**Consequências.** O binário leva o Lip Gloss e um segundo ultraviolet. Outras telas continuam desenhando direto em células; usar o Lip Gloss nelas é uma decisão à parte.

## ADR-055 — Modo escuro e claro dos temas

**Contexto.** Os temas eram todos escuros, menos `paper` e `sakura`, e quem usa um terminal claro precisava de outro tema. O mantenedor quer o mesmo tema, por exemplo o `blood`, nas duas versões, trocando por uma chave.

**Decisão.**
- `Theme` ganha `Light`, as cores para fundo claro, ao lado de `Colors`. Só os temas embutidos têm `Light`; `Theme.In(light)` devolve o tema com as cores do modo, e um tema sem `Light` fica com as próprias nos dois.
- O config ganha `appearance.mode`, `dark` (padrão) ou `light`, com a linha `mode` em **Appearance**. Vale para todas as sessões, que continuam com o próprio tema (ADR-050).
- Tudo o que a tela desenha passa por `App.shownTheme`, que aplica o modo. O editor de temas continua lendo o tema como está no arquivo, então copiar um tema embutido copia as cores escuras.
- Cada tema embutido tem as cores claras feitas à mão, com a mesma ideia das escuras: o texto e o texto sobre o destaque com contraste de pelo menos 4,5:1, o destaque, o texto apagado e os estados com pelo menos 3:1 sobre o fundo. `paper` e `sakura`, que eram claros, passam a ter um lado escuro.

**Alternativas.** Derivar as cores claras das escuras automaticamente (inverter a claridade) evitaria fazer 18 paletas, mas os destaques saíam lavados ou sem contraste; fica para os temas do usuário, se for preciso.

**Consequências.** Os temas do usuário e as cores próprias de painel e tab (`prefixo` + `P`) não mudam com o modo; dar a eles um lado claro (tabelas `[light.*]` no arquivo e uma chave no editor) e seguir o fundo do terminal (`mode = auto`, com a consulta OSC 11 vinda do cliente) são passos seguintes.

## ADR-056 — Instalador (`noct setup`)

**Contexto.** Quem instala o noct cai na home com tudo no padrão, sem saber o que dá para mudar. O mantenedor pediu um instalador que apresente as escolhas uma vez ([instalador.md](instalador.md)).

**Decisão.**
- O instalador é um modo do `App` (`setupMode`), desenhado como popup, não um programa à parte: usa o tema, as teclas e o palco que o noct já tem. Abre no primeiro attach quando não há `config.toml` nem `sessions.json`, e por `noct setup` (`Hello.Setup`) a qualquer momento. Quem já usava o noct não o vê abrir sozinho: recebe uma vez um aviso na barra de tabs, lembrado pelo arquivo `setup-offered` na pasta de estado, e não no config, que o noct só grava quando o usuário muda algo. `make install` chama `noct setup` no fim, só num terminal.
- A primeira tela oferece manter tudo (não grava nada), começar com os padrões (grava `Default()`) ou escolher passo a passo: teclas, temas (quais ficam na escolha do config, em `appearance.themes`, o ativo, o modo e o fundo, com a tela inteira mostrando o selecionado), animações (`all`, `few`, `none`), demo, clientes de IA encontrados no PATH e binds de exemplo para programas instalados. `s` pula um passo deixando-o como estava; tudo é gravado só no resumo final.
- Os comandos de MCP marcados rodam em painéis visíveis depois que o instalador fecha; nada roda sem marcar.

**Consequências.** `appearance.themes` filtra só os temas embutidos; os do usuário e o tema em uso sempre aparecem. Um daemon de uma versão antiga ignora o `noct setup`: é preciso reiniciá-lo depois de atualizar.

## ADR-057 — Binds próprias e `'` na busca

**Contexto.** Abrir uma ferramenta como o `less` era abrir um terminal, digitar o comando, completar o arquivo e fechar o terminal na mão.

**Decisão.**
- `[[custom_binds]]` no config: `key`, `name` e exatamente um de `command`, `script` ou `flow`. A tecla é direta (precisa de `ctrl` ou `alt`, e deixa `ctrl+c`, `ctrl+d`, `ctrl+z` e `ctrl+\` para o programa) ou `prefix x`, que não pode tomar a tecla de um comando do noct. Uma bind que não se usa é ignorada e o config diz por quê.
- Um `command` roda como um script com log e nome (`bind less`); cada `{{.nome}}` vira um parâmetro `path`, que `Tab` completa no formulário como um shell. O comando e o script abrem ao lado do painel com foco e o painel fecha quando o programa sai com 0. Um flow roda como pelo inspect.
- Os parâmetros de script ganham `path: true`, com a mesma completação.
- As binds se criam, editam e apagam na seção **custom** da página das teclas e entram na busca como `bind`.
- `'` é a segunda tecla padrão da busca, porque `Alt+Space` costuma ser do sistema.

**Consequências.** Uma bind direta é vista antes do atalho direto e do programa do painel.

## ADR-058 — Demo e animações padrão

**Contexto.** Os flows são a parte mais rica do noct e a mais difícil de descobrir; e as animações vinham desligadas em quase todos os momentos.

**Decisão.**
- Os scripts `demo/*` e os flows `demo-deploy` e `demo-loops` ficam em `examples/demo/`, embutidos no binário (`examples.InstallDemo`) e copiados pelo instalador sem sobrescrever o que já existe. Só usam `printf` e `sleep`.
- Os efeitos e tempos que o mantenedor usa viram o padrão: hover fade 150, panels jitter 300, tabs jitter 300, focus none 100, move fade 200, fullscreen 01 200, popup 01 200, activity fade 600.

**Consequências.** Quem nunca gravou as animações no config passa a ver as novas. Os testes de animação fixam os efeitos para que foram escritos, à parte dos padrões.

