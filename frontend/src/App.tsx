import { BookEditor } from "./BookEditor";
import { Welcome } from "./Welcome";
import type { Book } from "./book";
import { useAuth, UserButton } from "@clerk/react";
import { useCallback, useEffect, useRef, useState } from "react";
type Account = {
  id: string;
  email: string;
  name: string;
  admin: boolean;
  enabled: boolean;
  universal: boolean;
};
type Catalog = { id: string; name: string; owner: boolean };

type Comment = {
  id: string;
  body: string;
  created_at: string;
  author: { name: string; email: string };
};
type Grant = {
  email: string;
  enabled: boolean;
  universal: boolean;
  admin: boolean;
};
const blank = (kind: Book["kind"] = "physical"): Book => ({
  id: "",
  title: "",
  kind,
  format: kind === "physical" ? "FISICO" : "PDF",
  path: "",
  filename: "",
  category: "",
  metadata: {},
  summary: "",
  cover_url: "",
  favorite: false,
  reading: false,
  version: 0,
});
const norm = (s: string) =>
  s
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase();
export default function App() {
  const { isLoaded, isSignedIn, userId, getToken } = useAuth();
  const [account, setAccount] = useState<Account>();
  const [catalogs, setCatalogs] = useState<Catalog[]>([]);
  const [catalog, setCatalog] = useState("");
  const [books, setBooks] = useState<Book[]>([]);
  const [query, setQuery] = useState("");
  const [shelf, setShelf] = useState("all");
  const [kind, setKind] = useState("physical");
  const [category, setCategory] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<Book | null>(null);
  const [selected, setSelected] = useState<Book | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [note, setNote] = useState("");
  const [panel, setPanel] = useState<"members" | "admin" | null>(null);
  const [privacy, setPrivacy] = useState<
    { id: string; email: string; created_at: string; status: string }[]
  >([]);
  const [members, setMembers] = useState<string[]>([]);
  const [grants, setGrants] = useState<Grant[]>([]);
  const [email, setEmail] = useState("");
  const [universal, setUniversal] = useState(false);
  const [admin, setAdmin] = useState(false);
  const responseCache = useRef(
    new Map<string, { etag: string; data: unknown }>(),
  );
  const readEpoch = useRef(0);
  const writing = useRef(false);
  const active = useRef(catalog);
  active.current = catalog;
  const activeKind = useRef(kind);
  activeKind.current = kind;
  const api = useCallback(
    async <T,>(path: string, method = "GET", body?: unknown): Promise<T> => {
      const token = await getToken();
      const cacheKey = `${userId}:${path}`;
      const cached =
        method === "GET" && /\/books(?:\?|$)/.test(path)
          ? responseCache.current.get(cacheKey)
          : undefined;
      const response = await fetch(
        `${import.meta.env.VITE_API_URL || ""}/api${path}`,
        {
          method,
          headers: {
            Authorization: `Bearer ${token}`,
            ...(cached ? { "If-None-Match": cached.etag } : {}),
            ...(body ? { "Content-Type": "application/json" } : {}),
          },
          body: body ? JSON.stringify(body) : undefined,
        },
      );
      if (response.status === 304 && cached) return cached.data as T;
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || "Não foi possível conectar ao serviço.");
      }
      if (response.status === 204) return undefined as T;
      const data = await response.json();
      const etag = response.headers.get("ETag");
      if (etag && method === "GET" && /\/books(?:\?|$)/.test(path))
        responseCache.current.set(cacheKey, { etag, data });
      return data;
    },
    [getToken, userId],
  );
  const act = async (fn: () => Promise<void>) => {
    readEpoch.current++;
    writing.current = true;
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha na operação.");
    } finally {
      readEpoch.current++;
      writing.current = false;
      setBusy(false);
    }
  };
  useEffect(() => {
    responseCache.current.clear();
    setAccount(undefined);
    setBooks([]);
    setCatalogs([]);
    setCatalog("");
    if (!isSignedIn) return;
    let cancelled = false;
    setLoading(true);
    (async () => {
      try {
        const me = await api<Account>("/me");
        const list = await api<Catalog[]>("/catalogs");
        if (!cancelled) {
          setAccount(me);
          setCatalogs(list);
          setCatalog(list[0]?.id || "");
        }
      } catch (e) {
        if (!cancelled) setError((e as Error).message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isSignedIn, userId, api]);
  const reload = useCallback(
    async (id: string) => {
      if (writing.current) return;
      const epoch = readEpoch.current;
      const result = await api<Book[]>(`/catalogs/${id}/books?kind=${kind}`);
      if (
        active.current === id &&
        activeKind.current === kind &&
        epoch === readEpoch.current
      )
        setBooks(result);
    },
    [api, kind],
  );
  useEffect(() => {
    setBooks([]);
    setSelected(null);
    setEditing(null);
    setPanel(null);
    setCategory("");
    setQuery("");
    if (!catalog) return;
    let cancelled = false;
    setLoading(true);
    reload(catalog)
      .catch((e) => {
        if (!cancelled) setError(e.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [catalog, reload]);
  // Refresh shared edits without resetting an open form. Writes use version checks.
  useEffect(() => {
    if (!catalog) return;
    const refresh = () => {
      if (!document.hidden) reload(catalog).catch(() => {});
    };
    const timer = window.setInterval(refresh, 15000);
    window.addEventListener("focus", refresh);
    return () => {
      clearInterval(timer);
      window.removeEventListener("focus", refresh);
    };
  }, [catalog, reload]);
  useEffect(() => {
    if (!editing && !selected && !panel) return;
    const previous = document.activeElement as HTMLElement | null;
    const dialog = document.querySelector<HTMLElement>('[role="dialog"]');
    const focusable = () =>
      Array.from(
        dialog?.querySelectorAll<HTMLElement>(
          "button:not(:disabled),input,select,textarea,a[href]",
        ) || [],
      ).filter(
        (element) =>
          !element.closest("[hidden]") && !element.matches(":disabled"),
      );
    (
      dialog?.querySelector<HTMLElement>("input,textarea") || focusable()[0]
    )?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setEditing(null);
        setSelected(null);
        setPanel(null);
      }
      if (event.key === "Tab") {
        const items = focusable();
        const first = items[0];
        const last = items.at(-1);
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last?.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first?.focus();
        }
      }
    };
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      previous?.focus();
    };
  }, [Boolean(editing), Boolean(selected), panel]);
  const base = `/catalogs/${catalog}`;
  const current = catalogs.find((c) => c.id === catalog);
  const filtered = books.filter(
    (b) =>
      (shelf !== "favorites" || b.favorite) &&
      (shelf !== "reading" || b.reading) &&
      (!kind || b.kind === kind) &&
      (!category || b.category === category) &&
      norm(query)
        .split(/\s+/)
        .filter(Boolean)
        .every((t) =>
          norm(
            `${b.title} ${b.category} ${b.path} ${JSON.stringify(b.metadata)} ${b.summary}`,
          ).includes(t),
        ),
  );
  const toggle = (b: Book, field: "favorite" | "reading") =>
    act(async () => {
      const saved = await api<Book>(`${base}/books/${b.id}`, "PUT", {
        ...b,
        [field]: !b[field],
      });
      setBooks((list) => list.map((x) => (x.id === saved.id ? saved : x)));
      if (selected?.id === saved.id) setSelected(saved);
    });
  const openNotes = (b: Book) =>
    act(async () => {
      setComments(await api<Comment[]>(`${base}/books/${b.id}/comments`));
      setSelected(b);
      setNote("");
    });
  const save = (draft: Book) => {
    act(async () => {
      const saved = await api<Book>(
        `${base}/books${draft.id ? "/" + draft.id : ""}`,
        draft.id ? "PUT" : "POST",
        draft,
      );
      if (kind && saved.kind !== kind) setKind(saved.kind);
      setBooks((list) =>
        [...list.filter((x) => x.id !== saved.id), saved].sort((a, b) =>
          a.title.localeCompare(b.title, "pt-BR"),
        ),
      );
      setEditing(null);
    });
  };
  const openPanel = (value: "members" | "admin") =>
    act(async () => {
      setEmail("");
      setPanel(value);
      if (value === "members")
        setMembers(await api<string[]>(`${base}/members`));
      else {
        setGrants(await api<Grant[]>("/admin/access"));
        setPrivacy(await api<typeof privacy>("/admin/privacy"));
      }
    });
  if (!isLoaded)
    return <main className="setup">Carregando sua biblioteca…</main>;
  if (!isSignedIn) return <Welcome />;
  if (!account || account.id !== userId)
    return (
      <main className="setup">{error || "Carregando sua biblioteca…"}</main>
    );
  return (
    <div className="app">
      <header>
        <div className="brand">
          ▤{" "}
          <span>
            Biblioteca<small>CATÁLOGO DE LIVROS</small>
          </span>
        </div>
        <div className="actions">
          {account?.admin && (
            <button onClick={() => openPanel("admin")}>Administração</button>
          )}
          <UserButton />
        </div>
      </header>
      <div className="layout">
        <aside>
          <label>
            SEU CATÁLOGO
            <select
              value={catalog}
              onChange={(e) => setCatalog(e.target.value)}
            >
              {catalogs.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </label>
          <nav>
            {[
              ["all", "Todo o acervo"],
              ["favorites", "Favoritos"],
              ["reading", "Estou lendo"],
            ].map(([value, label]) => (
              <button
                key={value}
                aria-pressed={shelf === value}
                className={shelf === value ? "active" : ""}
                onClick={() => setShelf(value)}
              >
                {label}
                <span>
                  {
                    books.filter(
                      (b) =>
                        value === "all" ||
                        (value === "favorites" ? b.favorite : b.reading),
                    ).length
                  }
                </span>
              </button>
            ))}
          </nav>
          {current?.owner && (
            <button className="share" onClick={() => openPanel("members")}>
              Compartilhar catálogo
            </button>
          )}
          <button
            className="privacy-link"
            disabled={busy}
            onClick={() =>
              act(async () => {
                const data = await api<unknown>("/privacy/export");
                const url = URL.createObjectURL(
                  new Blob([JSON.stringify(data, null, 2)], {
                    type: "application/json",
                  }),
                );
                const a = document.createElement("a");
                a.href = url;
                a.download = "meus-dados.json";
                a.click();
                URL.revokeObjectURL(url);
              })
            }
          >
            Exportar meus dados
          </button>
          <button
            className="privacy-link"
            disabled={busy}
            onClick={() => {
              if (
                window.confirm(
                  "Registrar uma solicitação de exclusão dos seus dados para análise do administrador?",
                )
              )
                act(async () => {
                  await api("/privacy/erasure", "POST", {});
                  window.alert(
                    "Solicitação registrada. O administrador analisará a exclusão dos seus dados.",
                  );
                });
            }}
          >
            Solicitar exclusão de dados
          </button>
          <div className="aside-note">
            Um catálogo para organizar.
            <br />
            Anotações para compartilhar.
          </div>
        </aside>
        <main>
          <div className="page-head">
            <div>
              <p className="eyebrow">
                {current?.owner ? "SEU ACERVO" : "COMPARTILHADO COM VOCÊ"}
              </p>
              <h1>
                {shelf === "favorites"
                  ? "Seus favoritos"
                  : shelf === "reading"
                    ? "Estou lendo"
                    : kind === "physical"
                      ? "Livros físicos"
                      : kind === "digital"
                        ? "Livros digitais"
                        : "Todos os livros"}
              </h1>
            </div>
            {catalog && (
              <div className="actions add-books">
                <button
                  className="primary"
                  onClick={() => setEditing(blank("physical"))}
                >
                  + Livro físico
                </button>
                <button onClick={() => setEditing(blank("digital"))}>
                  + Livro digital
                </button>
              </div>
            )}
          </div>
          {error && (
            <div role="alert" className="error">
              {error}
              <button aria-label="Fechar mensagem" onClick={() => setError("")}>
                ×
              </button>
            </div>
          )}
          {!catalog && !loading && (
            <section className="empty">
              <h2>Seu catálogo ainda não está disponível.</h2>
              <p>
                {account?.enabled
                  ? "Atualize a página para tentar novamente."
                  : "Entre em contato com o administrador para liberar o acesso cloud."}
              </p>
            </section>
          )}
          {catalog && (
            <>
              <section className="collection-banner">
                <div>
                  <p className="eyebrow">ESPAÇO PARA SUAS DESCOBERTAS</p>
                  <h2>
                    Cada livro guarda uma ideia.
                    <br />
                    Aqui, elas ficam juntas.
                  </h2>
                </div>
                <div className="banner-count">
                  <strong>{books.length}</strong>
                  <span>livros no seu acervo</span>
                </div>
              </section>
              <div className="filters">
                <input
                  type="search"
                  aria-label="Pesquisar livros"
                  placeholder="Título, autor, editora ou assunto…"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
                <select
                  aria-label="Tipo de livro"
                  value={kind}
                  onChange={(e) => setKind(e.target.value)}
                >
                  <option value="">Físicos e digitais</option>
                  <option value="physical">Livros físicos</option>
                  <option value="digital">Livros digitais</option>
                </select>
                <select
                  aria-label="Assunto e subassunto"
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                >
                  <option value="">Todos os assuntos</option>
                  {[...new Set(books.map((b) => b.category).filter(Boolean))]
                    .sort()
                    .map((c) => (
                      <option key={c}>{c}</option>
                    ))}
                </select>
              </div>
              <p className="count" aria-live="polite">
                {loading
                  ? "Carregando…"
                  : `${filtered.length} livros no catálogo`}
              </p>
              <section className="books">
                {filtered.map((b) => (
                  <article key={b.id} className="book">
                    <div className="cover">
                      {b.cover_url ? (
                        <img
                          src={b.cover_url}
                          alt={`Capa de ${b.title}`}
                          loading="lazy"
                          referrerPolicy="no-referrer"
                        />
                      ) : (
                        <span className="cover-title" aria-hidden="true">
                          {b.title}
                          <small>{String(b.metadata.author || "")}</small>
                        </span>
                      )}
                    </div>
                    <div className="book-info">
                      <p className="eyebrow">
                        {b.kind === "physical"
                          ? "FÍSICO"
                          : b.format || "DIGITAL"}
                        {b.category && ` · ${b.category}`}
                      </p>
                      <h2>{b.title}</h2>
                      <p>
                        {String(b.metadata.author || "Autor não informado")}
                      </p>
                      {!!b.metadata.publisher && (
                        <small>{String(b.metadata.publisher)}</small>
                      )}
                      {b.kind === "digital" && (
                        <p className="file-path" title={b.path}>
                          {b.path || "Caminho local não informado"}
                        </p>
                      )}
                      <div className="book-actions">
                        <button
                          disabled={busy}
                          aria-pressed={b.favorite}
                          onClick={() => toggle(b, "favorite")}
                        >
                          {b.favorite ? "★ Favorito" : "☆ Favoritar"}
                        </button>
                        <button
                          disabled={busy}
                          aria-pressed={b.reading}
                          onClick={() => toggle(b, "reading")}
                        >
                          {b.reading ? "✓ Estou lendo" : "Marcar leitura"}
                        </button>
                        <button disabled={busy} onClick={() => openNotes(b)}>
                          Anotações
                        </button>
                        <button
                          onClick={() =>
                            setEditing({ ...b, metadata: { ...b.metadata } })
                          }
                        >
                          Editar
                        </button>
                      </div>
                    </div>
                  </article>
                ))}
              </section>
              {!loading && !filtered.length && (
                <section className="empty">
                  <h2>
                    {books.length
                      ? "Nenhum livro neste filtro."
                      : "Sua biblioteca começa aqui."}
                  </h2>
                  <p>
                    {books.length
                      ? "Experimente outros termos ou filtros."
                      : "Adicione seu primeiro livro físico ou digital."}
                  </p>
                </section>
              )}
            </>
          )}
        </main>
      </div>
      {editing && (
        <BookEditor
          key={editing.id || editing.kind}
          book={editing}
          busy={busy}
          serverError={error}
          onChange={setEditing}
          onClose={() => setEditing(null)}
          onSave={save}
          lookup={(isbn) =>
            api<Book>(`/books/lookup?isbn=${encodeURIComponent(isbn)}`)
          }
          onDelete={() => {
            if (
              window.confirm(
                "Excluir este cadastro? O arquivo no computador será mantido.",
              )
            )
              act(async () => {
                await api(`${base}/books/${editing.id}`, "DELETE", {
                  version: editing.version,
                });
                setBooks((list) => list.filter((b) => b.id !== editing.id));
                setEditing(null);
              });
          }}
        />
      )}
      {selected && (
        <div className="overlay">
          <section
            className="dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="notes-title"
          >
            <button
              className="close"
              aria-label="Fechar anotações"
              onClick={() => setSelected(null)}
            >
              ×
            </button>
            <p className="eyebrow">ANOTAÇÕES COMPARTILHADAS</p>
            <h2 id="notes-title">{selected.title}</h2>
            {comments.length ? (
              comments.map((c) => (
                <article className="comment" key={c.id}>
                  <strong>{c.author.name}</strong>
                  <small>
                    {c.author.email} ·{" "}
                    {new Date(c.created_at).toLocaleString("pt-BR")}
                  </small>
                  <p>{c.body}</p>
                </article>
              ))
            ) : (
              <p>A primeira anotação pode ser sua.</p>
            )}
            <form
              onSubmit={(e) => {
                e.preventDefault();
                act(async () => {
                  await api(`${base}/books/${selected.id}/comments`, "POST", {
                    body: note,
                  });
                  setComments(
                    await api<Comment[]>(
                      `${base}/books/${selected.id}/comments`,
                    ),
                  );
                  setNote("");
                });
              }}
            >
              <label>
                Sua anotação
                <textarea
                  required
                  maxLength={20000}
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                />
              </label>
              <button className="primary" disabled={busy}>
                Adicionar anotação
              </button>
            </form>
          </section>
        </div>
      )}
      {panel && (
        <div className="overlay">
          <section
            className="dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="panel-title"
          >
            <button
              className="close"
              aria-label="Fechar painel"
              onClick={() => setPanel(null)}
            >
              ×
            </button>
            <h2 id="panel-title">
              {panel === "members"
                ? "Compartilhar catálogo"
                : "Administração de acessos"}
            </h2>
            <p>
              {panel === "members"
                ? "Adicione o e-mail usado pela pessoa no Google. Ela receberá um convite por e-mail para acessar os livros e as anotações deste catálogo."
                : "Contas universais têm uso ilimitado. Cada catálogo permanece privado."}
            </p>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                act(async () => {
                  if (panel === "members") {
                    await api(`${base}/members`, "POST", { email });
                    setMembers(await api<string[]>(`${base}/members`));
                  } else {
                    await api("/admin/access", "PUT", {
                      email,
                      enabled: true,
                      universal,
                      admin,
                    });
                    setGrants(await api<Grant[]>("/admin/access"));
                  }
                  setEmail("");
                });
              }}
            >
              <label>
                E-mail
                <input
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </label>
              {panel === "admin" && (
                <>
                  <label className="check">
                    <input
                      type="checkbox"
                      checked={universal}
                      onChange={(e) => setUniversal(e.target.checked)}
                    />
                    Uso ilimitado
                  </label>
                  <label className="check">
                    <input
                      type="checkbox"
                      checked={admin}
                      onChange={(e) => setAdmin(e.target.checked)}
                    />
                    Administrador do serviço
                  </label>
                </>
              )}
              <button disabled={busy} className="primary">
                {panel === "members" ? "Adicionar usuário" : "Salvar acesso"}
              </button>
            </form>
            <ul className="access-list">
              {panel === "members"
                ? members.map((m) => (
                    <li key={m}>
                      <span>{m}</span>
                      <button
                        disabled={busy}
                        onClick={() =>
                          act(async () => {
                            await api(
                              `${base}/members/${encodeURIComponent(m)}`,
                              "DELETE",
                            );
                            setMembers((list) => list.filter((x) => x !== m));
                          })
                        }
                      >
                        Remover
                      </button>
                    </li>
                  ))
                : grants.map((g) => (
                    <li key={g.email}>
                      <span>
                        {g.email}
                        <small>
                          {g.admin ? "Administrador · " : ""}
                          {g.universal ? "Ilimitado · " : ""}
                          {g.enabled ? "Ativo" : "Suspenso"}
                        </small>
                      </span>
                      <button
                        disabled={busy}
                        onClick={() =>
                          act(async () => {
                            await api("/admin/access", "PUT", {
                              ...g,
                              enabled: !g.enabled,
                            });
                            setGrants(await api<Grant[]>("/admin/access"));
                          })
                        }
                      >
                        {g.enabled ? "Suspender" : "Ativar"}
                      </button>
                      <button
                        onClick={() => {
                          setEmail(g.email);
                          setAdmin(g.admin);
                          setUniversal(g.universal);
                        }}
                      >
                        Editar
                      </button>
                    </li>
                  ))}
            </ul>
            {panel === "admin" && (
              <section>
                <h3>Solicitações de privacidade</h3>
                {privacy.length ? (
                  privacy.map((p) => (
                    <p key={p.id}>
                      {p.email} · {p.status}
                      <br />
                      <small>
                        {new Date(p.created_at).toLocaleString("pt-BR")}
                      </small>
                    </p>
                  ))
                ) : (
                  <p>Nenhuma solicitação registrada.</p>
                )}
              </section>
            )}
          </section>
        </div>
      )}
    </div>
  );
}
