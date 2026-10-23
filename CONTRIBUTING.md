# tserver

## Visão geral

O `tserver` é um gateway HTTP API-only escrito em Go. O contrato assíncrono definido tem dois passos:

1. `POST /api/test-runs` recebe `application` e `version`; responde `202` com `test_id`, `status` e `status_url`.
2. `GET /api/test-runs/{test_id}` consulta os estados `queued`, `running`, `completed` ou `failed` e, quando concluído, o JSON dos resultados.

Os DTOs estão em `internal/dto/request` e `internal/dto/response`; a especificação está em `api/openapi.yaml`. `TestResults.Results` permanece JSON opaco porque o schema dos resultados pertence ao serviço Python. Os DTOs de resposta expõem somente metadados públicos e resultados, nunca credenciais.

**Estado da implementação:** os DTOs e a OpenAPI definem o contrato assíncrono, mas o handler ainda implementa o fluxo síncrono legado `POST /api/test-results`. A atualização das rotas e dos endpoints de jobs no Python é uma etapa separada; não assuma que as novas rotas já estão operacionais.

O request de início usa exatamente `{"application":"inventory","version":"1.2.3"}`. Os campos devem ser strings não vazias com até 128 bytes. Campos extras, JSON adicional e bodies acima de `MAX_REQUEST_BODY_BYTES` são rejeitados. Na integração síncrona atual, o upstream deve responder com JSON válido de até 4 MiB; seu status abaixo de 500 e corpo JSON são repassados sem transformação. Respostas não JSON, timeout, payload inválido e erros 5xx viram `502` genérico. O timeout é de 15 segundos e redirects não são seguidos.

No contrato assíncrono, `POST /api/test-runs` responde `202` com `test_id`, `status` e `status_url`. O cliente usa `GET /api/test-runs/{test_id}` para consultar o estado; `results` aparece quando `status` é `completed`.

## Organização do projeto

- `cmd/api`: bootstrap, injeção de dependências, servidor HTTP e encerramento gracioso.
- `internal/config`: leitura e validação das variáveis de ambiente.
- `internal/dto/request` e `internal/dto/response`: contratos de entrada e saída. O resultado de testes é encapsulado como JSON validado, sem inventar um schema de negócio que pertence ao serviço Python.
- `internal/handler`: contrato HTTP, validação do payload e serialização JSON.
- `internal/middleware`: autenticação/autorização, CORS, rate limit, limite de corpo, logs e headers de segurança.
- `internal/service`: orquestração da consulta de resultados.
- `internal/repository/python`: integração HTTP com o serviço Python. Não há banco local neste contrato.
- `internal/pkg/jwt` e `internal/pkg/requestid`: utilitários internos de token e correlação.
- `api/openapi.yaml`: especificação OpenAPI.
- `configs/example.env`: exemplo sem credenciais reais.

## Segurança HTTP

As rotas definidas exigem JWT HS256. O servidor valida assinatura, `iss`, `exp` e `sub`; o acesso exige a permissão `api:read`, extraída de `permissions` ou da claim OAuth `scope`. As claims e as permissões validadas ficam no contexto da requisição. O preflight CORS `OPTIONS` não exige JWT. O servidor não implementa descoberta OIDC nem busca de chaves JWKS para tokens RS256.

`JWT_SECRET` é obrigatório e deve ter ao menos 32 bytes; `JWT_ISSUER` também é obrigatório. O processo falha no startup se esses valores estiverem ausentes ou inválidos. Não use secrets padrão, registre tokens em logs ou versione chaves.

O rate limiter token bucket conta pelo IP do peer TCP: 10 requests por segundo e burst de 20 por IP. `X-Forwarded-For` é ignorado; só confie em IP encaminhado se a aplicação estiver atrás de um proxy confiável configurado para isso. Respostas limitadas usam status `429` e `Retry-After`.

O corpo é limitado a 1 MiB por padrão usando `http.MaxBytesReader`; configure `MAX_REQUEST_BODY_BYTES` para alterar o limite. `RATE_LIMIT_RPS` e `RATE_LIMIT_BURST` ajustam o rate limit.

CORS é fechado por padrão. Configure `CORS_ALLOWED_ORIGINS` com origens exatas separadas por vírgula. Wildcards não são aceitos. O preflight passa pelo rate limit por IP e é processado antes da autenticação; permite somente `POST` e os headers `Authorization` e `Content-Type`.

Cada request recebe `X-Request-ID`. O log estruturado registra request ID, método, path, status, duração, aplicação, versão e status do upstream quando disponível. Não registre tokens nem corpos de request/response.

## Configuração

O servidor escuta em `:8080` por padrão. A variável `TSERVER_ADDR` permite alterar o endereço, por exemplo:

```sh
TSERVER_ADDR=127.0.0.1:8080 go120 run ./cmd/api
```

`PYTHON_API_URL` é obrigatório e deve apontar para a URL HTTP(S) completa do endpoint Python. O cliente usa timeout de 15 segundos e não segue redirects.

## Desenvolvimento local

O módulo declara Go 1.20.13. Neste ambiente, o comando `go120` seleciona essa versão sem substituir a instalação global do Go. Também é possível selecioná-la diretamente:

```sh
GOTOOLCHAIN=go1.20.13 go test ./...
GOTOOLCHAIN=go1.20.13 go vet ./...
GOTOOLCHAIN=go1.20.13 go build -o /tmp/tserver-api ./cmd/api
```

Para executar o servidor:

```sh
go120 run ./cmd/api
```

Defina `JWT_SECRET` (mínimo de 32 bytes), `JWT_ISSUER` e `PYTHON_API_URL` antes de iniciar. Gere o segredo com `openssl rand -hex 32` e forneça-o por um gerenciador de secrets em ambientes compartilhados. Para permitir browser CORS, configure também `CORS_ALLOWED_ORIGINS` com as origens exatas.

O token deve ser emitido por uma identidade confiável e incluir `api:read`. A aplicação não armazena resultados: encaminha a pergunta ao serviço Python e retorna seu JSON.

## Testes

Os testes HTTP usam `httptest` para verificar o contrato POST, o JSON encaminhado e retornado, validação, erros do upstream, autenticação, CORS, rate limit e limite do corpo:

```sh
go120 test ./...
```

Antes de enviar uma alteração, formate os arquivos Go e execute os testes e o vet:

```sh
go120 fmt ./...
go120 test ./...
go120 vet ./...
go120 build -o /tmp/tserver-api ./cmd/api
```

## Docker

A imagem usa Go 1.20 na etapa de build e `bci-micro:15.5` no runtime. A compilação é estática, o processo roda como usuário não-root e o entrypoint inicia o gateway na porta `8080`. O container exige `JWT_SECRET`, `JWT_ISSUER` e `PYTHON_API_URL`.

A partir da raiz deste repositório, usando o Dockerfile do projeto de container:

```sh
docker build -f /path/to/server-go/Dockerfile -t tserver:local .
docker run --rm -p 8080:8080 \
  -e JWT_SECRET="$JWT_SECRET" \
  -e JWT_ISSUER="tserver-local" \
  -e PYTHON_API_URL="http://python-tests:8000/api/test-results" \
  -e CORS_ALLOWED_ORIGINS="https://app.example.com" \
  tserver:local
```

O container e o serviço Python precisam compartilhar uma rede Docker. Para alterar o endereço de escuta, passe `-e TSERVER_ADDR=:9090` e publique a porta correspondente, por exemplo `-p 9090:9090`.

## Contribuições

Mantenha as alterações compatíveis com Go 1.20.13 e preserve os contratos `POST /api/test-runs` e `GET /api/test-runs/{test_id}`. Inclua testes para autenticação, validação dos DTOs, estados do ciclo de execução e payload dos resultados. Não registre tokens nem segredos. Ao implementar as rotas, alinhe o contrato de jobs com o serviço Python antes de remover o handler síncrono legado.

Ao alterar o contrato, atualize este guia e o `README.md` junto com os testes. Não adicione rotas ou formatos alternativos sem definir explicitamente o contrato antes.
