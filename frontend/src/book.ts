export type Book = {
  id: string;
  title: string;
  kind: "physical" | "digital";
  format: string;
  path: string;
  filename: string;
  category: string;
  metadata: Record<string, unknown>;
  summary: string;
  cover_url: string;
  favorite: boolean;
  reading: boolean;
  version: number;
};
