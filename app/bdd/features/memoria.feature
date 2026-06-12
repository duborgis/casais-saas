# language: pt
Funcionalidade: Memória da conversa
  Para que o casal não perca o fio da mediação
  O histórico do chat deve sobreviver ao reload da página
  E todas as mensagens de uma sessão devem chegar ao agente na mesma
  sessão de runtime do AgentCore (é isso que dá memória curta ao agente)

  Cenário: Histórico do chat persiste após recarregar a página
    Dado que eu criei uma conta nova
    E eu consigo enviar a mensagem "Estamos discutindo muito sobre a rotina"
    Quando eu recarrego a página
    Então a resposta contém "Estamos discutindo muito sobre a rotina"
    E a resposta contém "resposta simulada pelo WireMock"

  Cenário: Mensagens da mesma sessão mantêm a mesma sessão de runtime no agente
    Dado que eu criei uma conta nova
    E que o histórico de chamadas do WireMock está limpo
    Quando eu consigo enviar a mensagem "Oi Valentina"
    E eu consigo enviar a mensagem "Você lembra do que eu acabei de dizer?"
    Então o agente recebeu 2 chamadas na mesma sessão de runtime
