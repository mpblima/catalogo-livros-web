import { BookCover } from "./BookCover";
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Book } from "./book";
import { classify } from "./classify";
import "./book-editor.css";

type Props = {
  book: Book;
  catalog?: string;
  busy: boolean;
  serverError?: string;
  onChange: (book: Book) => void;
  onClose: () => void;
  onSave: (book: Book) => void;
  batchMode?: boolean;
  onBatchModeChange?: (enabled: boolean) => void;
  onDelete: () => void;
  lookup: (isbn: string) => Promise<Book>;
};
export function normalizeISBN(raw: string) {
  let n = raw.replace(/[^0-9xX]/g, "").toUpperCase();
  if (n.length === 18) n = n.slice(0, 13);
  if (
    n.length === 10 &&
    /^\d{9}[\dX]$/.test(n) &&
    [...n].reduce((s, c, i) => s + (10 - i) * (c === "X" ? 10 : Number(c)), 0) %
      11 ===
      0
  ) {
    n = "978" + n.slice(0, 9);
    return (
      n +
      ((10 -
        ([...n].reduce((s, c, i) => s + Number(c) * (i % 2 ? 3 : 1), 0) % 10)) %
        10)
    );
  }
  if (
    /^97[89]\d{10}$/.test(n) &&
    [...n].reduce((s, c, i) => s + Number(c) * (i % 2 ? 3 : 1), 0) % 10 === 0
  )
    return n;
  return "";
}
const fields = [
  ["title", "Título *"],
  ["subtitle", "Subtítulo"],
  ["author", "Autor(es)"],
  ["publisher", "Editora"],
  ["edition", "Edição"],
  ["isbn", "ISBN"],
  ["year", "Publicação / ano"],
  ["language", "Idioma"],
  ["pages", "Total de páginas"],
  ["subject", "Assunto"],
  ["keywords", "Palavras-chave"],
];
export function BookEditor({
  book,
  catalog = "",
  busy,
  serverError,
  onChange,
  onClose,
  onSave,
  batchMode = false,
  onBatchModeChange = () => {},
  onDelete,
  lookup,
}: Props) {
  const [code, setCode] = useState(
    String(book.metadata.barcode || book.metadata.isbn || ""),
  );
  const [tab, setTab] = useState("details");
  const [searching, setSearching] = useState(false);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const [coverOptions, setCoverOptions] = useState(false);
  const [coverFailed, setCoverFailed] = useState(false);
  useEffect(() => setCoverFailed(false), [book.cover_url]);
  const [extras, setExtras] = useState(
    JSON.stringify(
      Object.fromEntries(
        Object.entries(book.metadata).filter(
          ([k]) =>
            ![
              ...fields.map(([k]) => k),
              "barcode",
              "notes",
              "page",
              "location",
              "source",
              "cover_urls",
            ].includes(k),
        ),
      ),
      null,
      2,
    ),
  );
  const request = useRef(0),
    lastCode = useRef("");
  const latest = useRef(book);
  latest.current = book;
  const lookupRef = useRef(lookup);
  lookupRef.current = lookup;
  const changeRef = useRef(onChange);
  changeRef.current = onChange;
  const customCategory = useRef(Boolean(book.category));
  const hasClassificationInput = Boolean(
    book.title ||
      book.metadata.subtitle ||
      book.metadata.subject ||
      book.metadata.keywords ||
      book.summary,
  );
  const suggestedCategory =
    !customCategory.current &&
    hasClassificationInput
      ? classify(
          `${book.title} ${book.metadata.subtitle || ""}`,
          `${book.metadata.subject || ""} ${book.metadata.keywords || ""} ${book.summary}`,
          book.kind,
        )
      : book.category;
  useEffect(
    () => () => {
      request.current++;
    },
    [],
  );
  const search = async (raw: string, force = false) => {
    const isbn = normalizeISBN(raw);
    if (!isbn) {
      setError(
        "Informe um ISBN-10 ou ISBN-13 válido. Você também pode preencher manualmente.",
      );
      return;
    }
    if (!force && lastCode.current === isbn) return;
    lastCode.current = isbn;
    const serial = ++request.current;
    setSearching(true);
    setError("");
    setStatus("Pesquisando dados e capa…");
    try {
      const found = await lookupRef.current(raw);
      if (serial !== request.current) return;
      const current = latest.current;
      changeRef.current({
        ...current,
        title: found.title,
        summary: found.summary,
        cover_url: found.cover_url,
        category:
          !customCategory.current
            ? classify(
                `${found.title} ${found.metadata.subtitle || ""}`,
                `${found.metadata.subject || ""} ${found.metadata.keywords || ""} ${found.summary}`,
                current.kind,
              )
            : current.category,
        metadata: { ...current.metadata, ...found.metadata },
      });
      setStatus(
        found.cover_url
          ? "Dados e capa preenchidos. Revise antes de salvar."
          : "Dados preenchidos. Capa não encontrada; você pode informar uma URL.",
      );
    } catch (e) {
      if (serial === request.current) {
        setStatus("");
        setError((e as Error).message);
      }
    } finally {
      if (serial === request.current) setSearching(false);
    }
  };
  useEffect(() => {
    if (book.id || !normalizeISBN(code)) return;
    const timer = setTimeout(() => void search(code), 450);
    return () => clearTimeout(timer);
  }, [code]);
  const meta = (key: string, value: unknown) =>
    onChange({ ...book, metadata: { ...book.metadata, [key]: value } });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      if (!book.title.trim()) {
        setTab("details");
        throw Error("Informe o título do livro.");
      }
      for (const key of ["pages", "page"]) {
        const value = String(book.metadata[key] ?? "");
        if (
          value &&
          (!/^\d+$/.test(value) ||
            Number(value) < 1 ||
            Number(value) > (key === "pages" ? 9999999 : 1000000))
        ) {
          setTab("details");
          throw Error("Número de páginas ou página atual inválida.");
        }
      }
      if (book.cover_url && new URL(book.cover_url).protocol !== "https:")
        throw Error("A capa precisa de uma URL HTTPS válida.");
      const extra = JSON.parse(extras);
      if (!extra || Array.isArray(extra) || typeof extra !== "object")
        throw Error("Os outros metadados devem ser um objeto JSON.");
      const raw = String(book.metadata.isbn || normalizeISBN(code) || "");
      if (raw && !normalizeISBN(raw))
        throw Error(
          "ISBN inválido. Corrija o código ou deixe o ISBN em branco para cadastro manual.",
        );
      const category =
        !customCategory.current
          ? classify(
              `${book.title} ${book.metadata.subtitle || ""}`,
              `${book.metadata.subject || ""} ${book.metadata.keywords || ""} ${book.summary}`,
              book.kind,
            )
          : book.category;
      const known = Object.fromEntries(
        Object.entries(book.metadata).filter(([k]) =>
          [
            ...fields.map(([k]) => k),
            "barcode",
            "notes",
            "page",
            "location",
            "source",
            "cover_urls",
          ].includes(k),
        ),
      );
      onSave({
        ...book,
        category,
        metadata: {
          ...extra,
          ...known,
          title: book.title,
          isbn: raw ? normalizeISBN(raw) : "",
          barcode: code,
        },
      });
    } catch (e) {
      setError((e as Error).message);
    }
  };
  return (
    <div className="overlay">
      <section
        className="dialog book-editor"
        role="dialog"
        aria-modal="true"
        aria-labelledby="edit-title"
      >
        <button
          type="button"
          className="close"
          aria-label="Fechar edição"
          onClick={onClose}
        >
          ×
        </button>
        <p className="eyebrow">
          {book.kind === "physical" ? "ACERVO FÍSICO" : "ACERVO DIGITAL"}
        </p>
        <h2 id="edit-title">
          {book.id
            ? "Editar livro"
            : book.kind === "physical"
              ? "Um novo livro na estante"
              : "Adicionar livro digital"}
        </h2>
        <p className="editor-intro">
          {book.kind === "physical"
            ? "Leia o código de barras ou digite o ISBN. Revise os dados antes de salvar."
            : "Cadastre os dados do livro. O arquivo continua no seu computador."}
        </p>
        <form onSubmit={submit} noValidate>
          {!book.id && (
            <div className="isbn-search">
              <label htmlFor="book-code">
                ISBN / ID · código de barras (opcional)
              </label>
              <div>
                <input
                  id="book-code"
                  autoFocus
                  value={code}
                  placeholder="Pronto para o leitor ou digite o ISBN…"
                  onChange={(e) => {
                    request.current++;
                    lastCode.current = "";
                    setSearching(false);
                    setStatus("");
                    setError("");
                    setCode(e.target.value);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      void search(code, true);
                    }
                  }}
                />
                <button
                  type="button"
                  disabled={searching || busy}
                  onClick={() => void search(code, true)}
                >
                  Buscar informações
                </button>
              </div>
              <small>
                Um ISBN válido inicia a pesquisa automaticamente. Um ID próprio
                pode ser guardado sem consulta.
              </small>
            </div>
          )}
          <div
            className="editor-tabs"
            role="tablist"
            aria-label="Informações do livro"
          >
            {[
              ["details", "Dados do livro"],
              ["texts", "Resumo e anotações"],
              ["extras", "Outros metadados"],
            ].map(([id, label]) => (
              <button
                type="button"
                key={id}
                role="tab"
                id={`tab-${id}`}
                aria-selected={tab === id}
                aria-controls={`panel-${id}`}
                onClick={() => setTab(id)}
              >
                {label}
              </button>
            ))}
          </div>
          <div className="editor-layout">
            <div>
              <fieldset
                disabled={busy || searching}
                hidden={tab !== "details"}
                id="panel-details"
                role="tabpanel"
                aria-labelledby="tab-details"
              >
                <legend className="sr-only">Dados do livro</legend>
                <div className="editor-fields">
                  {fields.map(([key, label]) => (
                    <label
                      key={key}
                      className={
                        ["title", "subtitle", "author"].includes(key)
                          ? "wide"
                          : ""
                      }
                    >
                      {label}
                      <input
                        required={key === "title"}
                        maxLength={key === "title" ? 500 : undefined}
                        type={key === "pages" ? "number" : "text"}
                        min={key === "pages" ? 1 : undefined}
                        max={key === "pages" ? 9999999 : undefined}
                        value={
                          key === "title"
                            ? book.title
                            : String(book.metadata[key] || "")
                        }
                        onChange={(e) =>
                          key === "title"
                            ? onChange({ ...book, title: e.target.value })
                            : meta(key, e.target.value)
                        }
                      />
                    </label>
                  ))}
                  <label className="wide">
                    Categoria
                    {!customCategory.current ? " · automática" : ""}
                    <input
                      value={suggestedCategory}
                      onChange={(e) => {
                        customCategory.current = Boolean(e.target.value);
                        onChange({ ...book, category: e.target.value });
                      }}
                      placeholder="Classificada pelos dados do livro; você pode personalizar"
                    />
                  </label>
                  {book.kind === "digital" && (
                    <>
                      <label>
                        Formato
                        <input
                          value={book.format}
                          onChange={(e) =>
                            onChange({ ...book, format: e.target.value })
                          }
                          placeholder="PDF, EPUB…"
                        />
                      </label>
                      <label>
                        Nome do arquivo
                        <input
                          value={book.filename}
                          onChange={(e) =>
                            onChange({ ...book, filename: e.target.value })
                          }
                        />
                      </label>
                      <label className="wide">
                        Caminho físico do arquivo
                        <input
                          value={book.path}
                          onChange={(e) =>
                            onChange({ ...book, path: e.target.value })
                          }
                        />
                      </label>
                    </>
                  )}
                </div>
              </fieldset>
              <fieldset
                disabled={busy || searching}
                hidden={tab !== "texts"}
                id="panel-texts"
                role="tabpanel"
                aria-labelledby="tab-texts"
              >
                <legend className="sr-only">Resumo e anotações</legend>
                <label>
                  Resumo
                  <textarea
                    rows={8}
                    value={book.summary}
                    onChange={(e) =>
                      onChange({ ...book, summary: e.target.value })
                    }
                  />
                </label>
                <p className="editor-help">
                  As anotações ficam em um único espaço por livro. Abra
                  “Anotações” no catálogo para escrever e consultar as
                  contribuições. Elas são privadas enquanto o catálogo não é
                  compartilhado e passam a ser vistas pelos participantes
                  quando você os convida.
                </p>
              </fieldset>
              <fieldset
                disabled={busy || searching}
                hidden={tab !== "extras"}
                id="panel-extras"
                role="tabpanel"
                aria-labelledby="tab-extras"
              >
                <legend className="sr-only">Outros metadados</legend>
                <label>
                  Outros metadados (JSON)
                  <textarea
                    className="metadata-json"
                    rows={16}
                    value={extras}
                    onChange={(e) => setExtras(e.target.value)}
                  />
                </label>
                <small>
                  Campos adicionais do arquivo ou da edição. Os campos do
                  formulário têm prioridade.
                </small>
              </fieldset>
            </div>
            <div className="editor-aside">
              <div className="editor-cover">
                <BookCover
                  url={book.cover_url}
                  stored={book.metadata._stored_cover === true}
                  catalog={catalog}
                  id={book.id}
                  version={book.version}
                  title={book.title}
                  onUnavailable={() => setCoverFailed(true)}
                />
              </div>
              <button
                type="button"
                className="cover-options"
                onClick={() => setCoverOptions(!coverOptions)}
              >
                {book.cover_url ? "Trocar capa…" : "Opções de capa…"}
              </button>
              {(coverOptions || coverFailed) && (
                <div className="cover-options-panel">
                  <a
                    href={
                      "https://www.google.com/search?tbm=isch&q=" +
                      encodeURIComponent(
                        book.title +
                          " " +
                          String(book.metadata.isbn || "") +
                          " capa",
                      )
                    }
                    target="_blank"
                    rel="noreferrer"
                  >
                    Buscar capa no Google
                  </a>
                  <label>
                    URL da capa
                    <input
                      type="url"
                      value={book.cover_url}
                      disabled={busy || searching}
                      onChange={(e) =>
                        onChange({ ...book, cover_url: e.target.value })
                      }
                      placeholder="https://…"
                    />
                  </label>
                </div>
              )}
              <fieldset disabled={busy || searching}>
                <legend>Marcadores do catálogo</legend>
                <label className="check">
                  <input
                    type="checkbox"
                    checked={book.favorite}
                    onChange={(e) =>
                      onChange({ ...book, favorite: e.target.checked })
                    }
                  />{" "}
                  Favorito
                </label>
                <label className="check">
                  <input
                    type="checkbox"
                    checked={book.reading}
                    onChange={(e) =>
                      onChange({ ...book, reading: e.target.checked })
                    }
                  />{" "}
                  Estou lendo
                </label>
                {book.kind === "physical" && (
                  <label>
                    Local / estante
                    <input
                      value={String(book.metadata.location || "")}
                      onChange={(e) => meta("location", e.target.value)}
                    />
                  </label>
                )}
              </fieldset>
            </div>
          </div>
          {status && (
            <p className="lookup-status" role="status">
              {status}
            </p>
          )}
          {(error || serverError) && (
            <p className="error" role="alert">
              {error || serverError}
            </p>
          )}
          <div className="editor-footer">
            <span>Os dados só são gravados ao salvar.</span>
            <div className="actions">
              {!book.id && <label className="check"><input type="checkbox" checked={batchMode} onChange={(e) => onBatchModeChange(e.target.checked)} /> Cadastro em lote</label>}
              <button type="button" onClick={onClose}>
                Cancelar
              </button>
              {book.id && book.kind === "physical" && (
                <button
                  type="button"
                  className="danger"
                  disabled={busy || searching}
                  onClick={onDelete}
                >
                  Excluir cadastro
                </button>
              )}
              <button className="primary" disabled={busy || searching}>
                {busy ? "Salvando…" : "Salvar livro"}
              </button>
            </div>
          </div>
        </form>
      </section>
    </div>
  );
}
