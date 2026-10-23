# tserver

## Visão geral

O `tserver` é um servidor HTTP API-only escrito em Go. Ele encaminha apenas as rotas `/api` e `/api/...`; caminhos fora desse prefixo retornam `404`. O servidor não entrega páginas HTML, JavaScript, CSS nem outros assets de uma aplicação web.

O handler atual entrega arquivos estáticos da API. Para uma requisição como `/api/health.json`, o arquivo esperado é `github.com/tcarvi/api/health.json`, relativo ao diretório de trabalho do processo. Não há endpoints de negócio implementados além desse mecanismo de entrega.

As respostas da API incluem `Cache-Control: no-cache`, `Content-Language: pt-br` e o cabeçalho legado `key-Code: 00000000001`.

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

A imagem usa Go 1.20 na etapa de build e `bci-micro:15.5` no runtime. A compilação é estática e o entrypoint inicia o servidor na porta `8080`.

A partir da raiz deste repositório, usando o Dockerfile do projeto de container:

```sh
docker build -f /path/to/server-go/Dockerfile -t tserver:local .
docker run --rm -p 8080:8080 \
  -v "$PWD/github.com/tcarvi/api:/app/webapp/github.com/tcarvi/api:ro" \
  tserver:local
```

O volume disponibiliza os arquivos da API ao container sem incluir arquivos web. Para alterar o endereço de escuta, passe `-e TSERVER_ADDR=:9090` e publique a porta correspondente, por exemplo `-p 9090:9090`.

## Contribuições

Mantenha as alterações compatíveis com Go 1.20.13, preserve o escopo API-only e inclua testes para mudanças no roteamento ou nas respostas HTTP. Não adicione dependências externas sem necessidade; o servidor usa atualmente apenas a biblioteca padrão do Go.
