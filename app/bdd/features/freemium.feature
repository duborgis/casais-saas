# language: pt
Funcionalidade: Freemium com anúncios mockados
  Para validar a disposição a pagar pelo plano Premium
  Como produto Harmonia
  Quero limitar mensagens grátis e oferecer anúncio recompensado ou assinatura

  Cenário: Conta nova começa com a quota grátis cheia
    Dado que eu criei uma conta nova
    Quando eu abro o chat
    Então eu vejo a pílula de quota "5/5 hoje"

  Cenário: Mensagem respondida pelo agente consome quota
    Dado que eu criei uma conta nova
    Quando eu envio a mensagem "Oi, Valentina"
    Então a resposta contém "resposta simulada pelo WireMock"
    E eu vejo a pílula de quota "4/5 hoje"

  Cenário: Paywall aparece quando a quota diária acaba
    Dado que eu criei uma conta nova
    E eu já enviei 5 mensagens
    Quando eu envio a mensagem "mais uma"
    Então a resposta contém "Suas mensagens de hoje acabaram"
    E a resposta contém "Assistir anúncio"
    E a resposta contém "Assinar Premium"
    E eu vejo a pílula de quota "0/5 hoje"

  Cenário: Anúncio recompensado libera mensagens extras
    Dado que eu criei uma conta nova
    E eu já enviei 5 mensagens
    Quando eu assisto um anúncio até o final
    Então a resposta contém "Obrigado por assistir"
    E eu vejo a pílula de quota "3/8 hoje"
    E eu consigo enviar a mensagem "voltei!"

  Cenário: Limite diário de anúncios é respeitado
    Dado que eu criei uma conta nova
    E eu já assisti 3 anúncios
    Quando eu tento iniciar outro anúncio
    Então a resposta contém "Assine o Premium para continuar agora"
    E a resposta não contém "Assistir anúncio"

  Cenário: Interesse no Premium é registrado
    Dado que eu criei uma conta nova
    Quando eu clico em assinar Premium
    Então a resposta contém "Obrigado pelo interesse"

  Cenário: Login com senha errada é recusado
    Dado que eu criei uma conta nova
    Quando eu tento entrar com a senha errada
    Então eu sou redirecionado para "/?error=1"

  Cenário: Métricas são invisíveis para usuários comuns
    Dado que eu criei uma conta nova
    Quando eu acesso a página "/metrics"
    Então o status da resposta é 404

  Cenário: Admin acompanha o funil em /metrics
    Dado que eu criei uma conta nova
    E eu já enviei 5 mensagens
    E eu envio a mensagem "mais uma"
    Quando eu entro como admin
    E eu acesso a página "/metrics"
    Então o status da resposta é 200
    E a resposta contém "paywall_shown"
    E a resposta contém "message_sent"
