import rules from "./classificacao.json";
const fold = (s: string) =>
  s
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase();
const aliases: Record<string, string> = {
  JavaScript: "java_script",
  TypeScript: "type_script",
  "DevOps e Infraestrutura": "dev_ops_e_infraestrutura",
  APIs: "ap_is",
  GraphQL: "graph_ql",
  NoSQL: "no_sql",
  MongoDB: "mongo_db",
  MySQL: "my_sql",
  PostgreSQL: "postgre_sql",
  OpenStack: "open_stack",
  OpenShift: "open_shift",
  "Git e GitHub": "git_e_git_hub",
  "C# e .NET": "csharp_e_dotnet",
  "C e C++": "c_e_cpp",
  "iOS e Swift": "i_os_e_swift",
  "Eletrônica e IoT": "eletronica_e_io_t",
  OCaml: "o_caml",
};
const snake = (s: string) =>
  fold(s)
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "") || "sem_nome";
// Rules and ordering mirror the desktop CategoryClassifier.
export function classify(title: string, subject: string) {
  const actual = fold(title);
  let category = "";
  const extra = [
    ["\\brust\\b", "programacao/rust"],
    ["\\bkotlin\\b", "programacao/kotlin"],
    ["\\btauri\\b", "desenvolvimento_desktop/tauri"],
    ["\\babap\\b", "programacao/abap"],
    [
      "\\bterraform\\b",
      "dev_ops_e_infraestrutura/infraestrutura_como_codigo_e_automacao/terraform",
    ],
    ["\\bguitarra?\\b|\\bguitar\\b", "musica/guitarra"],
    ["\\bviolao\\b", "musica/violao"],
    [
      "\\barchimate\\b|\\btogaf\\b",
      "arquitetura_e_integracao/arquitetura_empresarial",
    ],
  ];
  for (const [pattern, destination] of extra)
    if (new RegExp(pattern).test(actual))
      return `livros_fisicos/${destination}`;
  if (actual.includes("cassandra"))
    return "livros_fisicos/dados_e_bancos_de_dados/no_sql/cassandra";
  if (actual.includes("text adventure") && actual.includes("python"))
    return "livros_fisicos/desenvolvimento_de_jogos/python";
  for (const text of [title, subject.replace(/_/g, " ")]) {
    const rule = rules.find(([pattern]) =>
      new RegExp(pattern).test(fold(text)),
    );
    if (rule) {
      category = rule[1];
      break;
    }
  }
  if (!category) {
    const sciences = [
      [
        "devotional|devociona|reflexoes diarias|meditacoes diarias",
        "Religião e Espiritualidade/Devocionais",
      ],
      [
        "biblia|biblical|bible|teologia|theolog|christian|cristian|espiritualidade",
        "Religião e Espiritualidade/Estudos bíblicos e teologia",
      ],
      ["astronom|astrophys|cosmolog", "Ciências/Astronomia"],
      ["quantum|quantica|quantico", "Ciências/Computação quântica"],
      ["fisica|physics", "Ciências/Física"],
      ["biolog|genetic|bioinform", "Ciências/Biologia"],
      ["quimica|chemistry", "Ciências/Química"],
      ["economics|economia", "Gestão e Negócios/Economia"],
      ["financ|investiment", "Gestão e Negócios/Finanças"],
      ["culinaria|cozinha|cooking", "Conhecimentos Gerais/Culinária"],
      [
        "dostoi|tolstoi|machado de assis|jane austen|literatura|romance|novel",
        "Literatura/Ficção e poesia",
      ],
    ];
    category =
      sciences.find(([pattern]) =>
        new RegExp(pattern).test(actual + " " + fold(subject)),
      )?.[1] || "";
    if (!category) {
      category =
        subject.length >= 3 &&
        subject.length <= 90 &&
        /^[\p{L}\p{N}\s,;&()\-]+$/u.test(subject) &&
        !/(www|http|allitebooks|ebooksworld|microsoft|powerpoint|untitled)/.test(
          fold(subject),
        )
          ? "Conhecimentos Gerais/" + subject.split(/[;,]/)[0]
          : "a_classificar";
    }
  }
  if (category === "Programação/Python" || category === "Programação/Java") {
    const topics = [
      [
        "concurr|parallel|performance|clustering|distributed",
        "Concorrência e desempenho",
      ],
      ["data.struct|algorithm|algoritmo", "Algoritmos e estruturas de dados"],
      [
        "design.pattern|object.oriented|orientacao|clean|best.practices",
        "Orientação a objetos e boas práticas",
      ],
      ["django|requests|scraping|scrapping|web", "Web e extração de dados"],
      ["gui|wxpython|graphics|javafx", "Interfaces gráficas"],
    ];
    category +=
      "/" +
      (topics.find(([p]) => new RegExp(p).test(actual))?.[1] ||
        "Fundamentos e referência");
  }
  return (
    "livros_fisicos/" +
    category
      .split("/")
      .map((s) => aliases[s] || snake(s))
      .join("/")
  );
}
