# Automação do catálogo web

## Validação contínua

Pull requests executam testes React, compilação Vite/TypeScript, testes Go com race detector e PostgreSQL descartável, testes transacionais do migrador, verificação Go e build da imagem Docker de produção. Nenhum segredo de produção é necessário para esses testes.

A publicação em `main` depende dos três jobs. O job `production` está condicionado à variável `DEPLOY_ENABLED=true` e ao ambiente GitHub `production`. Antes de publicar, cria um backup criptografado, restaura esse backup em um PostgreSQL descartável, aplica as migrações pendentes, solicita a versão testada da API na Render, espera essa versão no healthcheck e solicita a publicação na Vercel. O pipeline não restaura dados no banco real.

## Configuração única

No repositório GitHub, Settings → Secrets and variables → Actions:

| Tipo | Nome | Origem |
|---|---|---|
| Secret | `PRODUCTION_DATABASE_URL` | Conexão privada Supabase com `sslmode=verify-full` e certificado `certs/prod-ca-2021.crt` |
| Secret | `RENDER_DEPLOY_HOOK_URL` | Render → serviço → Settings → Deploy Hook |
| Secret | `VERCEL_DEPLOY_HOOK_URL` | Vercel → projeto → Settings → Git → Deploy Hooks; branch `main` |
| Variable | `BACKUP_AGE_RECIPIENT` | Chave pública age do responsável pelo backup |
| Variable | `BACKUP_ENABLED` | `true`, depois do teste de backup/restauração |
| Variable | `DEPLOY_ENABLED` | `true`, somente depois da configuração dos dois hooks e da adoção do banco existente |

A conexão do banco já foi cadastrada como secret criptografado, e a chave pública de backup já foi cadastrada como variable. A chave privada está somente neste computador, em `~/.config/catalogo-cloud/backup.agekey`, com permissão 0600. Ela precisa ser guardada também no gerenciador de senhas do responsável; perder essa chave impede recuperar os backups. Não colocar a chave privada no repositório, no workflow ou no chat.

Criar o ambiente `production` no GitHub e restringi-lo à branch `main`. Na Render, depois de cadastrar o hook, definir Auto Deploy como Off para que somente o workflow publique. A Vercel mantém a integração Git e o hook; o build de produção aguarda a mesma revisão estar saudável na API, evitando publicar uma interface antes do backend correspondente.

Não ativar `DEPLOY_ENABLED` antes desses passos. Enquanto desativado, o resumo do workflow informa que a publicação está pendente de configuração; testes verdes não significam implantação em produção.

## Banco existente

Os arquivos 001 e 002 foram instalados manualmente. Uma única vez, depois de backup e validação da estrutura, executar em `backend` com `DATABASE_URL` configurada de forma privada:

```sh
go run ./cmd/migrate --baseline-through 002_delivery_audit_privacy.sql
```

O comando compara colunas, tipos, nulabilidade e RLS do banco existente com a estrutura original antes de registrar o histórico. Diferenças impedem a adoção automática. Migrações seguintes são aplicadas em transação. Depois, execuções normais usam `go run ./cmd/migrate`. Nunca editar uma migração que já foi aplicada: criar um novo arquivo numerado.

## Backups e recuperação

O workflow `Database backup and restore drill` cria um backup diário e pode ser iniciado manualmente. Cada execução testa a restauração em um container isolado antes de publicar somente o arquivo `.dump.age` criptografado. Os artefatos ficam disponíveis por 14 dias; isso é a janela operacional do backup, não uma política geral de retenção de todos os dados.

Para recuperar um artefato, baixar o arquivo na página da execução e, em ambiente privado:

```sh
age --decrypt -i ~/.config/catalogo-cloud/backup.agekey -o recuperacao.dump catalogo.dump.age
pg_restore --list recuperacao.dump
```

A restauração real exige escolher um banco de destino explicitamente e interromper as escritas da aplicação. Não existe restauração automática sobre o banco de produção. Para uma regressão de código, republicar a revisão anterior; migrações deste ciclo são aditivas, compatíveis com a API anterior. Não executar down migrations destrutivas automaticamente.

## Verificação

`python3 scripts/smoke.py` verifica saúde, a fronteira 401 sem sessão e o carregamento do frontend. `--revision SHA` exige que a API esteja servindo a revisão esperada. Esses checks não substituem testes de login, recuperação e entrega de e-mails com contas reais.
