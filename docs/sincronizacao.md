# Sincronização desktop e nuvem

O proprietário inicia a conexão no botão **Conta / sincronização…** do desktop, confirma no navegador o código temporário e escolhe um catálogo próprio. Conectar não envia dados. O botão **Sincronizar catálogo** envia e recebe os cadastros ativos do perfil aberto.

## Dados e segurança

Todos os livros físicos e digitais usam o mesmo modelo bibliográfico nos dois sentidos: título, subtítulo, autores, editora, edição, ano, idioma, ISBN/código, páginas totais, assunto, palavras-chave, resumo, favoritos, categorias, relações, metadados extras e anotações. Capas PNG são privadas, limitadas a512KiB/2000px, e imagens públicas mantêm a URL. Arquivos PDF/EPUB, OCR, senhas e posição/página privada do leitor permanecem no desktop.

Um livro recebido sem arquivo local fica como cadastro disponível. O desktop não tenta abrir nem movimentar um caminho de outro computador. Arquivos existentes nunca são apagados pela sincronização. Alterações concorrentes ficam em **Conflitos**, com escolha explícita da versão. Exclusões físicas locais exigem confirmação; exclusões remotas desativam o cadastro local, preservando o arquivo.

Novas anotações compartilhadas e edições do campo de notas ficam no histórico com autoria da conta verificada e origem Web/Desktop. Notas legadas são importadas pela conta que sincronizou; isso não comprova autoria histórica anterior. Remover um convidado revoga o acesso futuro; não apaga cópias que ele já tenha obtido.

Os tokens são privados por perfil SQLite, possuem escopo de catálogo e validade de30dias. O banco do servidor guarda somente o hash. Revogar um dispositivo impede novas sincronizações, mas mantém os dados já baixados. Renovar a mesma conta/catálogo preserva os vínculos; trocar de conta exige nova associação explícita. Não há sincronização automática no startup.

## Provas automatizadas

`CloudSyncTest` testa aplicação local, preservação da leitura e arquivos, separação de perfis e permissões do arquivo privado. `TestDesktopClient` executa o cliente Java real contra a API Go em PostgreSQL descartável: associação, todos os campos, comentários com autoria, lote repetido após perda da resposta, conflito, tombstone, cadastro sem arquivo, categorias/relações removidas sem ressurgir e revogação/renovação. O CI fixa a revisão desktop em `.github/desktop-sync-ref`.

Nunca definir `TEST_DATABASE_URL` para o banco de produção. Os testes usam identidades fictícias apenas no código de teste; a API de produção continua verificando o Clerk.
