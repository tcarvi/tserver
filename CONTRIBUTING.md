# tserver

## Visão geral

O `tserver` é um gateway HTTP API-only escrito em Go. Ele recebe uma solicitação para obter resultados de testes de uma aplicação e versão, encaminha essa solicitação ao serviço Python e devolve o JSON recebido. O servidor não inicia nem acompanha jobs assíncronos, não persiste resultados e não possui modelo de domínio próprio para o conteúdo dos testes.

O contrato implementado tem um único endpoint:

| Método | Caminho | Descrição |
| --- | --- | --- |
| `POST` | `/api/test-results` | Busca e devolve os resultados de uma aplicação e versão via serviço Python. |

`api/openapi.yaml` é a especificação do contrato HTTP implementado. Não há rotas runtime para `POST /api/test-runs` ou `GET /api/test-runs/{test_id}`.

## Fluxo de uma requisição

```text
Cliente → JSON → middlewares → Handler
                         → decodifica e valida request.StartTest
                         → Service.Get
                         → Repository.FetchTestResults
                         → API Python

Cliente ← JSON ← Handler ← resultado JSON da API Python
```

1. `RequestLogging` gera um identificador aleatório, adiciona `X-Request-ID` à resposta e o coloca no contexto.
2. O rate limiter aplica o limite por IP de origem. CORS processa as origens e os preflights configurados.
3. Autenticação valida o JWT e autorização exige `api:read`. O limite do corpo é aplicado antes de o handler ler o JSON.
4. O handler aceita somente `POST` com `Content-Type: application/json`, decodifica exatamente um objeto JSON, rejeita campos desconhecidos e valida `application` e `version`.
5. O DTO de entrada `request.StartTest` é normalizado (espaços nas extremidades removidos) e validado. O handler informa aplicação e versão nos metadados do request e chama `service.Get`.
6. O serviço delega ao repositório Python. O repositório envia um `POST` JSON com aplicação, versão e, quando disponível, o mesmo `X-Request-ID`.
7. A API Python responde com JSON. O handler valida novamente o JSON e devolve o corpo sem transformação de schema, usando o status HTTP recebido do Python.

Portanto, não existe conversão para `response.User` nem passagem por um modelo de domínio. `PythonResult` é um invólucro de JSON validado, pois o formato detalhado dos resultados pertence ao serviço Python. Os DTOs `TestStarted` e `TestResults` não fazem parte do fluxo runtime atual.

## Contrato HTTP e erros

O corpo esperado é um objeto como:

```json
{"application":"inventory","version":"1.2.3"}
```

Ambos os campos são strings obrigatórias, não vazias após trim, com no máximo 128 bytes cada. Propriedades adicionais, JSON malformado ou conteúdo JSON adicional após o primeiro objeto são rejeitados. `MAX_REQUEST_BODY_BYTES` limita o tamanho total do corpo.

O handler exige `application/json` como media type, aceitando parâmetros válidos como `charset=utf-8`. O repositório Python aceita `application/json` e media types terminados em `+json`; a resposta precisa conter JSON válido e ter no máximo 4 MiB.

| Status | Origem / significado |
| --- | --- |
| `2xx`, `3xx`, `4xx` | Status e corpo JSON do Python são encaminhados. Redirects não são seguidos. |
| `400` | JSON inválido, objeto incompleto, campo ausente/vazio ou desconhecido, ou conteúdo extra. |
| `401` | Bearer token ausente, inválido ou expirado. |
| `403` | Token válido sem `api:read`, ou origem/preflight CORS não permitido. |
| `404` | Rota não encontrada. |
| `405` | Método diferente de `POST` em `/api/test-results`; a resposta inclui `Allow: POST`. |
| `413` | Corpo acima do limite configurado. |
| `415` | `Content-Type` diferente de `application/json`. |
| `429` | Limite de requisições excedido; inclui `Retry-After: 1`. |
| `502` | Falha ao alcançar o Python, resposta Python 5xx, não JSON, JSON inválido ou acima de 4 MiB. |

Os erros produzidos pelo tserver usam JSON no formato `{"error":"..."}`. Detalhes internos do upstream não são repassados ao cliente.

## Arquitetura e organização

- `cmd/api`: inicialização, injeção de dependências, composição de middlewares, configuração do servidor e encerramento gracioso após `SIGINT` ou `SIGTERM`.
- `internal/config`: carregamento e validação das variáveis de ambiente.
- `internal/dto/request`: DTOs recebidos do cliente e normalização/validação.
- `internal/dto/response`: DTOs de erro e invólucro para JSON validado do Python.
- `internal/handler`: rota, validação HTTP, chamada ao serviço e escrita de respostas JSON.
- `internal/middleware`: autenticação/autorização, CORS, rate limit, limite do corpo, logs e headers de segurança.
- `internal/service`: orquestração da consulta de resultados.
- `internal/repository/python`: cliente HTTP do serviço Python.
- `internal/pkg/jwt` e `internal/pkg/requestid`: parsing do JWT e metadados de correlação no contexto.
- `api/openapi.yaml`: especificação OpenAPI do endpoint runtime.
- `configs/example.env`: exemplo de configuração com segredo placeholder, não utilizável em produção.

Não adicione uma camada de domínio apenas para encapsular o JSON de resultados, cujo schema pertence ao Python. Se uma mudança introduzir jobs assíncronos ou persistência, atualize conjuntamente o contrato OpenAPI, handlers, serviços, repositórios, DTOs, configuração e testes; não documente endpoints que ainda não funcionam.

## Autenticação e segurança

As requisições ao endpoint exigem `Authorization: Bearer <token>`. O servidor aceita somente JWT assinado com HS256 e valida assinatura, `iss`, `exp` obrigatório e `sub` não vazio. A permissão `api:read` pode vir em `permissions` ou como item separado por espaços em `scope`. Um token sem essa permissão recebe `403`.

`JWT_SECRET` deve ter pelo menos 32 bytes e `JWT_ISSUER` deve ser informado. O servidor falha no startup se essas configurações forem inválidas. Não há descoberta OIDC nem validação JWKS para RS256.

O rate limiter usa o IP do peer TCP (`RemoteAddr`), por padrão 10 requisições por segundo com burst de 20. `X-Forwarded-For` é ignorado; ao operar atrás de proxy, configure o limite no proxy confiável ou implemente explicitamente uma política segura de IP encaminhado.

CORS é fechado por padrão. `CORS_ALLOWED_ORIGINS` aceita origens HTTP(S) exatas separadas por vírgula; wildcards, paths e query strings não são aceitos. O preflight é limitado por IP, permite somente o método `POST` e os headers `Authorization` e `Content-Type`, e não exige JWT. Configure CORS apenas quando clientes de browser precisarem acessar a API.

O servidor acrescenta `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` e `Referrer-Policy: no-referrer`. Logs JSON incluem request ID, método, caminho, status, duração, aplicação, versão e status do Python quando disponíveis. Tokens e corpos de request/response não são registrados.

O request ID é criado pelo próprio tserver; o header enviado pelo cliente não é usado como identificador. O ID gerado é propagado ao Python por `X-Request-ID` para correlação.

## Configuração

| Variável | Obrigatória | Padrão / validação | Uso |
| --- | --- | --- | --- |
| `TSERVER_ADDR` | Não | `:8080` | Endereço HTTP local. |
| `JWT_SECRET` | Sim | mínimo 32 bytes | Segredo HS256. |
| `JWT_ISSUER` | Sim | não vazio | Issuer esperado dos JWTs. |
| `PYTHON_API_URL` | Sim | URL absoluta HTTP(S) | Endpoint Python; não pode conter user info ou fragmento. |
| `CORS_ALLOWED_ORIGINS` | Não | vazio | Origens exatas separadas por vírgula. |
| `RATE_LIMIT_RPS` | Não | `10`, positivo | Requisições por segundo por IP. |
| `RATE_LIMIT_BURST` | Não | `20`, inteiro positivo | Burst por IP. |
| `MAX_REQUEST_BODY_BYTES` | Não | `1048576` (1 MiB), inteiro positivo | Tamanho máximo do request. |

O cliente Python usa timeout de 15 segundos, não segue redirects e limita a resposta a 4 MiB. O servidor HTTP usa `ReadHeaderTimeout` de 5 segundos, `ReadTimeout` de 15 segundos, `WriteTimeout` de 30 segundos e `IdleTimeout` de 60 segundos.

## Desenvolvimento local

O módulo declara Go 1.20.13. No ambiente de desenvolvimento que disponibiliza `go120`, execute os comandos na raiz do módulo:

```sh
go120 version
go120 fmt ./...
go120 test ./...
go120 vet ./...
go120 build -o /tmp/tserver-api ./cmd/api
```

Para iniciar o servidor, configure os valores obrigatórios:

```sh
export JWT_SECRET="$(openssl rand -hex 32)"
export JWT_ISSUER="tserver-local"
export PYTHON_API_URL="http://127.0.0.1:8000/api/test-results"
go120 run ./cmd/api
```

Use um token emitido por uma identidade confiável que inclua `iss`, `sub`, `exp` e `api:read`, por exemplo na claim `permissions` ou `scope`. Não use o segredo de exemplo em ambiente compartilhado e não versione credenciais.

Exemplo de chamada:

```sh
curl -i -X POST http://localhost:8080/api/test-results \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"application":"inventory","version":"1.2.3"}'
```

Os testes usam `httptest` para verificar o contrato HTTP, validação, encaminhamento ao serviço e middlewares. Inclua testes para qualquer alteração de comportamento público, especialmente rotas, autenticação, validação do request e erros do upstream.

## Docker

O Dockerfile de deploy fica no diretório `server-go` adjacente a este repositório. Ele compila com Go 1.20 para Linux e executa o binário como usuário não-root em `bci-micro:15.5`. O contexto de build precisa incluir este módulo:

```sh
docker build -f /path/to/server-go/Dockerfile \
  -t tserver:local /path/to/tserver
docker run --rm -p 8080:8080 \
  -e JWT_SECRET="$JWT_SECRET" \
  -e JWT_ISSUER="tserver-local" \
  -e PYTHON_API_URL="http://python-tests:8000/api/test-results" \
  -e CORS_ALLOWED_ORIGINS="https://app.example.com" \
  tserver:local
```

O container e o serviço Python precisam compartilhar uma rede Docker para que `PYTHON_API_URL` seja resolvido. Para alterar a porta de escuta, defina `TSERVER_ADDR` e publique a porta correspondente. Armazene `JWT_SECRET` no gerenciador de secrets do ambiente; nunca o inclua na imagem ou no repositório.

## Contribuições

Mantenha compatibilidade com Go 1.20.13, atualize `api/openapi.yaml` e este guia quando o contrato público mudar e teste o comportamento runtime correspondente. Não exponha tokens, segredos ou corpos nos logs. Evite adicionar rotas, dependências ou formatos alternativos sem necessidade e sem documentar seu contrato.
