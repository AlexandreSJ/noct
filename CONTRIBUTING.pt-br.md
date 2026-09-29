# Contribuindo com o noct

[English](CONTRIBUTING.md)

Estas regras valem para todos que mudam este repositório, sejam pessoas ou agentes de IA. Leia antes de escrever código.

## 1. Regras de código

### Clareza em primeiro lugar

- O código deve ser fácil de ler para quem nunca o viu. Se o revisor precisa de explicação, reescreva.
- Os nomes carregam o contexto. Quem lê deve entender para que serve uma função, tipo ou variável sem abrir o corpo: `focusedPane`, `splitDirectionFor`, `ptyOutputBuffer`, não `fp`, `dir`, `buf2`.
- Prefira código simples e direto a código esperto. Nada de reflection, generics ou channels onde uma chamada de função comum resolve.
- Uma função, um objetivo. Se o nome da função precisa de "e", divida.

### Limites de complexidade

- Mantenha a complexidade cognitiva de cada função em até 15 (padrão do SonarQube). Aninhamento profundo, cadeias longas de `if/else` e laços dentro de laços são as causas comuns; use retornos antecipados e funções auxiliares pequenas.
- Uma função com mais de ~40 linhas é um sinal para dividir, não uma regra rígida.
- Cada arquivo tem um assunto claro. Um arquivo que não para de crescer está fazendo coisas demais.

### Comentários

- O melhor código dispensa comentários: funções e tipos simples não têm nenhum.
- Quando um comentário for necessário, escreva uma linha curta sobre o **objetivo** ou o **motivo**, não sobre o que cada linha faz.
  - Bom: `// Redraw at most once per frame so heavy output cannot starve input.`
  - Ruim: `// Loop over the panes` em cima de um laço sobre os painéis.
- Funções que orquestram muita coisa (muitas chamadas, ramificações e laços, perto de uma "brain function") podem ter um ou dois comentários curtos explicando o objetivo e as decisões que não são óbvias.
- Todo pacote tem um comentário de pacote de uma linha dizendo para que serve; é assim que agentes e pessoas se orientam no código.
- Nada de código comentado, nem `TODO` sem uma issue ou fase do roadmap ao lado.

### Práticas de Go

- `gofmt` e `go vet` limpos. `go test -race ./...` passando.
- Erros com contexto: `fmt.Errorf("starting shell: %w", err)`. Nunca ignore um erro em silêncio; se for seguro ignorar, diga por quê numa linha.
- Nada de `panic` fora do `main` em situações recuperáveis.
- Toda goroutine tem um dono e uma forma clara de parar. Nenhum vazamento de goroutine.
- Nenhum estado mutável no nível do pacote.
- Interfaces são definidas onde são consumidas, e só quando há duas implementações ou um ponto real de teste (por exemplo, em volta do PTY).
- Biblioteca padrão primeiro.

### Testes

- Teste comportamento, não detalhes internos. Testes em tabela para lógica pura.
- Toda correção de bug vem com um teste que falha sem a correção.
- Testes que iniciam processos usam comandos curtos e determinísticos (`printf`, `sh -c`), nunca a configuração do shell do usuário.

### Revise o próprio trabalho

Depois de implementar, antes de dar como pronto, releia a mudança inteira e confira:

1. Faz exatamente o que foi pedido, nada além?
2. Há falhas de lógica: entradas vazias, tamanhos zero, processos encerrados, acesso concorrente, erros em todos os caminhos?
3. Alguém novo entenderia todos os nomes sem explicação?
4. Algum comentário explica *o que* em vez de *por quê*? Remova, ou renomeie as coisas.
5. Entrou alguma dependência, opção ou abstração que ainda não é necessária? Remova.
6. `gofmt`, `go vet` e `go test -race ./...` passam?

## 2. Regras de escopo

O noct continua pequeno e focado seguindo estas regras:

- **Toda mudança corresponde ao roadmap.** O trabalho pertence à fase atual em [docs/roadmap.md](docs/roadmap.md). Nada de uma fase posterior entra antes, mesmo que seja "só uma coisinha".
- **Delegar, não reimplementar.** O noct orquestra o shell e os programas do usuário. Ele não reimplementa recursos do shell, autocompletar, gerenciadores de pacote ou editores.
- **Nada de código especulativo.** Nenhuma abstração, opção ou ponto de extensão para necessidades que ainda não existem. Adicione quando o segundo caso de uso aparecer.
- **Toda dependência é justificada.** Um módulo novo no `go.mod` precisa de um motivo no pull request: o que ele faz que a biblioteca padrão ou uma dependência existente não faz.
- **Nenhuma configuração sem necessidade real.** Os padrões devem bastar; cada opção é um custo de manutenção.
- **Arquivos são a fonte da verdade.** Scripts, fluxos e config são arquivos legíveis. A TUI, a CLI e a IA leem e escrevem o mesmo formato.
- **Teclado primeiro.** Todo recurso é totalmente utilizável pelo teclado.
- **Código morto é apagado.** Código sem uso é removido, não guardado "por via das dúvidas".
- **Decisões são registradas.** Uma mudança que altera uma decisão de design atualiza [docs/decisoes.md](docs/decisoes.md) na mesma mudança.

## 3. Idioma

- Código, identificadores, comentários, mensagens de commit e documentos para usuários (README, este arquivo) ficam em inglês, com tradução pt-BR dos documentos para usuários.
- Os documentos de design em [docs/](docs/) estão, por enquanto, em português.

## 4. Commits

- Uma mudança lógica por commit, com assunto curto no imperativo (`Add pane resize`, `Fix focus after closing a pane`).
- Build e testes verdes em todo commit.
