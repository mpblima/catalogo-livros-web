import { useState, type FormEvent } from "react";
import { BookCover } from "./BookCover";
import type { Book } from "./book";

type Props = {
  book: Book;
  catalog: string;
  mode: "register" | "request";
  busy: boolean;
  error: string;
  onClose: () => void;
  onSubmit: (values: Record<string, string>) => void;
};

export function LoanDialog({ book, catalog, mode, busy, error, onClose, onSubmit }: Props) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [note, setNote] = useState("");
  const digital = book.kind === "digital";
  const registering = mode === "register";

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit(registering ? { recipient_name: name.trim(), recipient_email: email.trim() } : { note: note.trim() });
  }

  return (
    <div className="overlay loan-overlay" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
      <section className="dialog loan-dialog" role="dialog" aria-modal="true" aria-labelledby="loan-dialog-title">
        <button className="close" type="button" aria-label="Fechar" disabled={busy} onClick={onClose}>×</button>
        <header className="loan-dialog-header">
          <span className="loan-dialog-icon" aria-hidden="true">{digital ? "↗" : "⇄"}</span>
          <p className="eyebrow">{registering ? (digital ? "ENVIO DIGITAL" : "EMPRÉSTIMO FÍSICO") : "PEDIDO AO RESPONSÁVEL"}</p>
          <h2 id="loan-dialog-title">{registering ? (digital ? "Registrar envio da cópia" : "Registrar empréstimo") : (digital ? "Pedir uma cópia digital" : "Pedir este livro emprestado")}</h2>
          <p>{registering ? "Registre para quem vai este livro e acompanhe a movimentação." : "Envie seu pedido ao responsável pelo catálogo."}</p>
        </header>

        <div className="loan-book-card">
          <div className="loan-book-cover"><BookCover url={book.cover_url} stored={false} catalog={catalog} id={book.id} version={book.version} title={book.title} /></div>
          <div className="loan-book-copy"><span>{digital ? "LIVRO DIGITAL" : "LIVRO FÍSICO"}</span><strong>{book.title}</strong><small>{book.format}{book.category ? ` · ${book.category}` : ""}</small></div>
        </div>

        <form className="loan-dialog-form" onSubmit={submit}>
          {registering ? <>
            <label>Nome de quem vai receber<input autoComplete="name" required maxLength={160} placeholder="Nome completo" value={name} onChange={(event) => setName(event.target.value)} /></label>
            <label>E-mail<input autoComplete="email" type="email" required maxLength={254} placeholder="pessoa@exemplo.com" value={email} onChange={(event) => setEmail(event.target.value)} /></label>
          </> : <label>Mensagem ao responsável <span className="loan-optional">opcional</span><textarea rows={3} maxLength={500} placeholder="Escreva uma mensagem junto com o pedido…" value={note} onChange={(event) => setNote(event.target.value)} /></label>}

          <div className={`loan-kind-note ${digital ? "digital" : "physical"}`}><span aria-hidden="true">{digital ? "✦" : "↻"}</span><p>{digital ? "A cópia digital é enviada uma vez e não precisa ser devolvida." : "O empréstimo fica em aberto até o responsável registrar a devolução."}</p></div>
          {error && <p className="loan-dialog-error" role="alert">{error}</p>}
          <footer className="loan-dialog-footer"><button type="button" disabled={busy} onClick={onClose}>Cancelar</button><button className="primary" type="submit" disabled={busy}>{busy ? "Salvando…" : registering ? (digital ? "Registrar envio" : "Confirmar empréstimo") : "Enviar pedido"}</button></footer>
        </form>
      </section>
    </div>
  );
}
