# Configuração guiada — primeira publicação de teste

Domínio existente: `praesens.tech`. Endereço planejado do catálogo: `biblioteca.praesens.tech`. Não alterar o site principal. Remetente proposto: `convites@praesens.tech`.

Cada etapa tem uma condição de saída. Não inserir livros pessoais nem sincronizar o desktop antes dos testes de isolamento com dados fictícios.

## 1. Preparar as contas gratuitas

Confirme acesso às contas Vercel Hobby, Supabase Free, Render Free, Clerk Hobby e Sender Free. Confirme também o login na Whois.com e onde os nameservers do domínio apontam. O DNS pode ser operado em outro provedor; não trocar nameservers automaticamente.

No painel Whois, verificar o status atual da confirmação do e-mail do titular. Os lembretes antigos não provam que exista pendência hoje.

Saída: acesso aos painéis, sem ativar upgrades ou inserir cartão para contratar recursos pagos.

## 2. Clerk — e-mail e senha

Crie a aplicação `Catálogo de Livros` em uma instância de desenvolvimento inicialmente. Habilite e-mail, verificação de e-mail e senha; Google pode ser habilitado como opção adicional. Não exigir organizações Clerk para compartilhar catálogos: os convidados são gerenciados pela API própria.

Copie a chave publicável para `frontend/.env.local` (`VITE_CLERK_PUBLISHABLE_KEY`) e a chave secreta para `backend/.env` (`CLERK_SECRET_KEY`). Não enviar chaves privadas por e-mail/chat nem colocá-las no Git.

Para produção, criar a instância de produção vinculada a `biblioteca.praesens.tech` e adicionar **somente os registros DNS que o Clerk mostrar**. As chaves de desenvolvimento e produção são diferentes; usuários não migram automaticamente entre instâncias. Criar a conta real do administrador somente no ambiente definitivo, após os testes.

E-mails de verificação e recuperação são enviados pelo Clerk nesta primeira integração. Não desabilitar “Delivered by Clerk”: o projeto ainda não implementa retransmissão desses e-mails pelo Sender.

Saída: cadastro e recuperação de senha funcionando com uma conta de teste verificada.

## 3. Supabase — banco

Criar projeto no plano Free, guardar a senha do banco em um gerenciador de senhas e escolher uma região próxima do backend. No SQL Editor, executar cada migração da pasta `supabase/migrations/` na ordem numérica, uma vez.

No botão Connect, obter a conexão PostgreSQL pelo pooler em modo de sessão. Configurar `DATABASE_URL` apenas no Render/backend, com `sslmode=verify-full`. Caso a cadeia de certificados exija o certificado CA fornecido pelo Supabase, configurá-lo no ambiente; não desabilitar validação TLS.

O esquema `catalog` é privado. Não adicioná-lo aos schemas expostos pela API pública do Supabase. Não colocar credenciais Postgres ou chave service-role no frontend. A autorização de cada catálogo acontece no Go; as roles públicas não têm acesso ao esquema.

Saída: migrações aplicadas e conexão TLS verificada pelo backend.

## 4. Sender — convites

Criar conta Free e cadastrar o domínio remetente. Publicar SPF/DKIM/DMARC exatamente conforme o Sender indicar. Preservar os registros existentes: não criar dois SPF concorrentes para o mesmo hostname.

Criar token de API transacional. Configurar no Render:

```text
SENDER_API_KEY=<token privado>
SENDER_FROM_EMAIL=convites@praesens.tech
```

O envio usa HTTPS, não SMTP, porque o Render Free bloqueia as portas SMTP usuais. O domínio verificado permite envio; isso não cria uma caixa postal para receber respostas. Confirmar um contato de suporte válido antes da publicação pública.

Saída: convite solicitado pela interface chega ao destinatário de teste. Verificar falha/reenvio também. A fila oferece entrega pelo menos uma vez; uma interrupção após aceitação pelo provedor pode produzir convite duplicado.

## 5. Render — API Go

Conectar este repositório e usar o Blueprint `render.yaml`, mantendo o serviço no plano Free. O build usa `backend/Dockerfile`. Preencher as variáveis privadas no painel:

```text
DATABASE_URL=<conexão Supabase com TLS verificado>
CLERK_SECRET_KEY=<chave da mesma instância usada pelo frontend>
BOOTSTRAP_ADMIN_EMAIL=<e-mail confirmado do administrador inicial>
AUTO_ACTIVATE_ACCOUNTS=true
FRONTEND_URL=https://biblioteca.praesens.tech
ALLOWED_ORIGINS=https://biblioteca.praesens.tech
SENDER_API_KEY=<token privado>
SENDER_FROM_EMAIL=convites@praesens.tech
```

Durante o desenvolvimento, pode incluir `http://localhost:5173` em `ALLOWED_ORIGINS`, separado por vírgula. Remover na implantação definitiva. Não usar `*`. A URL padrão `onrender.com` funciona para a API; um subdomínio próprio da API não é necessário.

`BOOTSTRAP_ADMIN_EMAIL` promove o primeiro administrador somente uma vez, depois da autenticação e verificação do e-mail. Depois, novos administradores são gerenciados no painel. Não altera dados do desktop.

Saída: `/healthz` retorna `ok` e uma chamada sem sessão para `/api/catalogs` retorna 401. Nenhum dado pessoal deve aparecer no healthcheck.

## 6. Vercel — frontend

Importar o repositório como projeto Vite, com Root Directory `frontend`, comando `npm run build` e saída `dist`. Configurar:

```text
VITE_CLERK_PUBLISHABLE_KEY=<chave publicável da instância correta>
VITE_API_URL=https://<nome-real-do-serviço>.onrender.com
```

Adicionar `biblioteca.praesens.tech` aos domínios do projeto e publicar o registro DNS exibido pela Vercel. Não substituir os registros de `praesens.tech` nem inventar destinos antes de consultar o painel.

Saída: HTTPS válido, cadastro/login funcionando e catálogo do administrador criado. Não há banco nem senha de banco no navegador.

## 7. Aceitação antes do acervo real

Com duas contas de teste:

1. Conta A não consegue consultar nem alterar o catálogo de B, mesmo mudando IDs nas URLs.
2. Adicionar B ao catálogo de A envia convite; B entra com o mesmo e-mail verificado e vê apenas o catálogo compartilhado.
3. As anotações mostram o autor correto; enviar outro autor no JSON é rejeitado.
4. Desmarcar favorito/leitura remove imediatamente o livro do filtro após a confirmação da API.
5. Remover B revoga seu acesso. Cadastro individual continua independente.
6. Uma edição concorrente retorna conflito sem sobrescrever a versão mais nova.
7. Conta suspensa não consegue operar. Administrador não obtém acesso automático a catálogos privados.
8. Conferir exportação e o registro de solicitações de privacidade.

A etapa de sincronização desktop só começa após esses testes. Ainda não há botão cloud instalado no aplicativo.

## Limites gratuitos e cuidados operacionais

Não existem limites artificiais de quantidade de livros nesta primeira versão. “Ilimitado” é uma permissão da aplicação; não elimina as cotas físicas dos provedores.

- Render Free entra em repouso após inatividade e pode demorar ao primeiro acesso. Banco e fila ficam no Supabase, não em disco do Render. [Documentação](https://render.com/docs/free)
- Supabase Free tem cotas de armazenamento/tráfego e pode pausar projetos de baixa atividade. Não enviar PDF/EPUB para reduzir armazenamento e exposição. [Cotas](https://supabase.com/docs/guides/platform/billing-on-supabase) e [pausa](https://supabase.com/docs/guides/platform/free-project-pausing).
- Clerk Development não serve como ambiente definitivo e tem limites próprios. Produção requer domínio controlado. [Ambientes](https://clerk.com/docs/guides/development/managing-environments) e [produção](https://clerk.com/docs/guides/development/deployment/production).
- Sender Free inclui envio transacional, sujeito às cotas vigentes. [Plano gratuito](https://www.sender.net/pricing/free-plan/).
- Vercel Hobby destina-se a uso pessoal não comercial. Antes de monetizar ou operar comercialmente, revisar o plano ou escolher hospedagem compatível. [Condições do Hobby](https://vercel.com/docs/plans/hobby).

Não contratar complementos pagos automaticamente. Exportações/backup, restauração e retenção precisam de validação operacional antes de produção pública.
