import { useState } from "react";
import type { Book } from "./book";
import type { Relation } from "./structure";
import "./structure.css";
const label = (s: string) =>
  s === "livros_fisicos"
    ? "Livros físicos"
    : s === "livros_digitais"
      ? "Livros digitais"
      : s.replace(/_/g, " ");
export function CategoryTree({
  paths,
  selected,
  onSelect,
}: {
  paths: string[];
  selected: string;
  onSelect: (p: string) => void;
}) {
  const root = new Map<string, any>();
  for (const path of paths) {
    let tree = root;
    for (const part of path.split("/")) {
      if (!tree.has(part)) tree.set(part, new Map());
      tree = tree.get(part);
    }
  }
  const branch = (tree: Map<string, any>, prefix = "") =>
    [...tree]
      .sort(([a], [b]) => a.localeCompare(b, "pt-BR"))
      .map(([part, children]) => {
        const path = prefix ? prefix + "/" + part : part;
        return (
          <li key={path}>
            {children.size ? (
              <details
                open={
                  selected === path ||
                  selected.startsWith(path + "/") ||
                  !prefix
                }
              >
                <summary>
                  <button
                    type="button"
                    aria-pressed={selected === path}
                    onClick={(e) => {
                      e.preventDefault();
                      onSelect(path);
                    }}
                  >
                    {label(part)}
                  </button>
                </summary>
                <ul>{branch(children, path)}</ul>
              </details>
            ) : (
              <button
                type="button"
                aria-pressed={selected === path}
                onClick={() => onSelect(path)}
              >
                {label(part)}
              </button>
            )}
          </li>
        );
      });
  return (
    <div className="category-tree">
      <h3>Assuntos</h3>
      <button
        type="button"
        aria-pressed={!selected}
        onClick={() => onSelect("")}
      >
        Todos os assuntos
      </button>
      <ul>{branch(root)}</ul>
    </div>
  );
}
type Change = (method: string, body: unknown) => Promise<void>;
export function CategoryManager({
  paths,
  onClose,
  change,
}: {
  paths: string[];
  onClose: () => void;
  change: Change;
}) {
  const [path, setPath] = useState("");
  const [next, setNext] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const submit = async (method: string) => {
    setBusy(true);
    setError("");
    try {
      await change(method, {
        path: method === "POST" ? next : path,
        new_path: next,
      });
      setPath("");
      setNext("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="overlay">
      <section
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="category-title"
      >
        <button
          className="close"
          aria-label="Fechar categorias"
          onClick={onClose}
        >
          ×
        </button>
        <h2 id="category-title">Categorias de livros físicos</h2>
        <p>
          Renomear uma categoria atualiza seus livros e subcategorias. Somente
          categorias vazias podem ser excluídas.
        </p>
        <label>
          Categoria existente
          <select
            value={path}
            onChange={(e) => {
              setPath(e.target.value);
              setNext(e.target.value);
            }}
          >
            <option value="">Nova categoria</option>
            {paths
              .filter((p) => p.startsWith("livros_fisicos/"))
              .map((p) => (
                <option key={p}>{p}</option>
              ))}
          </select>
        </label>
        <label>
          {path ? "Novo caminho" : "Caminho da nova categoria"}
          <input
            value={next}
            onChange={(e) => setNext(e.target.value)}
            placeholder="livros_fisicos/literatura/romance"
          />
        </label>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
        <div className="actions">
          <button
            className="primary"
            disabled={busy || !next}
            onClick={() => void submit(path ? "PUT" : "POST")}
          >
            {path ? "Renomear" : "Criar categoria"}
          </button>
          {path && (
            <button
              className="danger"
              disabled={busy}
              onClick={() => {
                if (
                  window.confirm(
                    "Excluir esta categoria vazia e suas subcategorias vazias?",
                  )
                )
                  void submit("DELETE");
              }}
            >
              Excluir categoria
            </button>
          )}
        </div>
      </section>
    </div>
  );
}
export function RelationManager({
  book,
  books,
  relations,
  onClose,
  change,
}: {
  book: Book;
  books: Book[];
  relations: Relation[];
  onClose: () => void;
  change: Change;
}) {
  const [related, setRelated] = useState("");
  const [kind, setKind] = useState("same_work");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const submit = async (method: string, body: unknown) => {
    setBusy(true);
    setError("");
    try {
      await change(method, body);
      setRelated("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="overlay">
      <section
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="relation-title"
      >
        <button
          className="close"
          aria-label="Fechar relações"
          onClick={onClose}
        >
          ×
        </button>
        <h2 id="relation-title">Versões e amostras</h2>
        <p>{book.title}. Agrupar mantém todos os cadastros e arquivos.</p>
        <label>
          Outro livro digital
          <select value={related} onChange={(e) => setRelated(e.target.value)}>
            <option value="">Selecione</option>
            {books
              .filter((b) => b.id !== book.id && b.kind === "digital")
              .map((b) => (
                <option key={b.id} value={b.id}>
                  {b.title} · {b.format} · {b.filename}
                </option>
              ))}
          </select>
        </label>
        <label>
          Relação
          <select value={kind} onChange={(e) => setKind(e.target.value)}>
            <option value="same_work">
              Mesma obra / outra edição ou formato
            </option>
            <option value="sample_of">
              Este cadastro é uma amostra do selecionado
            </option>
          </select>
        </label>
        <button
          className="primary"
          disabled={busy || !related}
          onClick={() =>
            void submit("POST", {
              book_id: book.id,
              related_book_id: related,
              kind,
            })
          }
        >
          Relacionar livros
        </button>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
        <ul className="access-list">
          {relations
            .filter(
              (a) => a.book_id === book.id || a.related_book_id === book.id,
            )
            .map((a, i) => (
              <li key={i}>
                <span>
                  {books.find(
                    (b) =>
                      b.id ===
                      (a.book_id === book.id ? a.related_book_id : a.book_id),
                  )?.title || "Outro cadastro"}
                  <small>
                    {a.kind === "sample_of" ? "Amostra" : "Mesma obra"} ·{" "}
                    {a.source === "isbn_e_titulo"
                      ? "ISBN e título confirmados"
                      : "Relação cadastrada"}
                  </small>
                </span>
                {a.source !== "isbn_e_titulo" && (
                  <button
                    disabled={busy}
                    onClick={() => void submit("DELETE", a)}
                  >
                    Remover relação
                  </button>
                )}
              </li>
            ))}
        </ul>
      </section>
    </div>
  );
}
