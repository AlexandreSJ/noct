# Instalador, binds próprias e demo

**Estado:** implementado (ADR-056, ADR-057, ADR-058). Antecipado da fase 5, a pedido do mantenedor, como os outros itens de polimento.

Três peças que se ajudam: o **instalador** (`noct setup`) prepara o noct na primeira vez, as **binds próprias** levam uma tecla a um comando, script ou flow, e a **demo** mostra os flows funcionando sem tocar em nada real. O mapa de mods, no fim, avalia o que viria depois.

## Por quê

Hoje, abrir uma ferramenta de terminal num arquivo é `prefixo` + `p`, digitar o comando e o começo do arquivo, `Tab`, `Enter` e fechar o terminal na mão, deixando um log a mais. E quem instala o noct cai na home com tudo no padrão, sem saber o que dá para mudar: temas, atalho, animações, MCP, flows. O instalador apresenta essas escolhas uma vez, com prévia, e a demo ensina os flows rodando.

## Instalador

### Quando aparece

- No primeiro `noct`, quando `~/.config/noct/config.toml` não existe: o daemon abre o instalador no lugar da home.
- Para quem já usava o noct antes do instalador (há config ou sessões), uma vez, na barra de tabs: `new: noct setup picks themes, animations, a demo of flows and binds`. O aviso some na primeira tecla e não volta: o arquivo `setup-offered` na pasta de estado (`~/.local/state/noct/`) lembra que ele foi dado ou que o instalador já abriu.
- Por `noct setup`, a qualquer momento, inclusive com config já existente: anexa à sessão e abre o instalador por cima.
- No fim de `make install`, que reinstala e chama `noct setup`. `go install` sozinho não roda nada; o primeiro `noct` cobre esse caso.

É um popup do próprio workspace (um modo do `App`, como o config), não um programa à parte: usa o tema, as teclas, os campos de texto e o palco de prévia que o noct já tem.

### Primeira tela

```
  noct setup

  > keep everything as it is       (padrão se já há config)
    start with the defaults        (padrão na primeira vez)
    choose step by step
```

- **keep everything / start with the defaults** grava (ou mantém) o config e fecha. Na primeira vez, os dois gravam os padrões.
- **choose step by step** passa pelos passos. Cada passo tem `skip` (`s`), que deixa aquele item como está, e `Esc` volta ao passo anterior.

### Passos

1. **Keys.** Prefixo (`ctrl+space`, `alt+space`, ou capturar outro) e atalho direto (`alt`, `ctrl`, `ctrl+alt`, `off`).
2. **Themes.** A lista dos temas prontos e os do usuário, cada um com uma faixa das cores e o palco ao lado mostrando o selecionado ao vivo. `Space` marca quais ficam disponíveis (`appearance.themes`), `Enter` escolhe o ativo, o modo (`dark` ou `light`, ADR-055) troca com `m` e o fundo (`terminal` ou `theme`) com `b`. Os não marcados somem da escolha de tema do config; a lista vazia mostra todos. Os temas do usuário (`themes/*.toml`) aparecem sempre.
3. **Animations.** `all` (o padrão novo, abaixo), `few` (só `hover`, `popup` e `activity`, curtas) ou `none` (**Appearance → animations** em `off`).
4. **Demo.** Instala os scripts `demo/*` na pasta de scripts e os flows `demo-*` na pasta de flows. Arquivos que já existem não são sobrescritos.
5. **MCP.** Os clientes de IA encontrados no PATH (`claude`, `codex`, `gemini`, `code`), cada um com o comando que adiciona o noct. Os marcados rodam num painel visível depois que o instalador fecha; nada roda sem marcar.
6. **Shortcuts.** As binds próprias já no config e `+ create new shortcut`, que cria uma como a página das teclas faz (a tecla, depois `c`/`s`/`f` para comando, script ou flow, o que roda e o nome); `continue` segue. O noct não sugere nenhuma: um atalho para uma ferramenta só existe se o usuário o criar.

O último passo mostra o resumo do que muda; `Enter` grava, `Esc` volta.

### Animações padrão

Os efeitos e tempos que o mantenedor usa passam a ser o padrão:

| Momento | Efeito | ms |
|---|---|---|
| hover | fade | 150 |
| panels | jitter | 300 |
| tabs | jitter | 300 |
| focus | none | 100 |
| move | fade | 200 |
| fullscreen | 01 | 200 |
| popup | 01 | 200 |
| activity | fade | 600 |

`few` deixa `hover` fade 150, `popup` 01 200 e `activity` fade 600, e o resto `none`.

## Binds próprias

### O que são

Uma tecla que roda uma de três coisas:

- **command:** uma linha de shell escrita na caixa de texto, como `bat {{.file}}`. Cada `{{.nome}}` vira um campo perguntado antes de rodar, no formulário dos params, como as variáveis de um flow. Os campos de um comando são caminhos: `Tab` completa o nome do arquivo (o prefixo comum; de novo, o próximo), `↑↓` trocam de campo.
- **script:** um script da biblioteca, pelo nome (`tools/view`), com os params que ele já tem.
- **flow:** um flow, pelo nome, com os params e os `prompt` que ele já tem.

O comando e o script rodam num painel próprio que **fecha sozinho quando o programa sai com 0**; com erro, o painel fica com a saída e o `✗`, como um script comum. O flow roda como pelo inspect.

### Tecla

- Direta, como `ctrl+alt+shift+g`, tomada antes do programa do painel. Com a codificação legada, teclas que o terminal não distingue (como `ctrl+shift+g` e `ctrl+g`) avisam no config.
- Ou depois do prefixo, escrita `prefix g`, entre as teclas do modo noct que estão livres.

### Arquivo

```toml
[[custom_binds]]
key = "ctrl+alt+shift+g"
name = "view"
command = "bat {{.file}}"

[[custom_binds]]
key = "prefix m"
name = "deploy"
flow = "demo-deploy"
```

Uma bind tem exatamente um de `command`, `script` e `flow`; uma bind inválida (sem nenhum, com dois, tecla que não se lê) é ignorada e o config avisa.

### Onde se mudam

No config, página **Keys → binds**, uma seção nova no fim, **custom**: uma linha por bind (`tecla  nome  → o que roda`). `n` cria (captura a tecla, escolhe `command`/`script`/`flow`, digita o resto), `Enter` edita, `Delete` apaga. As binds também entram na busca (`prefixo` + `Space`), com o tipo `bind`.

### Busca com `'`

A busca ganha `'` como segunda tecla padrão: `prefixo` + `'` e `shortcut` + `'`. `Alt+Space` costuma ser do sistema no Windows; `Space` continua.

## Demo

Tudo fictício, só `printf` e `sleep`, para rodar em qualquer máquina.

- **`demo-deploy`** (o overview): deploy de um serviço `api` com params `env` (`staging`/`production`), `version` e `fail_health` (`no`/`yes`). Confirma com `prompt` quando é produção (`if`); `demo/build` e `demo/test` com captura por regex (testes que passaram) e `expect`; `demo/deploy` com `pane: new`; checagem de saúde com `repeat` `until` e `max`; rollback por `if` quando a saúde falha, terminando com `on_fail`; resumo com `set`.
- **`demo-loops`**: cada recurso sozinho: `times`, `every`, `until`, `set`, `unset`, `wait`.
- **Scripts `demo/*`** com `notes.md` que ensinam: onde mudar um param, como abrir o flow no inspect (`prefixo` + `i`), rodar com `p`, testar um passo com `t`, ver o log no calendário (`prefixo` + `v`), trocar uma tecla no config. `demo/start-here` diz a ordem.

Os arquivos ficam em `examples/demo/` no repositório e embutidos no binário, de onde o instalador os copia.

## Mods: o que daria e quanto custa

Pedido: instalar ferramentas de fora (as da Charm, por exemplo) para rodar dentro do noct, os próprios painéis do noct (scripts, inspect, logs) como peças, e mods de usuários, tudo chamável por bind.

| Camada | O que é | Dificuldade |
|---|---|---|
| Ferramentas de terminal | Qualquer TUI já roda num painel. Um manifesto (`name`, `bin`, `install` por gerenciador, `run`) mais a bind própria faz o "instalar e mapear". É o catálogo da fase 4 ([catalogo.md](catalogo.md)) com um `run`. | Baixa. As binds deste documento já são metade; o resto é a fase 4. |
| Painéis do noct como peças | Scripts, inspect e logs já são `panelKinds`; uma bind pode abrir um painel pelo nome (`panel = "logs"`). | Baixa. Um campo a mais na bind. |
| Mods de usuário que desenham na TUI | Código de terceiros dentro do noct. Em Go não há carregamento seguro de código (`plugin` só Linux, mesma versão de compilador, sem isolamento). O caminho viável é um mod ser **um programa à parte** que fala com o noct: desenha no próprio painel (já funciona, é um terminal) e pede coisas ao workspace pelo mesmo protocolo do MCP (`list_panes`, `run`, `send_input`), com a política e as aprovações que já existem. | Média para o protocolo (o socket do MCP já existe; falta um cliente local e um manifesto de permissões). Alta para qualquer coisa que mude o desenho do próprio noct (bordas, barra de tabs), que fica fora. |
| Distribuição | Baixar mods de um repositório, versões, atualização. | Alta, e com risco de segurança (código de terceiros rodando com o usuário). Depois da fase 4, com assinatura ou só de fontes que o usuário adiciona. |

Conclusão: é possível sem reestruturar o noct. Ferramentas e painéis cabem nas binds e no catálogo; mods são programas em painéis que usam o protocolo do MCP. O que não cabe é código de terceiros rodando dentro do daemon.

## Decisões de construção

Escolhas feitas na implementação, sem o mantenedor; cada uma pode mudar.

- **Primeira vez** é: sem `config.toml` **e** sem `sessions.json`. Quem atualiza de uma versão antiga sem nunca ter salvo o config não vê o instalador sozinho (pode rodar `noct setup`), mas ganha as animações padrão novas.
- **all / few / none** são predefinições: escrevem os efeitos e tempos de cada momento (e `animations = off` no `none`); não há um modo novo guardado.
- **keep everything** não grava nada; **start with the defaults** grava só o `config.toml` padrão. Demo, MCP e atalhos só pelo passo a passo.
- **Um cliente por vez** (ADR-017): o instalador é um modo do `App`, como o config, e grava no fim.
- **Bind `command`** roda como um script: tem log com o nome da bind (`bind view`) e entra no calendário, em vez do log anônimo de um terminal.
- **Conflitos de tecla:** uma bind direta é vista antes do atalho direto e do programa do painel; o config avisa quando ela tira uma tecla de um comando do noct. Uma bind `prefix x` com uma tecla que já é de um comando é ignorada, com aviso.
- **MCP:** os comandos são os do popup do MCP (`prefixo` + `y`), só dos clientes com linha de comando (Claude Code, Codex, Gemini CLI, VS Code), no escopo do usuário.

## Fora do escopo agora

- Baixar temas ou mods da internet.
- Desinstalar temas embutidos do binário: "instalar" é escolher quais aparecem.
- Sugerir atalhos para ferramentas ou instalá-las (é a fase 4).
