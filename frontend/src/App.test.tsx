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
let writes: any[] = [];
beforeEach(() => {
  session.signedIn = true;
  book = { ...original };
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
        data = book;
      } else if (/\/books(?:\?|$)/.test(url)) data = [book];
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
