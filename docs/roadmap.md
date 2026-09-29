# Roadmap

Cada fase termina com algo usável no dia a dia. Nada da fase N+1 entra antes de a fase N estar pronta.

## Fase 0 — Fundação: terminais em painéis

**0a. Prova de emulação (prazo: 1–2 semanas).** O maior risco do projeto: em Go, a emulação VT é menos madura que em Rust. Esta fase responde se Go serve, e o código é escrito para ser mantido se passar: pacotes `pty`, `pane`, `layout` e `app`, tudo num processo só, ainda sem daemon.

Biblioteca VT escolhida para a prova: `github.com/charmbracelet/x/vt`, com renderização por `github.com/charmbracelet/ultraviolet` (ver ADR-008 em [decisoes.md](decisoes.md)).

Critérios, todos com a configuração do próprio usuário (zsh, oh-my-zsh, plugins e prompt como estão no `~/.zshrc`; o noct não reimplementa nada disso):

1. zsh em 2 painéis lado a lado, em 2 tabs.
2. Prompt do powerlevel10k correto, incluindo prompt da direita e ícones de Nerd Font.
3. Sugestão cinza do `zsh-autosuggestions` e menu de completar desenhados corretamente.
4. Redimensionar a janela sem quebrar o layout nem o conteúdo dos painéis.
5. `cat` de um arquivo de 100 MB num painel: a tela acompanha, e digitação e `Ctrl+C` continuam respondendo durante a saída.
6. Desejável: Claude Code, vim e htop utilizáveis num painel.

Resultado: **Go serve**, confirmado pelo mantenedor (ADR-011 em [decisoes.md](decisoes.md)); as correções de vazão das bibliotecas ficam em `third_party/` (2,5 → 7,6 MB/s). Critérios 1–4 e a resposta da entrada no critério 5 passaram; a vazão de saída (~2 MB/s) ficou abaixo do esperado por causa das bibliotecas, com causas e correções identificadas. Se falhar em 1–5 e não houver correção viável na biblioteca, reavaliar a linguagem (Rust com `alacritty_terminal` é o plano B).

**Workspace utilizável (feito junto da 0a).** Home com o corvo quando não há tabs; modo noct persistente para navegar, com popup de comandos; tabs como contêineres de painéis, que fecham com o último painel (ADR-019), preenchidas por painéis de terminal (`p`) ou de scripts (`s`); `Tab`/`Shift+Tab` e setas atravessando a borda da tab; mover painéis entre posições e tabs; renomear tabs; config (`prefixo` + `c`) com tecla prefixo, tema e fundo, salvo em `config.toml`; e o scripts panel como porta de entrada das fases 1 e 2 (ver [tui.md](tui.md)).

**Imagens nos painéis (antecipado da fase 5).** Programas que usam o protocolo gráfico do kitty, como o fastfetch, mostram imagens nos painéis quando o terminal de fora é o kitty ou o Ghostty (ADR-014).

**0b. Daemon.** Pacotes `ipc`, `daemon`, `client` e `logstore`: processos continuam rodando com a TUI fechada (`prefixo` + `d` desconecta, `noct` reconecta, `noct quit` encerra), a saída de cada terminal fica gravada em disco, e `prefixo` + `[` rola o histórico do terminal com foco (ADR-017). **Concluída.**

## Fase 1 — Scripts

A primeira base usável: uma biblioteca de scripts com notas (ver [scripts.md](scripts.md) e ADR-018).

- Adicionar, editar e remover scripts pela TUI; cada script é um arquivo YAML com um `notes.md` ao lado.
- Versão de cada programa mostrada na lista, ou `(not found)`.
- Invocações salvas com parâmetros perguntados na hora, lembrando os últimos valores.
- `prefixo + ?` mostra a ajuda do programa em primeiro plano no painel com foco.
- Busca fuzzy (`prefixo` + `Space`) sobre scripts, ações e tabs.
- `noct run`, `noct ls`, `noct check`.

**Concluída.** Além do previsto: pastas de scripts, execução em painel com código de saída, formulário por partes, busca, ajuda por programa, mouse, tiling como o i3 e painéis flutuantes.

**Pronto quando:** um teste repetitivo real da empresa vira um script com notas, roda com parâmetros diferentes pelo formulário e pode ser re-executado em uma tecla.

## Fase 2 — Fluxos

Encadear scripts num editor visual (ver [fluxos.md](fluxos.md)).

- Editor visual na TUI: fluxograma vertical com blocos de script, `se / senão`, repetição e espera.
- Repetição por intervalo (`every`), até uma condição (`until`) ou N vezes (`times`), sempre com limite de segurança.
- Capturas de saída por regex, reutilizadas nos passos seguintes.
- Triggers: manual, cron, mudança de arquivo, fim de outro fluxo; desligados até o usuário habilitar.
- Histórico e **calendário de execuções**: o mês com os dias coloridos por status; escolher um dia lista as execuções, e escolher uma mostra o log completo passo a passo, sem sair da view.

**Implementada; falta a confirmação do mantenedor pelo critério abaixo, num teste real.** Feitos: o inspect (`prefixo` + `i`) com o fluxograma montado pelas teclas, arrastável e com zoom; `script`, `run`, `if/else`, `repeat` (`times`, `until` com `max`, `every`), `wait`, capturas e `expect`; `noct flow run`; a view de logs com o calendário (`prefixo` + `v`), com limites de idade e tamanho dos logs (ADR-028, ADR-029); inserção exata pelas setas do fluxograma (ADR-030); variáveis com painel de escopo, avisos, busca ao digitar, `outputs` nos scripts, saída nos resultados e teste de trechos com `t` (ADR-031); triggers `cron`, `watch` e `after` disparados pelo daemon, adicionados pelo inspect (`T`) e habilitados por máquina (`o`, `noct enable`); `on_fail: ask`; passos em painel próprio (`pane: new`, `background`); abrir uma execução do calendário no fluxograma (`i`) e filtrar os logs (`f`) (ADR-032); params com tipo e o formulário `p`, e os passos `prompt`, `set` e `unset` (ADR-034).

**Pronto quando:** um teste de ponta a ponta com várias etapas é montado sem editar YAML à mão, roda sozinho por cron, para no passo certo em caso de falha, e a execução de ontem é encontrada pelo calendário.

## Fase 3 — IA (MCP)

**Implementada antes da confirmação da fase 2, a pedido do mantenedor (ADR-033).** `noct mcp` com todas as ferramentas de [mcp.md](mcp.md), exceto `install_program` (fase 4); política, popup do MCP (`prefixo` + `y`: aprovações, clientes conectados, comandos para adicionar o noct a cada IA), rascunhos com `P`/`noct promote` e auditoria. Falta a confirmação pelo critério abaixo.

Pacotes `mcp` e `policy` (ver [mcp.md](mcp.md)).

- A IA lista e lê scripts e fluxos com suas notas.
- A IA cria fluxos como rascunho a partir de prompts; o usuário revisa e promove.
- A IA executa e lê resultados **estruturados**: status por passo, exit codes, durações, capturas e o caminho percorrido pelos `se` e repetições, além de busca nos logs.
- View de Aprovações para ações que exigem confirmação; log de auditoria.

**Pronto quando:** o Claude, num painel, cria um fluxo como rascunho, o usuário promove, e o Claude o executa, lê o resultado estruturado e corrige um passo que falhou, sem nenhum comando arbitrário rodar sem aprovação.

## Fase 4 — Instalação de pacotes

Pacote `catalog`, adaptadores de gerenciadores e view Catálogo (ver [catalogo.md](catalogo.md)).

**Pronto quando:** numa máquina nova, abrir um fluxo com programas faltando oferece instalá-los pelo gerenciador preferido, e a instalação roda num painel. A IA também pode pedir instalação, sempre com confirmação.

## Fase 5 — Polimento

Imagens nas views do próprio noct (Kitty/iTerm2/Sixel/meios-blocos), temas, layouts salvos, notificações e os itens de qualidade de vida em [tui.md](tui.md). Itens pequenos de polimento podem entrar antes, em qualquer fase.

## Riscos

| Risco | Impacto | Mitigação |
|---|---|---|
| Emulação VT em Go insuficiente | Alto: bloqueia o projeto em Go | Fase 0a antes de tudo; plano B é Rust |
| Travamento com saída de alta vazão | Médio | Interpretar sempre, desenhar no máximo ~60 quadros/s e só o visível; contrapressão do PTY |
| Editor visual consumir o projeto | Alto | Fluxograma vertical estruturado, sem canvas livre; YAML continua sendo a fonte da verdade |
| Escopo grande demais | Alto | Cada fase entrega algo usável; regras de escopo em [CONTRIBUTING.md](../CONTRIBUTING.md) |
| Fluxos compartilhados rodando código inesperado | Médio | Triggers desligados por padrão; `noct check`; rascunhos da IA não rodam |
| Vazamento de segredos em logs ou para a IA | Alto | Segredos só por referência, mascarados, nunca enviados ao MCP |

## Questões em aberto

- Tecla prefixo padrão: `Ctrl+Space` conflita com alguns métodos de entrada; validar na fase 0a.
- Compartilhamento de scripts e fluxos entre times além do git.
