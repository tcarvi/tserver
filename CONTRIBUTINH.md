# tserver

## Visão geral

O `tserver` é um servidor HTTP API-only escrito em Go. Ele encaminha apenas as rotas `/api` e `/api/...`; caminhos fora desse prefixo retornam `404`. O servidor não entrega páginas HTML, JavaScript, CSS nem outros assets de uma aplicação web.

O handler atual entrega arquivos estáticos da API. Para uma requisição como `/api/health.json`, o arquivo esperado é `github.com/tcarvi/api/health.json`, relativo ao diretório de trabalho do processo. Não há endpoints de negócio implementados além desse mecanismo de entrega.

As respostas da API incluem `Cache-Control: no-cache`, `Content-Language: pt-br` e o cabeçalho legado `key-Code: 00000000001`.

## Segurança HTTP

Toda requisição à API exige um JWT HS256. O servidor valida assinatura, `iss`, `exp` e `sub`; o acesso exige a permissão `api:read`, extraída de `permissions` ou da claim OAuth `scope`. As claims e as permissões validadas ficam no contexto da requisição. O servidor não implementa descoberta OIDC nem busca de chaves JWKS para tokens RS256.

`JWT_SECRET` é obrigatório e deve ter ao menos 32 bytes; `JWT_ISSUER` também é obrigatório. O processo falha no startup se esses valores estiverem ausentes ou inválidos. Não use secrets padrão, registre tokens em logs ou versione chaves.

O rate limiter token bucket conta pelo IP do peer TCP: 10 requests por segundo e burst de 20 por IP. `X-Forwarded-For` é ignorado; só confie em IP encaminhado se a aplicação estiver atrás de um proxy confiável configurado para isso. Respostas limitadas usam status `429` e `Retry-After`.

O corpo é limitado a 1 MiB por padrão usando `http.MaxBytesReader`; configure `MAX_REQUEST_BODY_BYTES` para alterar o limite. `RATE_LIMIT_RPS` e `RATE_LIMIT_BURST` ajustam o rate limit.

CORS é fechado por padrão. Configure `CORS_ALLOWED_ORIGINS` com origens exatas separadas por vírgula. Wildcards não são aceitos. O preflight é processado antes do rate limit e da autenticação, e permite somente `GET`/`HEAD` e os headers `Authorization` e `Content-Type`.

## Configuração

O servidor escuta em `:8080` por padrão. A variável `TSERVER_ADDR` permite alterar o endereço, por exemplo:

```sh
TSERVER_ADDR=127.0.0.1:8080 go120 run .
```

## Desenvolvimento local

O módulo declara Go 1.20.13. Neste ambiente, o comando `go120` seleciona essa versão sem substituir a instalação global do Go. Também é possível selecioná-la diretamente:

```sh
GOTOOLCHAIN=go1.20.13 go test ./...
GOTOOLCHAIN=go1.20.13 go vet ./...
GOTOOLCHAIN=go1.20.13 go build ./...
```

Para executar o servidor:

```sh
go120 run .
```

Defina `JWT_SECRET` (mínimo de 32 bytes) e `JWT_ISSUER` antes de iniciar. Gere o segredo com `openssl rand -hex 32` e forneça-o por um gerenciador de secrets em ambientes compartilhados. Para permitir browser CORS, configure também `CORS_ALLOWED_ORIGINS` com as origens exatas.

O diretório de trabalho precisa conter a árvore `github.com/tcarvi/api/` com os arquivos que serão servidos.

## Testes

Os testes HTTP usam `httptest` para verificar o encaminhamento da API e o retorno `404` para caminhos web e desconhecidos:

```sh
go120 test ./...
```

Antes de enviar uma alteração, formate os arquivos Go e execute os testes e o vet:

```sh
go120 fmt ./...
go120 test ./...
go120 vet ./...
```

## Docker

A imagem usa Go 1.20 na etapa de build e `bci-micro:15.5` no runtime. A compilação é estática, o processo roda como usuário não-root e o entrypoint inicia o servidor na porta `8080`. O container exige `JWT_SECRET` e `JWT_ISSUER`.

A partir da raiz deste repositório, usando o Dockerfile do projeto de container:

```sh
docker build -f /path/to/server-go/Dockerfile -t tserver:local .
docker run --rm -p 8080:8080 \
  -e JWT_SECRET="$JWT_SECRET" \
  -e JWT_ISSUER="tserver-local" \
  -e CORS_ALLOWED_ORIGINS="https://app.example.com" \
  -v "$PWD/github.com/tcarvi/api:/app/webapp/github.com/tcarvi/api:ro" \
  tserver:local
```

O volume disponibiliza os arquivos da API ao container sem incluir arquivos web. Para alterar o endereço de escuta, passe `-e TSERVER_ADDR=:9090` e publique a porta correspondente, por exemplo `-p 9090:9090`.

## Contribuições

Mantenha as alterações compatíveis com Go 1.20.13 e preserve o escopo API-only. Inclua testes para alterações em autenticação, autorização, CORS, rate limiting, limites de payload e respostas HTTP. Não registre tokens nem segredos.
