# Sincronização desktop e nuvem

O proprietário inicia a conexão no botão **Conta / sincronização…** do desktop, confirma no navegador o código temporário e escolhe um catálogo próprio. Conectar não envia dados. O desktop sincroniza automaticamente por padrão a cada 5 minutos; o intervalo pode ser definido entre 1 minuto e 24 horas ou mudado para **Manual**. A sincronização manual continua disponível no botão **Sincronizar catálogo**. A web atualiza alterações de outros participantes pelo intervalo escolhido no cabeçalho (15 segundos, 30 segundos, 1 minuto, 5 minutos, um intervalo próprio entre 1 minuto e 24 horas ou **Manual**), com atualização imediata ao voltar o foco do navegador quando a atualização automática está habilitada. A preferência da web fica no navegador; a do desktop, no perfil SQLite.

## Dados e segurança

Todos os livros físicos e digitais usam o mesmo modelo bibliográfico nos dois sentidos: título, subtítulo, autores, editora, edição, ano, idioma, ISBN/código, páginas totais, assunto, palavras-chave, resumo, favoritos, categorias, relações, metadados extras e anotações. Capas PNG são privadas, limitadas a512KiB/2000px, e imagens públicas mantêm a URL. Arquivos PDF/EPUB, OCR, senhas e posição/página privada do leitor permanecem no desktop.

Um livro recebido sem arquivo local fica como cadastro disponível. O desktop não tenta abrir nem movimentar um caminho de outro computador. Arquivos existentes nunca são apagados pela sincronização. Alterações concorrentes ficam em **Conflitos**, com escolha explícita da versão. Exclusões físicas locais exigem confirmação; exclusões remotas desativam o cadastro local, preservando o arquivo.

Novas anotações compartilhadas e edições do campo de notas ficam no histórico com autoria da conta verificada e origem Web/Desktop. A web e o desktop apresentam o histórico em mensagens com autor e data. Notas legadas são importadas pela conta que sincronizou; isso não comprova autoria histórica anterior. Remover um convidado revoga o acesso futuro ao catálogo, mas preserva o histórico do proprietário e não apaga cópias que o convidado já tenha obtido.

Os tokens são privados por perfil SQLite, possuem escopo de catálogo e validade de30dias. O banco do servidor guarda somente o hash. Revogar um dispositivo impede novas sincronizações, mas mantém os dados já baixados. Renovar a mesma conta/catálogo preserva os vínculos; trocar de conta exige nova associação explícita. Quando a sincronização automática está habilitada, o desktop inicia a sincronização ao abrir a biblioteca e continua no intervalo escolhido enquanto permanecer conectado.

## Provas automatizadas

`CloudSyncTest` testa aplicação local, preservação da leitura e arquivos, separação de perfis e permissões do arquivo privado. `TestDesktopClient` executa o cliente Java real contra a API Go em PostgreSQL descartável: associação, todos os campos, comentários com autoria, lote repetido após perda da resposta, conflito, tombstone, cadastro sem arquivo, categorias/relações removidas sem ressurgir e revogação/renovação. O CI fixa a revisão desktop em `.github/desktop-sync-ref`.

Nunca definir `TEST_DATABASE_URL` para o banco de produção. Os testes usam identidades fictícias apenas no código de teste; a API de produção continua verificando o Clerk.
