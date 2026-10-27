// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import App from "./App";
const session = vi.hoisted(() => ({
  signedIn: true,
  getToken: vi.fn(async () => "signed-session"),
}));
vi.mock("@clerk/react", () => ({
  useAuth: () => ({
    isLoaded: true,
    userId: "owner",
    isSignedIn: session.signedIn,
    getToken: session.getToken,
  }),
  SignInButton: ({ children }: any) => children,
  SignUpButton: ({ children }: any) => children,
  UserButton: () => <button>Minha conta</button>,
}));
const original = {
  id: "book1",
  title: "Programação Java",
  kind: "digital",
  format: "PDF",
  path: "/Livros/java.pdf",
  filename: "java.pdf",
  category: "Tecnologia / Java",
  metadata: { author: "Maria" },
  summary: "",
  cover_url: "",
  favorite: true,
  reading: true,
  version: 1,
};
let book = { ...original };
let catalogBooks: any[] = [];
let writes: any[] = [];
beforeEach(() => {
  session.signedIn = true;
  book = { ...original };
  catalogBooks = [book];
  writes = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, options: any = {}) => {
      let data: any = {};
      let status = 200;
      if (url.endsWith("/me"))
        data = {
          id: "owner",
          name: "Dona",
          email: "owner@example.com",
          enabled: true,
          admin: false,
          universal: false,
        };
      else if (url.endsWith("/catalogs"))
        data = [{ id: "catalog1", name: "Meu catálogo", owner: true }];
      else if (url.endsWith("/books/book1") && options.method === "PUT") {
        writes.push(JSON.parse(options.body));
        book = { ...JSON.parse(options.body), version: 2 };
        catalogBooks = catalogBooks.map((item) => item.id === book.id ? book : item);
        data = book;
      } else if (/\/books(?:\?|$)/.test(url)) {
        const parsed = new URL(url, "http://localhost");
        if (parsed.searchParams.has("limit")) {
          const offset = Number(parsed.searchParams.get("offset") || 0);
          const limit = Number(parsed.searchParams.get("limit") || 30);
          const kind = parsed.searchParams.get("kind");
          const shelf = parsed.searchParams.get("shelf");
          const category = parsed.searchParams.get("category") || "";
          const query = (parsed.searchParams.get("q") || "").normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase().split(/\s+/).filter(Boolean);
          const selected = catalogBooks.filter((item) =>
            (!kind || item.kind === kind) &&
            (!category || item.category === category || item.category.startsWith(category + "/")) &&
            (shelf === "favorites" ? item.favorite : shelf === "reading" ? item.reading : true) &&
            query.every((term) => `${item.title} ${item.category} ${item.path} ${JSON.stringify(item.metadata)} ${item.summary}`.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase().includes(term)),
          );
          data = {
            items: selected.slice(offset, offset + limit),
            total: selected.length,
            groups: selected.length,
            offset,
            limit,
            counts: {
              all: catalogBooks.filter((item) => !kind || item.kind === kind).length,
              favorites: catalogBooks.filter((item) => (!kind || item.kind === kind) && item.favorite).length,
              reading: catalogBooks.filter((item) => (!kind || item.kind === kind) && item.reading).length,
            },
          };
        } else data = catalogBooks;
      }
      else if (url.endsWith("/categories") || url.endsWith("/relations"))
        data = [];
      else if (url.endsWith("/comments"))
        data = [
          {
            id: "n1",
            body: "Uma ótima referência.",
            author: { name: "Maria", email: "maria@example.com" },
            created_at: "2026-10-26T12:00:00Z",
          },
        ];
      return {
        ok: status < 400,
        headers: new Headers(),
        status,
        json: async () => data,
      };
    }),
  );
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
describe("catálogo web", () => {
  it("não consulta dados sem autenticação", () => {
    session.signedIn = false;
    render(<App />);
    expect(screen.getByText("Criar conta")).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();
  });
  it("limita a lista a 30 livros por página", async () => {
    catalogBooks = Array.from({ length: 31 }, (_, index) => ({
      ...original,
      id: `book-${index + 1}`,
      title: `Livro ${index + 1}`,
      kind: "physical",
      format: "FISICO",
      metadata: {},
    }));
    const user = userEvent.setup();
    render(<App />);
    await screen.findByRole("heading", { name: "Livro 1" });
    expect(screen.getByRole("heading", { name: "Livro 30" })).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "Livro 31" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Próxima" }));
    expect(await screen.findByRole("heading", { name: "Livro 31" })).toBeTruthy();
    expect(screen.getByText("Página 2 de 2 · até 30 livros")).toBeTruthy();
    expect(fetch).toHaveBeenCalledWith(
      expect.stringContaining("/books?kind=physical&limit=30&offset=30"),
      expect.anything(),
    );
  });
  it("remove favorito do filtro assim que a alteração é salva", async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByRole("button", { name: "+ Livro digital" });
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Tipo de livro" }),
      "digital",
    );
    await screen.findByRole("heading", { name: "Programação Java" });
    await user.click(screen.getByRole("button", { name: /Favoritos/ }));
    await user.click(screen.getByRole("button", { name: "★ Favorito" }));
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: "Programação Java" }),
      ).toBeNull(),
    );
    expect(writes[0].favorite).toBe(false);
    expect(writes[0].version).toBe(1);
  });
  it("remove livro desmarcado do filtro de leitura", async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByRole("button", { name: "+ Livro digital" });
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Tipo de livro" }),
      "digital",
    );
    await screen.findByRole("heading", { name: "Programação Java" });
    await user.click(screen.getByRole("button", { name: /Estou lendo1/ }));
    await user.click(screen.getByRole("button", { name: "✓ Estou lendo" }));
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: "Programação Java" }),
      ).toBeNull(),
    );
    expect(writes[0].reading).toBe(false);
  });
  it("mostra caminho local sem link de leitura", async () => {
    render(<App />);
    await screen.findByRole("button", { name: "+ Livro digital" });
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Tipo de livro" }),
      "digital",
    );
    await screen.findByRole("heading", { name: "Programação Java" });
    expect(screen.getByText("/Livros/java.pdf")).toBeTruthy();
    expect(screen.queryByRole("link", { name: /Abrir/ })).toBeNull();
  });
  it("identifica o autor da anotação compartilhada", async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByRole("button", { name: "+ Livro digital" });
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Tipo de livro" }),
      "digital",
    );
    await screen.findByRole("heading", { name: "Programação Java" });
    await user.click(screen.getByRole("button", { name: "Anotações" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Maria")).toBeTruthy();
    expect(within(dialog).getByText(/maria@example.com/)).toBeTruthy();
    expect(within(dialog).getByText("Uma ótima referência.")).toBeTruthy();
  });
});
