# Catálogo de Livros Web

Aplicação web em desenvolvimento, independente do [catálogo desktop](https://github.com/mpblima/catalogo-livros).
Frontend React/TypeScript na Vercel, API Go no Render, PostgreSQL no Supabase e autenticação por e-mail/senha no Clerk. Convites de compartilhamento enviados pela API HTTPS do Sender.

**Estado:** aplicação publicada em https://biblioteca.praesens.tech, com API no Render e PostgreSQL no Supabase. O cadastro web foi alinhado aos campos do editor desktop. A sincronização bidirecional está implementada e possui testes de protocolo Java/Go com PostgreSQL descartável. A abertura dos arquivos e o acompanhamento da leitura ficam no desktop.

## Disponível no código

- Entrada e criação de conta usando os componentes do Clerk. E-mail verificado exigido pelo backend; senhas ficam no Clerk.
- Catálogo próprio criado automaticamente na primeira entrada. Acesso gratuito inicialmente liberado a novas contas.
- Cadastro separado de livros físicos e digitais, com os campos bibliográficos do editor desktop, resumo, anotações do exemplar, localização física e metadados extras em JSON.
- ISBN/código de barras em destaque: consulta automática após digitação/leitura, preenchimento de dados e capa, classificação pelas regras do desktop e revisão manual antes de salvar. Open Library e Google Books são consultados pelo backend; falhas permitem cadastro manual.
- Busca por palavras, filtros de assunto/subassunto, tipo, favoritos e leitura. Alterações dos marcadores atualizam a lista após confirmação da API.
- Caminho físico dos arquivos digitais como texto. Sem upload ou leitor de PDF/EPUB na nuvem; a posição privada do leitor permanece no desktop.
- Compartilhamento por e-mail verificado com até 50 convidados por catálogo. Contas universais e administradores não têm esse limite. Acesso compartilhado permite consultar, cadastrar, editar e anotar; somente o proprietário gerencia convidados.
- Convites numa fila persistente, com envio pelo Sender, retentativas e expiração de pendências. A inclusão só fica disponível quando o provedor de e-mail está configurado.
- Anotações compartilhadas em histórico de conversa com autor e data atribuídos pelo servidor. Revogar o compartilhamento bloqueia acesso futuro, sem apagar o histórico do catálogo.
- Administração: ativar/suspender acessos, conceder uso ilimitado e adicionar/trocar administradores. O último administrador ativo não pode ser removido. Administração não dá acesso a catálogos privados de outras pessoas.
- Exportação dos dados da conta/catálogos próprios e registro de solicitação de exclusão para análise administrativa.

## Operação e sincronização

- Árvore de categorias com descendentes e gestão de categorias físicas; agrupamento de edições/amostras sem excluir arquivos.
- Sincronização desktop ↔ nuvem de todos os campos dos livros, inclusive metadados extras, capas, categorias, relações e anotações. Arquivos digitais e estado privado do leitor não são enviados.
- Atualização automática ajustável na web (15 s, 30 s, 1 min, 5 min ou intervalo próprio de 1 min a 24 h), atualização manual e sincronização desktop configurável de 1 minuto a 24 horas ou manual. A preferência web fica neste navegador; a do desktop fica no perfil SQLite.
- Associação autorizada no navegador, token por dispositivo, revogação, lotes idempotentes, conflitos com escolha explícita e retomada após falhas.
- Anotações assinadas pelo servidor com nome, data e origem; mudanças do campo de notas também geram histórico atribuído. Remover um convidado bloqueia novos acessos ao catálogo, inclusive com ETag antigo.
- Exclusão/anonimização em duas fases com proteção do último administrador, retry do provedor e aviso de privacidade com contato público.
- CI com testes Go/React/Java integrados, imagem Docker, migrador com checksum e backup criptografado restaurado em ambiente descartável antes da publicação. Consulte [automação](docs/automacao.md) e [sincronização](docs/sincronizacao.md).

### Configuração externa adiada pelo proprietário

Clerk permanece como provedor de identidade. Troca para Supabase Auth, SMTP Sender e credenciais Google ficam para outra etapa. Entrega real de convites/recuperação e fluxos Google precisam de configuração e validação com contas reais; testes automatizados não comprovam a entrega de e-mail.

## Estrutura

```text
frontend/                React, TypeScript, Vite, Clerk
backend/                 API Go, Clerk SDK, PostgreSQL via pgx
supabase/migrations/     esquema privado e migrações SQL
render.yaml              configuração do serviço gratuito
.github/workflows/       validação automática
```

## Desenvolvimento local

Requisitos: Node 22.12+ e Go conforme `backend/go.mod`, PostgreSQL 17 e uma aplicação Clerk de desenvolvimento. Use dados fictícios nesta etapa.

1. Crie um PostgreSQL de teste. Para um Postgres comum, crie as roles vazias `anon` e `authenticated` antes das migrações; no Supabase elas já existem.
2. Aplique todos os arquivos de `supabase/migrations/`, em ordem, com um usuário proprietário do esquema.
3. Copie `backend/.env.example` para `backend/.env` e `frontend/.env.example` para `frontend/.env.local`. Preencha localmente as chaves e URLs; não publique segredos.
4. Exporte as variáveis do backend no terminal e execute `go run ./cmd/api` dentro de `backend/`. A aplicação não carrega arquivos `.env` automaticamente.
5. Dentro de `frontend/`, execute `npm ci` e `npm run dev`.

Sem `RESEND_API_KEY` e `RESEND_FROM_EMAIL`, o catálogo individual funciona, mas a inclusão de convidados retorna uma mensagem de configuração pendente.

## Validação

```sh
cd frontend
npm ci
npm test
npm run build
```

Dentro de `backend/`:

```sh
go test ./...
go vet ./...
# Use exclusivamente um banco de teste descartável: o teste limpa o esquema catalog.
TEST_DATABASE_URL='postgresql://USER:PASSWORD@localhost:5432/testdb?sslmode=disable' go test -race ./...
```

Os testes de backend substituem apenas a autenticação externa por identidades de teste. As verificações SQL de permissão, concorrência e autoria são exercitadas no PostgreSQL real. O teste do Sender usa servidor HTTP local e não envia mensagens reais. O login Clerk real e a entrega em caixa postal dependem da configuração dos serviços.

## Implantação e segurança

- [Configuração guiada](docs/implantacao.md)
- [Regras e fronteiras de dados](docs/arquitetura.md)
- [Segurança e preparação de privacidade](docs/seguranca-privacidade.md)

Livros, SQLite pessoal, configurações locais e credenciais não são versionados. As chaves privadas ficam no backend; o frontend recebe somente a chave publicável do Clerk e a URL da API.
