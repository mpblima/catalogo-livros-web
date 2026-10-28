import { DeviceConnection, Devices } from "./DeviceConnection";
import { BookCover } from "./BookCover";
import { PrivacyNotice } from "./PrivacyNotice";
import {
  CategoryTree,
  CategoryManager,
  RelationManager,
} from "./CatalogStructure";
import { groupBooks, type Relation } from "./structure";
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
  origin?: string;
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
type ManagedCatalog = { id: string; name: string; owner_email?: string; members?: string[] };
type ManagedUser = {
  id: string;
  email: string;
  name: string;
  created_at: number;
  last_sign_in_at: number | null;
  banned: boolean;
  locked: boolean;
  enabled: boolean;
  universal: boolean;
  admin: boolean;
  access_granted: boolean;
  owned_catalogs: ManagedCatalog[];
  shared_catalogs: ManagedCatalog[];
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
export default function App() {
  const { isLoaded, isSignedIn, userId, getToken } = useAuth();
  const [account, setAccount] = useState<Account>();
  const [catalogs, setCatalogs] = useState<Catalog[]>([]);
  const [catalog, setCatalog] = useState("");
  const [books, setBooks] = useState<Book[]>([]);
  const [bookTotal, setBookTotal] = useState(0);
  const [bookGroupTotal, setBookGroupTotal] = useState(0);
  const [bookCounts, setBookCounts] = useState({ all: 0, physical: 0, digital: 0, favorites: 0, reading: 0 });
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [shelf, setShelf] = useState("all");
  const [kind, setKind] = useState("physical");
  const [category, setCategory] = useState("");
  const [categoryPaths, setCategoryPaths] = useState<string[]>([]);
  const [relations, setRelations] = useState<Relation[]>([]);
  const [versions, setVersions] = useState<Record<string, string>>({});
  const [categoryManager, setCategoryManager] = useState(false);
  const [relationBook, setRelationBook] = useState<Book | null>(null);
  const [pairing, setPairing] = useState("");
  const [deviceManager, setDeviceManager] = useState(false);
  useEffect(() => {
    if (account)
      setPairing(
        new URLSearchParams(window.location.search).get("conectar") || "",
      );
  }, [Boolean(account)]);
  const closePairing = () => {
    setPairing("");
    const url = new URL(window.location.href);
    url.searchParams.delete("conectar");
    window.history.replaceState(null, "", url.pathname + url.search + url.hash);
  };
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [refreshSeconds, setRefreshSeconds] = useState(() => {
    const saved = Number(window.localStorage.getItem("catalog-refresh-seconds"));
    return [0, 15, 30, 60, 300].includes(saved) || (saved >= 60 && saved <= 86400) ? saved : 15;
  });
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<Book | null>(null);
  const [batchMode, setBatchMode] = useState(false);
  const [selected, setSelected] = useState<Book | null>(null);
  const [bookPage, setBookPage] = useState(1);
  const [bookRefresh, setBookRefresh] = useState(0);
  const [comments, setComments] = useState<Comment[]>([]);
  const [note, setNote] = useState("");
  const [panel, setPanel] = useState<"members" | "admin" | null>(null);
  const [privacy, setPrivacy] = useState<
    { id: string; email: string; created_at: string; status: string }[]
  >([]);
  const [members, setMembers] = useState<string[]>([]);
  const [grants, setGrants] = useState<Grant[]>([]);
  const [managedUsers, setManagedUsers] = useState<ManagedUser[]>([]);
  const [managedUserPage, setManagedUserPage] = useState(1);
  const [managedUserQuery, setManagedUserQuery] = useState("");
  const [email, setEmail] = useState("");
  const [universal, setUniversal] = useState(false);
  const [admin, setAdmin] = useState(false);
  const responseCache = useRef(
    new Map<string, { etag: string; data: unknown }>(),
  );
  const reloadRequest = useRef<{ key: string; promise: Promise<void> } | null>(null);
  const commentsRequest = useRef(false);
  const readEpoch = useRef(0);
  const writing = useRef(false);
  const active = useRef(catalog);
  active.current = catalog;
  const activeKind = useRef(kind);
  activeKind.current = kind;
  const activeBookQuery = useRef({ catalog, kind, bookPage, category, shelf, query: debouncedQuery });
  activeBookQuery.current = { catalog, kind, bookPage, category, shelf, query: debouncedQuery };
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
    (id: string): Promise<void> => {
      if (writing.current) return Promise.resolve();
      const offset = (bookPage - 1) * 30;
      const params = new URLSearchParams({ kind, limit: "30", offset: String(offset), shelf });
      if (category) params.set("category", category);
      if (debouncedQuery.trim()) params.set("q", debouncedQuery.trim());
      const requestState = { catalog: id, kind, bookPage, category, shelf, query: debouncedQuery };
      const key = `${id}:${params.toString()}:${readEpoch.current}`;
      if (reloadRequest.current?.key === key) return reloadRequest.current.promise;
      const epoch = readEpoch.current;
      const request = (async () => {
        const [result, paths, links] = await Promise.all([
          api<{ items: Book[]; total: number; groups: number; counts: Partial<typeof bookCounts> }>(`/catalogs/${id}/books?${params}`),
          api<string[]>(`/catalogs/${id}/categories`),
          api<Relation[]>(`/catalogs/${id}/relations`),
        ]);
        if (
          active.current === id &&
          activeKind.current === kind &&
          JSON.stringify(activeBookQuery.current) === JSON.stringify(requestState) &&
          epoch === readEpoch.current
        ) {
          setBooks(result.items);
          setBookTotal(result.total);
          setBookGroupTotal(result.groups);
          setBookCounts({ all: 0, physical: 0, digital: 0, favorites: 0, reading: 0, ...result.counts });
          setCategoryPaths(paths);
          setRelations(links);
        }
      })();
      let tracked: Promise<void>;
      tracked = request.finally(() => {
        if (reloadRequest.current?.promise === tracked) reloadRequest.current = null;
      });
      reloadRequest.current = { key, promise: tracked };
      return tracked;
    },
    [api, bookPage, category, debouncedQuery, kind, shelf],
  );
  useEffect(() => {
    setBooks([]);
    setBookTotal(0);
    setBookGroupTotal(0);
    setBookCounts({ all: 0, physical: 0, digital: 0, favorites: 0, reading: 0 });
    setCategoryPaths([]);
    setRelations([]);
    setCategoryManager(false);
    setRelationBook(null);
    setSelected(null);
    setEditing(null);
    setPanel(null);
    setCategory("");
    setQuery("");
    if (!catalog) return;
  }, [catalog]);
  useEffect(() => {
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
  }, [catalog, reload, bookRefresh]);
  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query), 250);
    return () => window.clearTimeout(timer);
  }, [query]);
  // Refresh shared edits without resetting an open form. Writes use version checks.
  useEffect(() => {
    if (!catalog) return;
    const refresh = () => {
      if (!document.hidden) reload(catalog).catch(() => {});
    };
    const timer = refreshSeconds > 0 ? window.setInterval(refresh, refreshSeconds * 1000) : undefined;
    if (refreshSeconds > 0) window.addEventListener("focus", refresh);
    return () => {
      if (timer !== undefined) clearInterval(timer);
      if (refreshSeconds > 0) window.removeEventListener("focus", refresh);
    };
  }, [catalog, reload, refreshSeconds]);
  useEffect(() => {
    if (!selected || refreshSeconds === 0) return;
    const refreshComments = () => {
      if (document.hidden || writing.current || commentsRequest.current) return;
      commentsRequest.current = true;
      api<Comment[]>(`${base}/books/${selected.id}/comments`)
        .then(setComments)
        .catch(() => {})
        .finally(() => { commentsRequest.current = false; });
    };
    const timer = window.setInterval(refreshComments, refreshSeconds * 1000);
    return () => clearInterval(timer);
  }, [selected, refreshSeconds, api]);
  useEffect(() => {
    window.localStorage.setItem("catalog-refresh-seconds", String(refreshSeconds));
  }, [refreshSeconds]);
  useEffect(() => {
    if (
      !editing &&
      !selected &&
      !panel &&
      !categoryManager &&
      !relationBook &&
      !pairing &&
      !deviceManager
    )
      return;
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
        setCategoryManager(false);
        setRelationBook(null);
        setDeviceManager(false);
        closePairing();
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
  }, [
    Boolean(editing),
    Boolean(selected),
    panel,
    categoryManager,
    Boolean(relationBook),
    Boolean(pairing),
    deviceManager,
  ]);
  const base = `/catalogs/${catalog}`;
  const current = catalogs.find((c) => c.id === catalog);
  const filtered = books;
  const bookGroups = groupBooks(filtered, relations);
  const pageCount = Math.max(1, Math.ceil(bookGroupTotal / 30));
  const visibleGroups = bookGroups;
  const filteredManagedUsers = managedUsers.filter((u) =>
    `${u.name} ${u.email}`.toLocaleLowerCase("pt-BR").includes(managedUserQuery.trim().toLocaleLowerCase("pt-BR")),
  );
  useEffect(() => setBookPage(1), [catalog, kind, category, shelf, query]);
  useEffect(() => setBookPage((page) => Math.min(page, pageCount)), [pageCount]);
  const toggle = (b: Book, field: "favorite" | "reading") =>
    act(async () => {
      const saved = await api<Book>(`${base}/books/${b.id}`, "PUT", {
        ...b,
        [field]: !b[field],
      });
      const remainsInShelf = shelf === "all" || (shelf === "favorites" ? saved.favorite : saved.reading);
      setBooks((list) => remainsInShelf ? list.map((x) => (x.id === saved.id ? saved : x)) : list.filter((x) => x.id !== saved.id));
      if (!remainsInShelf) setBookTotal((total) => Math.max(0, total - 1));
      if (selected?.id === saved.id) setSelected(saved);
      setBookRefresh((value) => value + 1);
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
      setEditing(batchMode && !draft.id ? blank(saved.kind) : null);
      setCategoryPaths(await api<string[]>(`${base}/categories`));
      setRelations(await api<Relation[]>(`${base}/relations`));
      setBookRefresh((value) => value + 1);
    });
  };
  const openPanel = (value: "members" | "admin") =>
    act(async () => {
      setEmail("");
      if (value === "admin") { setManagedUserPage(1); setManagedUserQuery(""); }
      setPanel(value);
      if (value === "members")
        setMembers(await api<string[]>(`${base}/members`));
      else {
        setGrants(await api<Grant[]>("/admin/access"));
        setPrivacy(await api<typeof privacy>("/admin/privacy"));
        setManagedUsers(await api<ManagedUser[]>("/admin/users"));
      }
    });
  if (!isLoaded)
    return <main className="setup">Carregando sua biblioteca…</main>;
  if (!isSignedIn) return <Welcome />;
  if (!account || account.id !== userId)
    return (
      <main className="setup">{error || "Carregando sua biblioteca…"}</main>
    );
  const annotations: Comment[] = [...comments];
  const legacyNote = String(selected?.metadata.notes || "").trim();
  if (selected && legacyNote && !annotations.some((item) => item.body.trim() === legacyNote)) {
    annotations.push({
      id: `catalog-note-${selected.id}`,
      body: legacyNote,
      created_at: "",
      origin: "catalog",
      author: {
        name: String(selected.metadata.notes_author_name || "Anotação do catálogo"),
        email: String(selected.metadata.notes_author_email || ""),
      },
    });
  }
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
          <label className="refresh-setting">
            Atualizar
            <select aria-label="Intervalo de atualização automática" value={[0, 15, 30, 60, 300].includes(refreshSeconds) ? refreshSeconds : -1} onChange={(e) => setRefreshSeconds(Number(e.target.value) === -1 ? 600 : Number(e.target.value))}>
              <option value={0}>Manual</option>
              <option value={15}>15 segundos</option>
              <option value={30}>30 segundos</option>
              <option value={60}>1 minuto</option>
              <option value={300}>5 minutos</option>
              <option value={-1}>Personalizado</option>
            </select>
            {refreshSeconds > 0 && ![15, 30, 60, 300].includes(refreshSeconds) && <input aria-label="Intervalo personalizado em minutos" type="number" min={1} max={1440} value={Math.round(refreshSeconds / 60)} onChange={(e) => { const minutes = Number(e.target.value); if (minutes >= 1 && minutes <= 1440) setRefreshSeconds(minutes * 60); }} />}
          </label>
          <button disabled={!catalog || loading} onClick={() => Promise.all([reload(catalog), selected ? api<Comment[]>(`${base}/books/${selected.id}/comments`).then(setComments) : Promise.resolve()]).catch((e) => setError(e.message))}>Atualizar agora</button>
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
                  {bookCounts[value as keyof typeof bookCounts]}
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
          <CategoryTree
            paths={categoryPaths.filter(
              (p) =>
                !kind ||
                (kind === "physical"
                  ? p.startsWith("livros_fisicos/") ||
                    books.some((b) => b.category === p)
                  : !p.startsWith("livros_fisicos/")),
            )}
            selected={category}
            onSelect={(p) => {
              setCategory(p);
              setQuery("");
              setShelf("all");
            }}
          />
          <button
            className="category-management"
            onClick={() => setCategoryManager(true)}
          >
            Gerenciar categorias
          </button>
          <button
            className="privacy-link"
            onClick={() => setDeviceManager(true)}
          >
            Dispositivos conectados
          </button>
          <PrivacyNotice />
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
                  <strong>{bookCounts.all}</strong>
                  <span>livros no seu acervo</span>
                  <small>{bookCounts.physical} físicos · {bookCounts.digital} digitais</small>
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
                  {[...categoryPaths].sort()
                    .map((c) => (
                      <option key={c}>{c}</option>
                    ))}
                </select>
              </div>
              <p className="count" aria-live="polite">
                {loading
                  ? "Carregando…"
                  : `${bookTotal} livros no catálogo`}
              </p>
              <section className="books">
                {visibleGroups.map((group) => {
                  const b =
                    group.books.find(
                      (item) => item.id === versions[group.id],
                    ) || group.books[0];
                  return (
                    <article key={b.id} className="book">
                      <div className="cover">
                        <BookCover
                          url={b.cover_url}
                          stored={b.metadata._stored_cover === true}
                          catalog={catalog}
                          id={b.id}
                          version={b.version}
                          title={b.title}
                        />
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
                        {group.books.length > 1 && (
                          <label className="book-versions">
                            {group.books.length} versões / arquivos
                            <select
                              aria-label={"Versões de " + b.title}
                              value={b.id}
                              onChange={(e) =>
                                setVersions({
                                  ...versions,
                                  [group.id]: e.target.value,
                                })
                              }
                            >
                              {group.books.map((item) => (
                                <option value={item.id} key={item.id}>
                                  {item.format} ·{" "}
                                  {group.samples.has(item.id)
                                    ? "Amostra · "
                                    : ""}
                                  {item.filename || item.title}
                                </option>
                              ))}
                            </select>
                          </label>
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
                          {b.kind === "digital" && (
                            <button onClick={() => setRelationBook(b)}>
                              Versões e amostras
                            </button>
                          )}
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
                  );
                })}
              </section>
              {pageCount > 1 && (
                <nav className="catalog-pagination" aria-label="Paginação do catálogo">
                  <button disabled={bookPage === 1} onClick={() => setBookPage((page) => Math.max(1, page - 1))}>
                    Anterior
                  </button>
                <span aria-live="polite">Página {bookPage} de {pageCount} · até 30 livros</span>
                  <button disabled={bookPage === pageCount} onClick={() => setBookPage((page) => Math.min(pageCount, page + 1))}>
                    Próxima
                  </button>
                </nav>
              )}
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
      {pairing && (
        <DeviceConnection
          pair={pairing}
          catalogs={catalogs}
          api={api}
          onClose={closePairing}
        />
      )}
      {deviceManager && (
        <Devices api={api} onClose={() => setDeviceManager(false)} />
      )}
      {categoryManager && (
        <CategoryManager
          paths={categoryPaths}
          onClose={() => setCategoryManager(false)}
          change={async (method, body) => {
            await api(`${base}/categories`, method, body);
            await reload(catalog);
          }}
        />
      )}
      {relationBook && (
        <RelationManager
          book={relationBook}
          books={books}
          relations={relations}
          onClose={() => setRelationBook(null)}
          change={async (method, body) => {
            await api(`${base}/relations`, method, body);
            await reload(catalog);
          }}
        />
      )}
      {editing && (
        <BookEditor
          key={editing.id || editing.kind}
          book={editing}
          catalog={catalog}
          busy={busy}
          serverError={error}
          onChange={setEditing}
          onClose={() => setEditing(null)}
          onSave={save}
          batchMode={batchMode}
          onBatchModeChange={setBatchMode}
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
                setBookTotal((total) => Math.max(0, total - 1));
                setBookRefresh((value) => value + 1);
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
            <p className="eyebrow">ANOTAÇÕES</p>
            <h2 id="notes-title">{selected.title}</h2>
            {annotations.length ? (
              annotations.map((c) => (
                <article className={`comment ${c.author.email === account?.email ? "own" : ""}`} key={c.id}>
                  <strong>{c.author.name}</strong>
                  {(c.author.email || c.origin || c.created_at) && <small>
                    {[c.author.email, c.origin === "desktop" ? "Desktop" : c.origin === "catalog" ? "Catálogo" : c.origin ? "Web" : "", c.created_at ? new Date(c.created_at).toLocaleString("pt-BR") : ""].filter(Boolean).join(" · ")}
                  </small>}
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
                ? "Adicione o e-mail usado pela pessoa no cadastro. Ela receberá um convite por e-mail para acessar os livros e as anotações deste catálogo."
                : "Consulte contas e compartilhamentos, suspenda acessos ou exclua uma conta. O painel mostra relações entre catálogos, sem abrir livros ou anotações. Contas universais têm uso ilimitado."}
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
              {panel === "members" && members.map((m) => (
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
                  ))}
            </ul>
            {panel === "admin" && (
              <section>
                <h3>Contas e catálogos</h3>
                <input type="search" aria-label="Buscar contas por nome ou e-mail" placeholder="Buscar conta por nome ou e-mail…" value={managedUserQuery} onChange={(e) => { setManagedUserQuery(e.target.value); setManagedUserPage(1); }} />
                {filteredManagedUsers.length ? (
                  <div className="admin-users">
                    {filteredManagedUsers.slice((managedUserPage - 1) * 10, managedUserPage * 10).map((u) => (
                      <details className="admin-user" key={u.id}>
                        <summary>
                          <span>
                            <strong>{u.name}</strong>
                            <small>{u.email} · {u.banned || u.locked ? "Bloqueada no Clerk" : u.enabled ? "Acesso ativo" : "Acesso suspenso"}{u.admin ? " · Administrador" : ""}</small>
                          </span>
                          <span>{u.owned_catalogs.length} próprios · {u.shared_catalogs.length} recebidos</span>
                        </summary>
                        <div className="admin-user-detail">
                          <p>Criada: {new Date(u.created_at).toLocaleString("pt-BR")}</p>
                          <p>Último acesso: {u.last_sign_in_at ? new Date(u.last_sign_in_at).toLocaleString("pt-BR") : "Ainda não entrou"}</p>
                          <h4>Catálogos próprios</h4>
                          {u.owned_catalogs.length ? u.owned_catalogs.map((c) => (
                            <p key={c.id}><strong>{c.name}</strong><br />Compartilhado com: {c.members?.length ? c.members.join(", ") : "ninguém"}</p>
                          )) : <p>Nenhum.</p>}
                          <h4>Catálogos compartilhados com a conta</h4>
                          {u.shared_catalogs.length ? u.shared_catalogs.map((c) => (
                            <p key={c.id}><strong>{c.name}</strong><br />Responsável: {c.owner_email}</p>
                          )) : <p>Nenhum.</p>}
                          <div className="admin-user-actions">
                            <button disabled={busy || u.banned || u.locked || (u.admin && grants.filter((g) => g.admin && g.enabled).length <= 1)} onClick={() => act(async () => {
                              await api("/admin/access", "PUT", { email: u.email, enabled: !u.enabled, universal: u.universal, admin: u.admin });
                              setGrants(await api<Grant[]>("/admin/access"));
                              setManagedUsers(await api<ManagedUser[]>("/admin/users"));
                            })}>{u.enabled ? "Suspender acesso" : "Reativar acesso"}</button>
                            <button className="danger" disabled={busy || u.id === userId} onClick={() => {
                              const confirmation = window.prompt(`A exclusão remove os catálogos próprios de ${u.email}, revoga o acesso recebido e apaga a conta no Clerk. Anotações em catálogos de outras pessoas permanecem sem identificação. Digite EXCLUIR para confirmar.`);
                              if (confirmation === "EXCLUIR") act(async () => {
                                const request = await api<{ request_id: string }>(`/admin/users/${encodeURIComponent(u.id)}/erasure`, "POST", { confirmation });
                                await api(`/admin/privacy/${request.request_id}/complete`, "POST", { confirmation });
                                setManagedUsers(await api<ManagedUser[]>("/admin/users"));
                                setGrants(await api<Grant[]>("/admin/access"));
                                setPrivacy(await api<typeof privacy>("/admin/privacy"));
                              });
                            }}>Excluir conta</button>
                          </div>
                        </div>
                      </details>
                    ))}
                  </div>
                ) : <p>Nenhuma conta encontrada.</p>}
                {filteredManagedUsers.length > 10 && <nav className="catalog-pagination" aria-label="Paginação de contas">
                  <button disabled={managedUserPage <= 1} onClick={() => setManagedUserPage((page) => Math.max(1, page - 1))}>Anterior</button>
                  <span>Página {managedUserPage} de {Math.ceil(filteredManagedUsers.length / 10)} · {filteredManagedUsers.length} contas</span>
                  <button disabled={managedUserPage >= Math.ceil(filteredManagedUsers.length / 10)} onClick={() => setManagedUserPage((page) => Math.min(Math.ceil(filteredManagedUsers.length / 10), page + 1))}>Próxima</button>
                </nav>}
                <h3>Acessos e permissões</h3>
                <ul className="access-list">
                  {grants.map((g) => (
                    <li key={g.email}>
                      <span>{g.email}<small>{g.admin ? "Administrador · " : ""}{g.universal ? "Ilimitado · " : ""}{g.enabled ? "Ativo" : "Suspenso"}</small></span>
                      <button disabled={busy} onClick={() => act(async () => {
                        await api("/admin/access", "PUT", { ...g, enabled: !g.enabled });
                        setGrants(await api<Grant[]>("/admin/access"));
                        setManagedUsers(await api<ManagedUser[]>("/admin/users"));
                      })}>{g.enabled ? "Suspender" : "Ativar"}</button>
                      <button onClick={() => { setEmail(g.email); setAdmin(g.admin); setUniversal(g.universal); }}>Editar</button>
                    </li>
                  ))}
                </ul>
                <h3>Solicitações de privacidade</h3>
                {error && (
                  <p className="error" role="alert">
                    {error}
                  </p>
                )}
                {privacy.length ? (
                  privacy.map((p) => (
                    <p key={p.id}>
                      {p.email} · {p.status}
                      {p.status !== "completed" && (
                        <button
                          className="danger"
                          disabled={busy}
                          onClick={() => {
                            const confirmation = window.prompt(
                              "Esta operação remove os catálogos próprios, revoga dispositivos e exclui a conta " +
                                p.email +
                                ". O histórico em catálogos alheios ficará com a identidade removida. Digite EXCLUIR para confirmar.",
                            );
                            if (confirmation === "EXCLUIR")
                              act(async () => {
                                await api(
                                  `/admin/privacy/${p.id}/complete`,
                                  "POST",
                                  { confirmation },
                                );
                                setPrivacy(
                                  await api<typeof privacy>("/admin/privacy"),
                                );
                                setGrants(await api<Grant[]>("/admin/access"));
                              });
                          }}
                        >
                          {p.status === "provider_pending"
                            ? "Concluir exclusão no Clerk"
                            : "Concluir exclusão"}
                        </button>
                      )}
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
