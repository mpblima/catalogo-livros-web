import { useEffect, useRef, useState } from "react";
export function PrivacyNotice() {
  const [open, setOpen] = useState(false);
  const box = useRef<HTMLElement>(null);
  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement as HTMLElement | null;
    box.current?.querySelector<HTMLElement>("button")?.focus();
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
      if (e.key === "Tab") {
        const items = Array.from(
          box.current?.querySelectorAll<HTMLElement>("button,a[href]") || [],
        );
        if (e.shiftKey && document.activeElement === items[0]) {
          e.preventDefault();
          items.at(-1)?.focus();
        } else if (!e.shiftKey && document.activeElement === items.at(-1)) {
          e.preventDefault();
          items[0]?.focus();
        }
      }
    };
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      previous?.focus();
    };
  }, [open]);
  return (
    <>
      <button
        type="button"
        className="privacy-trigger"
        onClick={() => setOpen(true)}
      >
        Privacidade e contato
      </button>
      {open && (
        <div className="overlay">
          <section
            ref={box}
            className="dialog privacy-notice"
            role="dialog"
            aria-modal="true"
            aria-labelledby="privacy-title"
          >
            <button
              className="close"
              aria-label="Fechar privacidade"
              onClick={() => setOpen(false)}
            >
              ×
            </button>
            <h2 id="privacy-title">Seus dados na biblioteca</h2>
            <p>
              Contato do responsável pelo serviço:{" "}
              <a href="mailto:mpblima@gmail.com">mpblima@gmail.com</a>.
            </p>
            <h3>O que é registrado</h3>
            <p>
              Nome, e-mail verificado, identificador da conta, catálogos,
              metadados dos livros, capas, caminhos locais informados, anotações
              e compartilhamentos. A senha fica no Clerk. Os arquivos PDF/EPUB e
              o conteúdo de OCR não são enviados pelo catálogo.
            </p>
            <h3>Compartilhamento</h3>
            <p>
              As pessoas convidadas para um catálogo podem consultar, cadastrar,
              editar e anotar seus livros. Elas também veem os caminhos locais e
              as anotações informadas. Somente o responsável pelo catálogo
              gerencia os convidados. Evite incluir dados sensíveis.
            </p>
            <h3>Serviços utilizados</h3>
            <p>
              O serviço usa Clerk para autenticação, Vercel para a interface,
              Render para a API, Supabase para o banco e Sender para convites
              quando configurado. A busca de informações e capas consulta fontes
              bibliográficas externas. Capas externas podem gerar requisições ao
              provedor da imagem.
            </p>
            <h3>Exportar e excluir</h3>
            <p>
              No catálogo, você pode exportar seus dados ou solicitar exclusão.
              A exclusão exige confirmação administrativa: remove os catálogos
              próprios e seus convites, revoga os dispositivos conectados e
              exclui a conta de autenticação. Em catálogos de outras pessoas, o
              histórico de anotações permanece com o nome “Conta excluída”, sem
              o nome e e-mail da conta. Arquivos no seu computador são
              preservados.
            </p>
            <h3>Backups</h3>
            <p>
              A rotina de backups automáticos, quando habilitada, gera cópias
              criptografadas mantidas por até 14 dias no GitHub. Cópias manuais
              de recuperação também podem ser guardadas pelo responsável. Dados
              já removidos podem permanecer nessas cópias até expiração. Uma
              restauração exige revisão das solicitações de exclusão antes de
              reabrir o serviço.
            </p>
            <p>
              Para esclarecimentos ou solicitações sobre seus dados, use o
              contato acima. Este aviso descreve o funcionamento do serviço e
              não declara certificação de conformidade.
            </p>
            <button type="button" onClick={() => setOpen(false)}>
              Entendi
            </button>
          </section>
        </div>
      )}
    </>
  );
}
