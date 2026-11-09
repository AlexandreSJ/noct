# noct — documentação

noct é uma TUI em Go que junta, num lugar só, um multiplexador de terminais, uma biblioteca de scripts com notas, um editor visual de fluxos automatizados e um servidor MCP para agentes de IA.

## Índice

| Documento | Conteúdo |
|---|---|
| [visao.md](visao.md) | Problema, público, princípios e não-objetivos |
| [arquitetura.md](arquitetura.md) | Daemon + clientes, componentes, armazenamento, protocolo |
| [tui.md](tui.md) | Workspace, tabs e painéis, views, atalhos, estética, imagens |
| [scripts.md](scripts.md) | Scripts: formato, versão, ajuda, parâmetros, notas e CLI |
| [fluxos.md](fluxos.md) | Fluxos: editor visual, se/senão, repetições, triggers e calendário |
| [writing-flows.md](writing-flows.md) | Guia prático (em inglês) para agentes escreverem fluxos e scripts em YAML |
| [mcp.md](mcp.md) | Ferramentas expostas à IA e política de segurança |
| [roadmap.md](roadmap.md) | Fases, critérios de pronto, ideias para depois, dívida técnica e riscos |
| [temas.md](temas.md) | Editor de temas, seletor de cores, tema por sessão, cores por painel e tab, o palco |
| [templates.md](templates.md) | Templates de tab: salvar com `R`, abrir com `T`, comandos com `run`/`ask`/`none` |
| [animacoes.md](animacoes.md) | Animações por momento e efeito, regras, prévia e catálogo de efeitos |
| [instalador.md](instalador.md) | `noct setup`, binds próprias e a demo |
| [codemap.md](codemap.md) | Onde fica o código de cada arquivo (em inglês) |
| [decisoes.md](decisoes.md) | Registro de decisões de arquitetura (ADRs) |

## Estado

Fases 0 a 3 implementadas e publicadas em releases (a mais recente no [CHANGELOG.md](../CHANGELOG.md)), com itens da fase 5 antecipados; falta a confirmação das fases 2 e 3 em uso real. A fase 4 (instalação de pacotes) saiu do plano (ADR-062). O que falta corrigir está em [roadmap.md](roadmap.md#dívida-técnica). Pendente: decisão sobre a vazão de saída (ADR-011 em [decisoes.md](decisoes.md)).

Regras de código e de escopo: [CONTRIBUTING.md](../CONTRIBUTING.md). Estes documentos de design estão em português.
