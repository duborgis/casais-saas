# Guia de Mediação - Harmony SaaS

O Harmony utiliza o `dynamic-agentcore` para fornecer uma experiência de mediação inteligente, baseada em três pilares:

## 1. Memória de Longo Prazo (Long-term Memory)
O agente utiliza a ferramenta `agent_core_memory` para salvar fatos importantes sobre o casal.
- **Como funciona:** Quando o usuário envia uma mensagem, o backend repassa o `userId` (ID único do casal). O agente automaticamente recupera contextos passados e salva novas informações.
- **Exemplo:** "Nós sempre brigamos por causa da louça" -> O agente guarda esse padrão e pode referenciá-lo em sessões futuras para sugerir um plano de ação.

## 2. RAG (Busca em Base de Conhecimento)
O mediador pode ser configurado com o `KNOWLEDGE_BASE_ID` apontando para uma base de dados no Amazon Bedrock contendo:
- Livros de Comunicação Não-Violenta (CNV).
- Manuais de mediação de conflitos.
- Estudos de caso de terapia de casais.

## 3. Ferramentas (Tools)
O agente tem acesso a:
- **web_search**: Para buscar técnicas modernas de mediação ou artigos úteis.
- **calculator**: Para ajudar em mediações financeiras (ex: divisão de contas).
- **fetch_url_content**: Para ler artigos que o casal queira discutir.

## Integração Técnica
A chamada ocorre via `agent-sse-backend` (Go) que orquestra a execução no `dynamic-agentcore` (Python/Bedrock).

```javascript
// Exemplo de como a memória é ativada no payload
{
  "agentId": "mediator-agent",
  "message": "Estamos com dificuldade de organizar o tempo com as crianças.",
  "userId": "couple_unique_id_001" // A memória é atrelada a este ID
}
```
