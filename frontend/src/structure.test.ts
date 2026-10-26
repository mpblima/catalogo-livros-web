import { expect, it } from "vitest";
import { groupBooks } from "./structure";
import type { Book } from "./book";
const book = (id: string, format = "PDF") =>
  ({ id, title: id, format, kind: "digital" }) as Book;
it("mantém os cadastros e agrupa transitivamente com a amostra por último", () => {
  const books = [book("a"), book("b", "EPUB"), book("sample")];
  const groups = groupBooks(books, [
    { book_id: "a", related_book_id: "b", kind: "same_work" },
    { book_id: "sample", related_book_id: "b", kind: "sample_of" },
  ]);
  expect(groups).toHaveLength(1);
  expect(groups[0].books.map((b) => b.id)).toEqual(["a", "b", "sample"]);
  expect(books).toHaveLength(3);
});
it("ignora relações fora do filtro sem esconder livros visíveis", () => {
  const groups = groupBooks(
    [book("a")],
    [{ book_id: "a", related_book_id: "outside", kind: "same_work" }],
  );
  expect(groups[0].books.map((b) => b.id)).toEqual(["a"]);
});
