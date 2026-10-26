# Execução das pendências

Escopo solicitado em 26/10/2026: automação GitHub Actions e restante das implementações do README.

- [x] Migrador com lock, checksum, transações, adoção validada de 001/002 e testes de rollback.
- [x] Backup real criptografado e restauração em PostgreSQL descartável, executados localmente.
- [x] Secret privado do banco e chave pública de backup cadastrados no GitHub.
- [ ] Workflows: primeiro CI no GitHub aprovado; ambos os hooks cadastrados, Render Auto Deploy Off. Publicação e backup agendado aguardam o teste final com as novas revisões.
- [x] Árvore de categorias e agrupamento de obras/edições/amostras implementados e testes locais aprovados.
- [x] Sincronização desktop/cloud, associação pelo navegador, conflitos e retomada sem envio dos arquivos. Protocolo real Java/Go passou em banco descartável, incluindo campos completos, autoria, remoções de categorias/relações e renovação.
- [x] Exclusão em duas fases com retry do Clerk, proteção do último administrador e preservação de histórico compartilhado sem nome/e-mail, testada em PostgreSQL descartável. Aviso com contato mpblima@gmail.com.
- [ ] Busca ampliada de capas, teste de autenticação/recuperação e convites Sender reais; credenciais Google/Sender dependem do proprietário.

Repositórios: web em /home/mpblima/Projetos/catalogo_livros_web (branch feature/automacao-e-sincronizacao); desktop em /home/mpblima/Projetos/catalogo_livros (branch feature/automacao-e-sincronizacao, commit fef64c9).

A chave privada age foi criada em ~/.config/catalogo-cloud/backup.agekey, fora do Git; backup inicial criptografado em backups/before-automation.dump.age (ignorado). Ferramentas locais de teste em /tmp/catalogo-ci-tools. PostgreSQL descartável catalogo-web-form-tests, porta 55438, nunca usar a URL de produção em TEST_DATABASE_URL.

## Protocolo de sincronização em implementação

Backend atual no branch de trabalho: `devices.go`, `sync.go`, `sync_test.go`; migração 004 serializa eventos por catálogo. TestDesktopSync passou em banco isolado: pairing com SHA-256/verificador aleatório, código, token opaco de 30 dias com escopo de catálogo; batches idempotentes, version CAS, PNG privado, anotações deduplicadas, tombstones confirmados, revogação. `pull:false` permite terminar os uploads antes de baixar eventos; padrão é pull true. Resposta: accepted, conflicts, events, cursor, has_more, categories, relations. Uploads limitados a 100 registros/2MB; paginação de eventos limitada por 100 itens e cerca de 1.5MB. Mudanças de livro precisam iniciar com beginCatalogWrite para ordenar commits/eventos.

Cliente desktop implementado em CloudSync.java e CloudDialog.java. Conta por perfil SQLite, token privado0600, lotes persistentes, sincronização manual. Leitura/página privadas e arquivos preservados; cadastro remoto sem arquivo local permanece visível e não pode abrir/mover arquivos. Conflitos têm resolução explícita; exclusões físicas locais exigem confirmação. Categorias e relações usam delta e remoções remotas não ressurgem no próximo ciclo.

Anotações via campo do exemplar também geram histórico compartilhado assinado pela conta verificada; notas legadas não comprovam autoria histórica anterior. API de comentários expõe a origem. Exclusão de conta anonimiza os campos de autoria adicionais. O frontend não oferece página/posição do leitor.

Busca ampliada em páginas de livrarias exige ISBN exato e restringe navegação ao host da loja. Testes locais: suíte Java/Swing completa,14testes React e build, Go race/vet e TestDesktopClient aprovados. CI cruzado fixa o SHA desktop em .github/desktop-sync-ref. Testes de entrega real de e-mail/Google e troca para Supabase Auth foram adiados pelo usuário.

Configuração externa atual: VERCEL_DEPLOY_HOOK_URL, RENDER_DEPLOY_HOOK_URL e PRODUCTION_DATABASE_URL confirmados; usuário confirmou Auto Deploy Off na Render. production environment criado só main. BACKUP_AGE_RECIPIENT configurado. DEPLOY_ENABLED e BACKUP_ENABLED ainda desligados. Primeiro CI do branch 60d272a passou (run40382866598). Não mesclar/main antes da implementação/testes e migração com backup. Até agora somente banco descartável recebeu migrações003/004; produção só backup/read-only, nada migrado ainda.
