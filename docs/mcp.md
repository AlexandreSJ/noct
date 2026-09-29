# Servidor MCP

Fase 3 (ver [roadmap.md](roadmap.md)). Decisões no ADR-033 em [decisoes.md](decisoes.md).

## Como a IA se conecta

`noct mcp` é um servidor MCP via **stdio** (JSON-RPC 2.0, uma mensagem por linha). Qualquer cliente MCP o registra como outro servidor qualquer, inclusive clientes com modelos locais:

```json
{ "mcpServers": { "noct": { "command": "noct", "args": ["mcp"] } } }
```

No Claude Code: `claude mcp add --scope user noct -- noct mcp`. Não é preciso lembrar a sintaxe de cada cliente: `prefixo` + `y` abre o **popup do MCP**, que gera o comando (ou, para quem não tem um comando `mcp add`, o JSON e o arquivo onde ele vai) para Claude Code, Codex, Gemini CLI, VS Code, opencode e Cursor, com o caminho completo deste binário, para funcionar mesmo fora do `PATH` do cliente; `Enter` copia (OSC 52).

O popup também lista os **clientes conectados agora**: no `initialize`, o `noct mcp` abre um pedido `client` ao daemon com o `clientInfo` (nome e versão) e o mantém aberto enquanto a sessão dura; o daemon o lista até a conexão fechar, e avisa `<cliente> connected to noct` na barra de tabs. Esse pedido não inicia o daemon (sair do noct continua saindo); sem daemon, o `noct mcp` tenta de novo a cada 3 s, então um noct aberto depois vê o cliente logo.

O que lê arquivos (scripts, fluxos, execuções, logs) o `noct mcp` lê direto, como a CLI; o que precisa do workspace (painéis, execuções que o usuário vê, aprovações) ele pede ao daemon pelo socket (`Purpose: request`), iniciando o daemon se preciso. Assim a IA enxerga os mesmos scripts, fluxos, painéis e execuções que o usuário vê na TUI, e o que ela roda aparece na TUI.

O protocolo é implementado à mão, sem SDK: `initialize` (versões `2025-06-18`, `2025-03-26` e `2024-11-05`), `ping`, `tools/list` e `tools/call`. Cada ferramenta responde JSON como texto e como `structuredContent`; uma falha da ferramenta volta como resultado com `isError`, para a IA corrigir o pedido.

## Ferramentas

| Ferramenta | Faz | Política padrão |
|---|---|---|
| `list_scripts` | Scripts com descrição, programa, params (os secretos sem valor), `outputs`, rascunho ou quebrado | allow |
| `list_flows` | Fluxos com descrição, params, triggers e se estão ligados, rascunho, problema, avisos, última execução | allow |
| `get_item` | Um fluxo (YAML, problema, avisos) ou um script (YAML e `notes.md`) | allow |
| `check` | Como `noct check`: problemas e avisos de escopo | allow |
| `flow_scope` | O que os templates leem num passo, com a certeza (`set`, `this_pass`, `maybe`) e o valor da última execução | allow |
| `draft_flow` | Cria ou substitui um fluxo como **rascunho**; substituir um fluxo promovido guarda `<nome>.yaml.bak` | allow (rascunho não roda) |
| `draft_script` | Cria ou substitui um script (e `notes.md`) como **rascunho**, guardando `script.yaml.bak` | allow (rascunho não roda) |
| `set_triggers` | Liga ou desliga os triggers de um fluxo | **ask** |
| `list_runs` | Execuções por nome, status e dias, as mais novas primeiro, com o trigger que as disparou | allow |
| `get_run` | Resultado **estruturado** de uma execução (abaixo); `wait_seconds` espera ela terminar | allow |
| `search_logs` | Regex nos logs, por execução, nome ou dias, com linhas de contexto | allow |
| `read_log` | Um log paginado, sem cores | allow |
| `list_panes` / `read_pane` | Painéis abertos e o texto que um painel mostra agora | allow |
| `run` | Roda um script ou fluxo (não rascunho) num painel do workspace e devolve o `run_id`; `steps` testa só uma faixa de passos | allow |
| `run_command` | Roda uma linha de shell qualquer num painel novo | **ask** |
| `send_input` | Digita texto (e `Enter`) num painel existente | **ask** |

`install_program` fica para a fase 4, junto do catálogo.

## Resultado estruturado

`get_run` devolve dados, não só texto; o `run_id` é o caminho do log da execução:

```json
{
  "run": { "run_id": "…/logs/2026-09-28/020001-flow-deploy.log", "name": "deploy", "kind": "flow",
           "started_at": "2026-09-28T02:00:01-03:00", "status": "failed", "exit": 1, "trigger": "cron 0 2 * * *" },
  "ok": false,
  "duration_ms": 48210,
  "steps": [
    { "path": "0", "id": "build", "kind": "run", "status": "ok", "exit": 0, "duration_ms": 1200, "captures": { "version": "2.1" } },
    { "path": "1/then/0", "id": "ship", "kind": "script", "status": "failed", "exit": 1, "reason": "exit 1", "output": "… até 4 KB …" }
  ],
  "log_tail": "… últimas 50 linhas …",
  "log_ref": "…/logs/2026-09-28/020001-flow-deploy.log"
}
```

Os passos vêm na ordem em que o fluxo roda. Uma execução sem resultado guardado (anterior à fase 2, ou ainda rodando) vem sem `steps`, com uma nota. Logs completos ficam em `read_log` e `search_logs`, que só leem logs do noct.

## Política

Três níveis por ferramenta: `allow`, `ask`, `deny`, sobre o padrão de cada uma. No `config.toml`:

```toml
[mcp]
approval_seconds = 300    # quanto uma aprovação espera antes de negar

[mcp.policy]
run = "ask"
run_command = "deny"
```

Uma palavra desconhecida vale `ask`, nunca `allow`.

- **`ask`** põe o pedido nas **aprovações** do noct, com o que exatamente seria feito (a linha de comando, o texto a digitar). A barra de tabs mostra `⚑ N waiting for approval`; `prefixo` + `y` abre o popup do MCP com as aprovações no topo, `y` aprova, `n` nega. A chamada MCP espera a decisão; sem decisão em `approval_seconds`, nega. Uma chamada negada diz isso à IA.
- **Rascunhos** (`draft: true`) não rodam, nem à mão, nem por trigger, nem como passo de outro fluxo, até o usuário revisar e promover: `P` no inspect ou no scripts panel (com confirmação), ou `noct promote <nome>`. É assim que a IA cria planos sem executar nada por conta própria.
- Parâmetros `secret` nunca vão à IA: `list_scripts` não mostra os valores, e eles vêm do ambiente (`from: env:`). O que os programas imprimem nos logs vai como está.
- Toda chamada vai ao log de auditoria `~/.local/state/noct/logs/mcp.log`: hora, ferramenta, política, argumentos (até 2000 caracteres) e se deu certo.

## Fluxo típico com IA

1. O usuário abre o Claude num painel e pede: "monte um teste de ponta a ponta do pedido em staging".
2. O Claude chama `list_scripts`, `list_flows` e `get_item` para entender o que existe e ler as notas.
3. O Claude chama `draft_flow` com um fluxo novo que reutiliza scripts existentes, e `check`/`flow_scope` para conferir o que cada passo lê.
4. O usuário revisa o rascunho no inspect e promove (`P`).
5. O Claude chama `run`, acompanha com `get_run` (`wait_seconds`) e `search_logs`, testa um trecho com `run` + `steps` e ajusta o rascunho se algo falhar.
