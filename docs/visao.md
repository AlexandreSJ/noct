# Visão

## Problema

Testes manuais e semiautomatizados numa empresa costumam ser repetitivos, mas não idênticos: muda o ambiente, o usuário, o dado de entrada, a ordem dos passos. Um script fixo não serve, e reescrever comandos à mão toda vez é lento e propenso a erro. O conhecimento de *como* testar fica espalhado em histórico de shell, arquivos soltos e memória.

Ao mesmo tempo, o fluxo de trabalho real envolve vários terminais abertos: um processo rodando, um log sendo acompanhado, um shell livre, e cada vez mais um agente de IA (Claude) ajudando a escrever e depurar.

## Proposta

`noct` é um binário único que, ao ser executado, abre um workspace de terminais onde:

1. **Scripts** — linhas de comando salvas, com versão do programa, ajuda, parâmetros perguntados na hora e notas explicando o que e como rodar — são executados em painéis próprios.
2. **Fluxos**, montados num editor visual, encadeiam scripts com `se/senão`, repetições, esperas, capturas de saída e triggers; cada execução fica num calendário de logs.
3. **Qualquer coisa** pode rodar em paralelo em tabs e painéis ilimitados, inclusive programas TUI completos (Claude, vim, htop).
4. Um **catálogo** mostra que ferramentas estão instaladas e instala as que faltam pelo gerenciador de pacotes que o usuário preferir.
5. Um **servidor MCP** deixa uma IA listar, criar, executar e avaliar fluxos, com o usuário no controle.

É como se tmux, just e um caderno de testes fossem uma coisa só, pensada para quem testa.

## Público

Primeiro: quem faz testes (QA, dev testando o próprio trabalho) em Linux/macOS, confortável no terminal, que quer repetir processos com flexibilidade. Segundo: times que compartilham scripts e fluxos via git.

## Princípios

- **Leve e portátil.** Um binário estático, sem runtime. Inicia rápido.
- **Arquivo como fonte da verdade.** Scripts, fluxos e config são arquivos legíveis e versionáveis; o editor visual e a IA leem e escrevem o mesmo formato.
- **Delegar, não reinventar.** O noct orquestra ferramentas existentes (shell, gerenciadores de pacote, programas do usuário); não as substitui.
- **O usuário no controle.** Nada que a IA propõe roda sem política explícita; segredos nunca ficam em texto puro.
- **Estética é requisito.** Tema coeso, animações discretas, identidade própria (o corvo da home, em `internal/app/raven2.ascii`).
- **Qualidade de vida acumulada.** Muitos detalhes pequenos (paleta de comandos, notificações, re-executar com os mesmos parâmetros) valem mais que uma feature grande.

## Não-objetivos (v1)

- Não é um gerenciador de pacotes: delega para apt/pacman/paru/brew/go/cargo.
- Não é um servidor de CI nem roda em cluster; é local, de um usuário.
- Não tem canvas livre de nós (estilo n8n): o editor de fluxos é estruturado.
- Não suporta Windows na v1 (PTYs funcionam diferente; ver [decisoes.md](decisoes.md)).
- Não é multiusuário nem acessível pela rede.
