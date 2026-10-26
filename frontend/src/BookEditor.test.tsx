// @vitest-environment jsdom
import { useState } from "react";
import { afterEach, describe, it, expect, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BookEditor, normalizeISBN } from "./BookEditor";
import type { Book } from "./book";
const empty: Book = {
  id: "",
  title: "",
  kind: "physical",
  format: "FISICO",
  path: "",
  filename: "",
  category: "",
  metadata: {},
  summary: "",
  cover_url: "",
  favorite: false,
  reading: false,
  version: 0,
};
const found: Book = {
  ...empty,
  title: "Effective Java",
  metadata: {
    isbn: "9780134685991",
    subtitle: "Third edition",
    author: "Joshua Bloch",
    pages: "416",
    publisher: "Addison-Wesley",
    language: "en",
    subject: "Java",
  },
  summary: "A reference",
  cover_url: "https://example.com/cover.jpg",
};
function Form({
  lookup,
  onSave,
  initial = empty,
}: {
  lookup: (isbn: string) => Promise<Book>;
  onSave: (b: Book) => void;
  initial?: Book;
}) {
  const [book, setBook] = useState(initial);
  return (
    <BookEditor
      book={book}
      busy={false}
      lookup={lookup}
      onSave={onSave}
      onChange={setBook}
      onClose={() => {}}
      onDelete={() => {}}
    />
  );
}
afterEach(cleanup);
describe("cadastro equivalente ao desktop", () => {
  it("valida dígitos e normaliza ISBN-10 e extensão EAN", () => {
    expect(normalizeISBN("0134685997")).toBe("9780134685991");
    expect(normalizeISBN("978013468599112345")).toBe("9780134685991");
    expect(normalizeISBN("9780134685992")).toBe("");
  });
  it("pesquisa automaticamente e exige salvar após revisão", async () => {
    const lookup = vi.fn(async () => found),
      save = vi.fn();
    render(<Form lookup={lookup} onSave={save} />);
    expect(document.activeElement).toBe(screen.getByLabelText(/ISBN \/ ID/));
    fireEvent.change(screen.getByLabelText(/ISBN \/ ID/), {
      target: { value: "9780134685991" },
    });
    await waitFor(() =>
      expect(screen.getByLabelText("Título *")).toHaveProperty(
        "value",
        "Effective Java",
      ),
    );
    expect(lookup).toHaveBeenCalledTimes(1);
    expect(save).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Autor(es)")).toHaveProperty(
      "value",
      "Joshua Bloch",
    );
    await userEvent.click(screen.getByRole("button", { name: "Salvar livro" }));
    expect(save.mock.calls[0][0].metadata.pages).toBe("416");
    expect(save.mock.calls[0][0].category).toMatch(
      /livros_fisicos\/programacao\/java/,
    );
  });
  it("sugere categoria pelo assunto e preserva uma personalização", async () => {
    const save = vi.fn();
    render(<Form lookup={vi.fn()} onSave={save} />);
    const category = screen.getByLabelText(/Categoria/);
    fireEvent.change(screen.getByLabelText("Assunto"), {
      target: { value: "Java" },
    });
    expect(category).toHaveProperty(
      "value",
      "livros_fisicos/programacao/java/fundamentos_e_referencia",
    );
    fireEvent.change(category, { target: { value: "Minha categoria" } });
    fireEvent.change(screen.getByLabelText("Assunto"), {
      target: { value: "Literatura" },
    });
    expect(category).toHaveProperty("value", "Minha categoria");
    fireEvent.change(screen.getByLabelText("Título *"), {
      target: { value: "Livro de teste" },
    });
    await userEvent.click(screen.getByRole("button", { name: "Salvar livro" }));
    expect(save.mock.calls[0][0].category).toBe("Minha categoria");
  });
  it("ignora resposta atrasada após trocar o código", async () => {
    let finish!: (b: Book) => void;
    const lookup = vi.fn(
      () => new Promise<Book>((resolve) => (finish = resolve)),
    );
    render(<Form lookup={lookup} onSave={() => {}} />);
    const input = screen.getByLabelText(/ISBN \/ ID/);
    fireEvent.change(input, { target: { value: "9780134685991" } });
    await waitFor(() => expect(lookup).toHaveBeenCalled());
    fireEvent.change(input, { target: { value: "ID-local" } });
    finish(found);
    await waitFor(() =>
      expect(screen.getByLabelText("Título *")).toHaveProperty("value", ""),
    );
  });
  it("preserva campos digitais e permite editar metadados extras", async () => {
    const save = vi.fn();
    render(
      <Form
        initial={{
          ...empty,
          id: "digital",
          kind: "digital",
          title: "Livro digital",
          filename: "livro.epub",
          path: "/Livros/livro.epub",
          format: "EPUB",
          metadata: {
            vendor: { id: 7 },
            page: 12,
            notes: "Anotação",
            subtitle: "Subtítulo",
          },
        }}
        lookup={vi.fn()}
        onSave={save}
      />,
    );
    expect(screen.getByLabelText("Subtítulo")).toHaveProperty(
      "value",
      "Subtítulo",
    );
    expect(screen.getByLabelText("Caminho físico do arquivo")).toHaveProperty(
      "value",
      "/Livros/livro.epub",
    );
    expect(screen.queryByLabelText("Página atual")).toBeNull();
    await userEvent.click(
      screen.getByRole("tab", { name: "Outros metadados" }),
    );
    fireEvent.change(screen.getByLabelText("Outros metadados (JSON)"), {
      target: { value: '{"vendor":{"id":8},"custom":"novo"}' },
    });
    await userEvent.click(screen.getByRole("button", { name: "Salvar livro" }));
    expect(save.mock.calls[0][0].metadata).toMatchObject({
      vendor: { id: 8 },
      custom: "novo",
      page: 12,
      notes: "Anotação",
    });
    expect(save.mock.calls[0][0].format).toBe("EPUB");
  });
  it("falha na consulta mantém o preenchimento manual disponível", async () => {
    render(
      <Form
        lookup={async () => {
          throw Error("Serviço temporariamente indisponível");
        }}
        onSave={() => {}}
      />,
    );
    fireEvent.change(screen.getByLabelText(/ISBN \/ ID/), {
      target: { value: "9780134685991" },
    });
    await screen.findByRole("alert");
    expect(
      (screen.getByLabelText("Título *") as HTMLInputElement).disabled,
    ).toBe(false);
  });
});
