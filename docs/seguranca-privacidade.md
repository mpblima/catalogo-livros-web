# Segurança e preparação de privacidade

O projeto está em desenvolvimento. Não há promessa de inviolabilidade nem certificação de conformidade LGPD. Os controles de código precisam ser acompanhados por configuração, revisão, operação e procedimentos de atendimento.

## Controles já implementados

- Todas as rotas `/api/` exigem Bearer JWT verificado pelo SDK do Clerk. O backend verifica a origem autorizada (`azp`) e consulta o usuário no Clerk para obter e-mail principal verificado e recusar contas bloqueadas/banidas.
- Identidade, autoria e permissões são calculadas no servidor. A cada requisição, a API consulta os acessos locais e rejeita contas suspensas.
- Catálogos, livros, comentários e membros são consultados pelo catálogo autorizado. Ser administrador não ignora esse isolamento.
- O esquema PostgreSQL `catalog` é privado; RLS habilitado e nenhum acesso para roles `anon`/`authenticated`. O backend utiliza conexão administrativa privada e é responsável pelo escopo das consultas. **Não confundir RLS habilitado com políticas por usuário:** não existem políticas públicas para uso direto pelo navegador. Antes de produção pública, revisar credenciais de banco, privilégios mínimos e políticas transacionais como defesa adicional.
- SQL parametrizado, limites de corpo e campos, rejeição de campos JSON desconhecidos, verificação de versões e identificadores e rejeição de URLs de capa sem HTTPS.
- Rate limit em memória de 120 requisições autenticadas/minuto por usuário, com limite de memória. Reinicia com a instância e não substitui proteção de borda nem as cotas do Clerk.
- API com timeout, `no-store`, CORS por lista explícita e `nosniff`; autenticação por header, não por cookies da API. A autorização não depende de CORS.
- TLS com validação de certificado exigido para banco não local. Senhas somente no Clerk. Segredos não entram no bundle frontend, Git ou logs de corpo.
- Alterações de permissões e inclusão de membros geram eventos de auditoria sem guardar texto das anotações. A auditoria não é imutável diante de um administrador do banco; auditoria externa/tamper-evident ainda não implementada.
- Convites persistidos em transação com a associação; fila descartada quando o acesso é removido ou após 7 dias sem entrega. Conteúdo da fila é removido após envio; erros não registram corpos de e-mail.
- Fontes hospedadas junto ao frontend. Capas externas podem gerar requisições ao provedor da imagem e usam `no-referrer`; não armazenar informações confidenciais em URLs de capa.
- Exportação de dados próprios e solicitação de exclusão disponível. A solicitação não equivale a exclusão concluída.

## Dados tratados

Identificador Clerk, e-mail, nome, catálogos, lista de convidados, metadados de livros, caminhos locais de arquivos, anotações, autor/data, eventos de administração e fila temporária de convites. Não são enviados PDFs/EPUBs, senhas, OCR local ou páginas do leitor.

Caminhos locais e anotações podem revelar dados pessoais. O catálogo compartilhado permite que seus convidados vejam esses campos. Evitar dados sensíveis em testes, e explicar o compartilhamento no aviso de privacidade.

## Pendências antes da abertura pública

- Identificar controlador, canal de atendimento e responsáveis; aprovar aviso de privacidade e termos próprios. Não inventar bases legais ou contatos de suporte.
- Confirmar finalidade, necessidade e permissões de uso de e-mails de convidados. Convites não devem inscrever pessoas em marketing.
- Documentar operadores, regiões de armazenamento e transferências internacionais (Clerk, Supabase, Render, Vercel e Sender).
- Definir retenção/expurgo de dados excluídos logicamente, auditoria e backups, e implementar a exclusão/anonimização efetiva com preservação adequada do histórico compartilhado.
- Implementar procedimento para direitos do titular, validação de identidade, prazo e confirmação de atendimento. O painel atual apenas lista os pedidos.
- Validar backup/restauração, rotação de segredos, resposta a incidentes, acesso administrativo e MFA no plano em que estiver disponível.
- Completar testes de penetração, dependências, autorização por objeto e CSP específica da instância Clerk e dos domínios usados.
- O único endpoint público é `/healthz`, que não retorna dados pessoais. Não há webhook público implementado nesta etapa. Qualquer webhook futuro do Clerk deverá validar assinatura e repetição de eventos antes de processar dados.
