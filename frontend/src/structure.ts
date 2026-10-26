import type { Book } from "./book";
export type Relation = {
  book_id: string;
  related_book_id: string;
  kind: "same_work" | "sample_of";
  source?: string;
};
export function groupBooks(books: Book[], relations: Relation[]) {
  const parent = new Map(books.map((b) => [b.id, b.id]));
  const samples = new Set<string>();
  const root = (id: string): string => {
    const p = parent.get(id)!;
    if (p === id) return id;
    const result = root(p);
    parent.set(id, result);
    return result;
  };
  for (const relation of relations) {
    if (!parent.has(relation.book_id) || !parent.has(relation.related_book_id))
      continue;
    if (relation.kind === "sample_of") samples.add(relation.book_id);
    parent.set(root(relation.book_id), root(relation.related_book_id));
  }
  const groups = new Map<string, Book[]>();
  for (const book of books) {
    const id = root(book.id);
    groups.set(id, [...(groups.get(id) || []), book]);
  }
  return [...groups].map(([id, items]) => ({
    id,
    books: items.sort(
      (a, b) =>
        Number(samples.has(a.id)) - Number(samples.has(b.id)) ||
        Number(b.format === "PDF") - Number(a.format === "PDF") ||
        a.title.localeCompare(b.title),
    ),
    samples,
  }));
}
