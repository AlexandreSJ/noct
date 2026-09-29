# Arquitetura

## Visão geral: daemon + clientes

O noct segue o modelo de tmux/Zellij: um **daemon** é dono de todos os processos, e **clientes** se conectam a ele. Isso resolve três requisitos de uma vez:

- programas continuam rodando quando a TUI fecha (desconectar/reconectar);
- tabs e painéis ilimitados são só estado no daemon;
- a IA (via MCP) e a CLI enxergam os mesmos painéis e execuções que a TUI.

```
                ┌───────────────── noct daemon ────────────────┐
 noct (TUI) ◄──►│  Session ─ Tabs ─ Panes                      │
 noct run   ◄──►│   Pane = PTY + processo + emulador VT        │
 noct mcp   ◄──►│          + scrollback + log em disco         │
 (stdio p/ IA)  │  Runner (scripts/fluxos)  · Triggers         │
                │  Catálogo · Policy (aprovações)              │
                └──────────────────────────────────────────────┘
                        socket Unix: $XDG_RUNTIME_DIR/noct/noct.sock
```

Tudo é **um único binário**: `noct` sem argumentos inicia o daemon se necessário e conecta a TUI; `noct daemon`, `noct run <script|fluxo>`, `noct ls`, `noct check`, `noct mcp` são subcomandos.

## Componentes

Cada componente é um pacote Go em `internal/`, com interface pequena e testável isoladamente. A coluna "Fase" diz quando o pacote nasce.

| Pacote | Responsabilidade | Depende de | Fase |
|---|---|---|---|
| `pty` | Iniciar processos em pseudo-terminais e redimensioná-los | `creack/pty` | 0a |
| `pane` | Um terminal: processo + PTY + emulador VT; repassa teclas e desenha a tela | `pty`, `charmbracelet/x/vt` (cópia com correções em `third_party/`, ADR-011) | 0a |
| `layout` | Árvore de splits de uma tab: dividir, remover, calcular retângulos, foco por direção | — | 0a |
| `config` | Ler e gravar `config.toml` (tecla prefixo, tema, fundo) | `BurntSushi/toml` | 0a |
| `kittygfx` | Imagens do protocolo gráfico do kitty nos painéis: separar comandos, traduzir ids, marcadores | — | 0a |
| `app` | Workspace: home, tabs de views (terminais e painéis do noct), modo noct, popups, config, loop de eventos e renderização | `pane`, `layout`, `config`, `ultraviolet` (cópia em `third_party/`) | 0a |
| `logstore` | Gravar a saída de cada terminal em disco, com rotação por tamanho; listar os logs por dia e apagar os que passam dos limites do config | — | 0b |
| `ipc` | Protocolo cliente↔daemon: mensagens com prefixo de tamanho sobre socket Unix | — | 0b |
| `daemon` | Iniciar o daemon em segundo plano quando preciso e rodá-lo servindo o `app` | `app`, `ipc`, `logstore` | 0b |
| `client` | O `noct` no terminal do usuário: modo raw, repassar teclas e tamanho, escrever o que o daemon desenha | `ipc` | 0b |
| `script` | Carregar, validar e renderizar scripts; biblioteca pessoal e do projeto; detectar versão; lembrar valores | `go.yaml.in/yaml/v3` | 1 |
| `flow` | Modelo, validação e edição de fluxos por caminho do passo; execução (passos, condições, repetições, esperas, capturas) e o resultado da última | `script`, `logstore` | 2 |
| `schedule` | Decidir quando fluxos habilitados disparam: cron, arquivos que mudam (por varredura, sem dependência) e fim de outro fluxo; quem roda é o `app`, no daemon (ADR-032) | `flow` | 2 |
| `history` | (Não criado: o histórico ficou em `runs.json`, no pacote `script`, e no último resultado de cada fluxo; ADR-029) | — | 2 |
| `policy` | Decidir se uma ação (da IA ou de trigger) roda, pede confirmação ou é negada | — | 3 |
| `mcp` | Servidor MCP por stdio (protocolo próprio, sem SDK): ferramentas sobre scripts, fluxos, execuções e logs, e pedidos ao daemon para painéis, execuções e aprovações; política e auditoria (ADR-033) | `script`, `flow`, `logstore`, `config` | 3 |
| `catalog` | Manifestos de programas, detecção e adaptadores de gerenciadores de pacote | — | 4 |

Desde a fase 0b, o `app` roda **dentro do daemon** (ADR-017), e o processo `noct` que o usuário abre é o `client`, um cliente fino.

Regra: clientes (`client`, e no futuro `mcp` e a CLI) **só** falam com o daemon via `ipc`; nunca importam `app`, `runner` ou `pane`. Isso mantém os clientes finos e garante que tudo que a TUI faz, a IA e a CLI também podem fazer.

## Renderização

O workspace é desenhado direto sobre o `ultraviolet` (a camada de células sobre a qual o Bubble Tea v2 é construído), não sobre o modelo de mensagens do Bubble Tea (ver ADR-008 em [decisoes.md](decisoes.md)):

- Cada painel lê seu PTY numa goroutine própria e alimenta o emulador VT o tempo todo.
- A saída só marca a tela como alterada; o loop principal redesenha no máximo ~60 vezes por segundo e só quando algo mudou.
- O `ultraviolet` compara o quadro novo com o anterior e envia ao terminal apenas as células que mudaram. Se o modelo dele se separa do terminal, células antigas ficam na tela até mudarem ("ghosts"). Um teste em `third_party/ultraviolet/noct_render_test.go` reproduz milhares de quadros aleatórios num emulador e compara com o esperado; para um ghost no terminal real, `NOCT_TRACE=arquivo noct` grava tudo o que o cliente escreve no terminal, para ser reproduzido depois.
- Se o noct não consegue acompanhar, ele simplesmente lê o PTY mais devagar: o kernel segura o programa (contrapressão), sem filas crescendo em memória.

Views ricas das fases seguintes (formulários, listas, editor de fluxos) podem usar componentes do ecossistema Charm, desde que desenhem no mesmo `ultraviolet`.

## Daemon e cliente

O daemon mantém tudo: o emulador VT de cada painel, as tabs e o workspace inteiro, inclusive a renderização. Motivos:

- ao reconectar, o cliente recebe a tela atual pronta, sem reprocessar histórico;
- o MCP poderá ler "o que está na tela do painel 3" sem uma TUI aberta;
- execuções de fluxo podem verificar a saída renderizada.

Como no tmux, **o daemon desenha para o terminal do cliente** (ADR-017):

```
 terminal do usuário              noct (client)                 noct daemon (app)
 ─ teclas (bytes crus) ─►  mensagem Input  ─────────────►  decodifica teclas, modo noct, painéis
 ◄─ bytes de tela ───────  mensagem Output ◄─────────────  desenha o quadro para aquele terminal
 ─ SIGWINCH ─────────────► mensagem Resize ─────────────►  redimensiona tabs e painéis
```

- O cliente abre com um `Hello`: versão do protocolo, ambiente, tamanho em células e pixels, e o perfil de cores, que só ele consegue detectar.
- Um cliente conectado por vez; conectar de outro terminal desconecta o anterior.
- `prefixo` + `d` desconecta; sem cliente, os programas continuam rodando e nada é desenhado. `noct` reconecta.
- `noct quit` fecha tudo e para o daemon. `noct` inicia o daemon sozinho quando ele não está rodando (`noct daemon`, numa sessão própria, com a saída em `daemon.log`).
- Protocolo: 1 byte de tipo + 4 bytes de tamanho + conteúdo; `Hello`, `Resize` e `Exit` em JSON, `Input` e `Output` em bytes crus.

## Armazenamento (XDG)

| Caminho | Conteúdo |
|---|---|
| `~/.config/noct/config.toml` | Configuração do usuário (tema, atalhos, gerenciadores preferidos, política MCP) |
| `~/.local/share/noct/scripts/`, `.../flows/` | Scripts e fluxos pessoais |
| `<projeto>/.noct/scripts/`, `.../flows/` | Scripts e fluxos do projeto, versionados no git e compartilhados com o time |
| `~/.local/share/noct/catalog/` | Manifestos de ferramentas adicionados pelo usuário |
| `~/.local/state/noct/logs/<data>/<hora>-pane<N>.log` | Saída crua de cada terminal, até 10 MiB + um arquivo `.1` com o anterior |
| `~/.local/state/noct/daemon.log` | Mensagens do daemon |
| `~/.local/state/noct/params.json`, `runs.json` | Últimos valores dos parâmetros; as últimas 1000 execuções de scripts (fase 1) |
| `~/.local/state/noct/runs.json` | As últimas 1000 execuções de scripts e fluxos: quando, código de saída e log (ADR-029) |
| `~/.local/state/noct/flows/<fluxo>.json` | Resultado da última execução de cada fluxo, passo a passo (ADR-028) |
| `$XDG_RUNTIME_DIR/noct/noct.sock` | Socket do daemon (permissão 0600; sem `XDG_RUNTIME_DIR`, fica em `/tmp/noct-<uid>/`) |

Itens de projeto e pessoais aparecem juntos nas listas, com o escopo indicado. Em conflito de nome, a do projeto vence.

Segredos nunca são gravados em scripts, fluxos, `params.json` nem em `runs.json`: parâmetros `secret` são mascarados na TUI e podem vir de variável de ambiente (`from: env:`). Detalhes e limites em [scripts.md](scripts.md).

## Tratamento de erros

- **Daemon cai:** processos filhos morrem com ele (como no tmux). Logs em disco sobrevivem; o cliente restaura o terminal sozinho e avisa; `noct` inicia um daemon novo.
- **Cliente cai** (janela fechada): nada acontece com os processos; `noct` reconecta.
- **Passo de fluxo falha:** o fluxo para (padrão) ou segue, conforme `on_fail` do passo; o estado fica no histórico com o log.
- **Programa ausente:** antes de rodar, o runner confere se os programas usados existem e avisa; a partir da fase 4, oferece instalar.
- **Versão de protocolo:** o cliente manda a versão no `Hello`; em divergência, o daemon recusa e manda rodar `noct quit` e abrir o noct de novo.

## Testes

- `layout`, `command`, `flow`, `policy`, `catalog`: testes unitários puros (tabelas de casos).
- `pane`: testes de integração com processos reais curtos (`printf`, `sh -c`) num PTY, conferindo a tela do emulador.
- `runner`: fluxos reais executando scripts simples (`echo`, `false`, `sleep`).
- `ipc`, `logstore`: testes unitários.
- `app`: a lógica de teclas, tabs e modos é testada com views falsas, sem processos nem terminal.
- `cmd/noct`: testes ponta a ponta rodam o binário real dentro de um painel que faz o papel do terminal, cada um com seu próprio daemon, socket e pasta de estado, e param o daemon no fim com `noct quit`.
