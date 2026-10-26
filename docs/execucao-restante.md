# Execução das pendências

Escopo solicitado em 26/10/2026: automação GitHub Actions e restante das implementações do README.

- [x] Migrador com lock, checksum, transações, adoção validada de 001/002 e testes de rollback.
- [x] Backup real criptografado e restauração em PostgreSQL descartável, executados localmente.
- [x] Secret privado do banco e chave pública de backup cadastrados no GitHub.
- [ ] Workflows CI/publicação/backup validados em execução GitHub; hooks ainda precisam ser criados pelo proprietário.
- [x] Árvore de categorias e agrupamento de obras/edições/amostras implementados e testes locais aprovados.
- [ ] Sincronização desktop/cloud, associação pelo navegador, conflitos e retomada sem envio dos arquivos.
- [x] Exclusão em duas fases com retry do Clerk, proteção do último administrador e preservação de histórico compartilhado sem nome/e-mail, testada em PostgreSQL descartável. Aviso com contato mpblima@gmail.com.
- [ ] Busca ampliada de capas, teste de autenticação/recuperação e convites Sender reais; credenciais Google/Sender dependem do proprietário.

Repositórios: web em /home/mpblima/Projetos/catalogo_livros_web (branch feature/automacao-e-sincronizacao); desktop em /home/mpblima/Projetos/catalogo_livros (master ainda sem alterações).

A chave privada age foi criada em ~/.config/catalogo-cloud/backup.agekey, fora do Git; backup inicial criptografado em backups/before-automation.dump.age (ignorado). Ferramentas locais de teste em /tmp/catalogo-ci-tools. PostgreSQL descartável catalogo-web-form-tests, porta 55438, nunca usar a URL de produção em TEST_DATABASE_URL.
