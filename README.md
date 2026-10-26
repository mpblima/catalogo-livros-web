# Catálogo de Livros Web

Aplicação web em desenvolvimento, independente do [catálogo desktop](https://github.com/mpblima/catalogo-livros).
Frontend React/TypeScript na Vercel, API Go no Render, PostgreSQL no Supabase e autenticação por e-mail/senha no Clerk. Convites de compartilhamento enviados pela API HTTPS do Sender.

**Estado:** primeira etapa implementada e testada localmente. Nenhum serviço cloud foi configurado ou publicado por este repositório. A paridade com o desktop e a sincronização ainda não estão concluídas.

## Disponível no código

- Entrada e criação de conta usando os componentes do Clerk. E-mail verificado exigido pelo backend; senhas ficam no Clerk.
- Catálogo próprio criado automaticamente na primeira entrada. Acesso gratuito inicialmente liberado a novas contas.
- Cadastro, edição e exclusão lógica de livros físicos e digitais; metadados, resumo, categoria, localização e URL HTTPS de capa.
- Busca por palavras, filtros de assunto/subassunto, tipo, favoritos e leitura. Alterações dos marcadores atualizam a lista após confirmação da API.
- Caminho físico dos arquivos digitais como texto. Nenhum upload ou leitor de PDF/EPUB, nem acompanhamento de páginas na nuvem.
- Compartilhamento por e-mail verificado com até 50 convidados por catálogo. Contas universais e administradores não têm esse limite. Acesso compartilhado permite consultar, cadastrar, editar e anotar; somente o proprietário gerencia convidados.
- Convites numa fila persistente, com envio pelo Sender, retentativas e expiração de pendências. A inclusão só fica disponível quando o provedor de e-mail está configurado.
- Anotações compartilhadas com autor e data atribuídos pelo servidor. Anotações são acrescentadas, não sobrescritas coletivamente.
- Administração: ativar/suspender acessos, conceder uso ilimitado e adicionar/trocar administradores. O último administrador ativo não pode ser removido. Administração não dá acesso a catálogos privados de outras pessoas.
- Exportação dos dados da conta/catálogos próprios e registro de solicitação de exclusão para análise administrativa.

## Próximas etapas ainda pendentes

1. Configurar e validar Clerk, Supabase, Render, Vercel, Sender e DNS com as contas do proprietário.
2. Validar login real, criação de senha, recuperação de acesso e entrega de convites entre duas contas.
3. Completar recursos do desktop: consulta ISBN/código de barras, classificação, gestão de árvore de categorias, capas automáticas e agrupamento de edições/amostras.
4. Implementar sincronização bidirecional, associação de conta e botão no desktop, incluindo conflitos e retomada após falhas. A tabela `sync_links` é uma reserva de modelo; não existe endpoint de sincronização implementado.
5. Completar operação de exclusão/anonimização, retenção, aviso de privacidade, backups e restauração antes de abrir ao público.

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

Sem `SENDER_API_KEY` e `SENDER_FROM_EMAIL`, o catálogo individual funciona, mas a inclusão de convidados retorna uma mensagem de configuração pendente.

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
