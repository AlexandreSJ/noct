# noct — documentação

noct é uma TUI em Go que junta, num lugar só, um multiplexador de terminais, uma biblioteca de scripts com notas, um editor visual de fluxos automatizados, um servidor MCP para agentes de IA e um catálogo de programas.

## Índice

| Documento | Conteúdo |
|---|---|
| [visao.md](visao.md) | Problema, público, princípios e não-objetivos |
| [arquitetura.md](arquitetura.md) | Daemon + clientes, componentes, armazenamento, protocolo |
| [tui.md](tui.md) | Workspace, tabs e painéis, views, atalhos, estética, imagens |
| [scripts.md](scripts.md) | Scripts: formato, versão, ajuda, parâmetros, notas e CLI |
| [fluxos.md](fluxos.md) | Fluxos: editor visual, se/senão, repetições, triggers e calendário |
| [writing-flows.md](writing-flows.md) | Guia prático (em inglês) para agentes escreverem fluxos e scripts em YAML |
| [catalogo.md](catalogo.md) | Catálogo de ferramentas e adaptadores de gerenciadores de pacote |
| [mcp.md](mcp.md) | Ferramentas expostas à IA e política de segurança |
| [roadmap.md](roadmap.md) | Fases, critérios de pronto e riscos |
| [decisoes.md](decisoes.md) | Registro de decisões de arquitetura (ADRs) |

## Estado

Fases 0 e 1 concluídas; fases 2 (fluxos, triggers e calendário) e 3 (MCP) implementadas, aguardando a confirmação pelo critério de pronto (ver [roadmap.md](roadmap.md)). Pendente: decisão sobre a vazão de saída (ADR-011 em [decisoes.md](decisoes.md)).

Regras de código e de escopo: [CONTRIBUTING.md](../CONTRIBUTING.md). Estes documentos de design estão em português.
