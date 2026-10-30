import { PrivacyNotice } from "./PrivacyNotice";
import { SignInButton, SignUpButton } from "@clerk/react";
import "./welcome.css";

const desktopDownloads = "https://mpblima.github.io/catalogo-livros/#downloads";

export function Welcome() {
  return (
    <main className="welcome">
      <div className="welcome-shell">
        <header className="welcome-nav">
          <a className="brand" href="#inicio" aria-label="Catálogo de Livros, início">
            <img src="/praesens-logo-escuro.svg" alt="Praesens.tech" />
          </a>
          <nav aria-label="Navegação principal">
            <a href="#recursos">Recursos</a>
            <a href="#web-desktop">Web e desktop</a>
            <a href="#perguntas">Perguntas</a>
            <a className="nav-download" href={desktopDownloads}>Versão desktop ↗</a>
          </nav>
        </header>

        <section className="welcome-hero" id="inicio" aria-labelledby="welcome-title">
          <div className="welcome-copy">
            <p className="eyebrow">UM CATÁLOGO PARA LER, LEMBRAR E COMPARTILHAR</p>
            <h1 id="welcome-title">Seus livros em ordem.<br /><em>Suas ideias por perto.</em></h1>
            <p className="hero-description">
              Reúna livros físicos e digitais, encontre qualquer título pela busca
              e compartilhe o catálogo e as anotações com quem lê com você.
            </p>
            <div className="actions">
              <SignUpButton mode="modal">
                <button className="welcome-signup">Criar minha conta</button>
              </SignUpButton>
              <SignInButton mode="modal">
                <button className="welcome-signin">Entrar</button>
              </SignInButton>
            </div>
            <p className="hero-note">Acesse pelo navegador. Convide outras pessoas quando quiser.</p>
          </div>
          <figure className="welcome-image">
            <img
              src="/images/biblioteca.webp"
              alt="Livros sobre uma mesa, com um exemplar aberto e um marcador verde"
              width="1122"
              height="1402"
              fetchPriority="high"
            />
            <figcaption>Um lugar para o acervo e para as ideias que cada leitura deixa.</figcaption>
          </figure>
        </section>

        <section className="welcome-facts" aria-label="O que você encontra no catálogo">
          <div><strong>Físicos + digitais</strong><span>Um catálogo para os livros da estante e os arquivos cadastrados.</span></div>
          <div><strong>Busca em todo o acervo</strong><span>Pesquise títulos, autores, assuntos e anotações.</span></div>
          <div><strong>Anotações compartilhadas</strong><span>Quem participa do catálogo acompanha as mesmas ideias.</span></div>
        </section>

        <section className="welcome-section" id="recursos" aria-labelledby="resources-title">
          <div className="section-heading">
            <p className="eyebrow">DO PRIMEIRO CADASTRO À PRÓXIMA LEITURA</p>
            <h2 id="resources-title">Mais do que uma lista<br />de títulos.</h2>
            <p>Veja o que você pode fazer no catálogo web e como ele acompanha seu jeito de ler.</p>
          </div>
          <div className="feature-grid">
            <article className="feature feature-large">
              <span className="feature-number">01</span>
              <h3>Encontre um livro sem lembrar onde guardou.</h3>
              <p>Pesquise no acervo por título, autoria, editora, assunto, categoria e anotações. Use filtros para percorrer livros físicos e digitais.</p>
              <div className="search-example"><span>⌕</span><span>autor, assunto ou uma frase anotada</span><kbd>Buscar</kbd></div>
            </article>
            <article className="feature">
              <span className="feature-number">02</span>
              <h3>Deixe o acervo com a sua cara.</h3>
              <p>Organize por categorias, acompanhe favoritos e leituras em andamento e relacione edições do mesmo livro.</p>
            </article>
            <article className="feature feature-note">
              <span className="feature-number">03</span>
              <h3>Uma conversa por livro.</h3>
              <p>As anotações ficam reunidas em um só espaço. Quando o catálogo é compartilhado, cada participante pode ler e acrescentar contribuições que todos veem.</p>
            </article>
          </div>
        </section>

        <section className="web-desktop" id="web-desktop" aria-labelledby="web-desktop-title">
          <div className="web-desktop-copy">
            <p className="eyebrow">UM CATÁLOGO, DOIS JEITOS DE USAR</p>
            <h2 id="web-desktop-title">Consulte na web.<br /><em>Organize no desktop.</em></h2>
            <p>Use o navegador para consultar, pesquisar, cadastrar e compartilhar livros e anotações. No computador, o aplicativo desktop também trabalha com os arquivos PDF e EPUB que ficam armazenados localmente.</p>
            <ul>
              <li>O desktop pode catalogar arquivos digitais e ajudar a encontrar livros na estante.</li>
              <li>Abra PDFs no aplicativo e mantenha o ponto de leitura salvo no computador.</li>
              <li>Ao conectar um catálogo, seus cadastros e anotações podem ser sincronizados com a web.</li>
            </ul>
            <a className="desktop-download" href={desktopDownloads}>
              Conhecer a versão desktop <span>Windows, macOS e Linux · versões, arquivos e instruções de instalação</span>
            </a>
          </div>
          <div className="web-desktop-aside">
            <img className="aside-mark" src="/praesens-simbolo.svg" alt="" />
            <h3>Seus arquivos continuam no seu computador.</h3>
            <p>A sincronização do catálogo leva informações dos livros e anotações. Os arquivos digitais, o OCR e o progresso local de leitura permanecem no desktop.</p>
            <a href={desktopDownloads}>Ver downloads e detalhes da versão ↗</a>
          </div>
        </section>

        <section className="welcome-steps" aria-labelledby="steps-title">
          <div className="section-heading">
            <p className="eyebrow">COMECE PELO QUE VOCÊ JÁ LÊ</p>
            <h2 id="steps-title">Sua biblioteca ganha forma<br />a cada livro.</h2>
          </div>
          <ol>
            <li><span>1</span><div><strong>Crie sua conta</strong><p>Entre pelo navegador e crie seu primeiro catálogo.</p></div></li>
            <li><span>2</span><div><strong>Cadastre e organize</strong><p>Adicione livros físicos e digitais, revise os dados e monte suas categorias.</p></div></li>
            <li><span>3</span><div><strong>Compartilhe quando fizer sentido</strong><p>Convide alguém para consultar o catálogo e participar das anotações dos livros.</p></div></li>
          </ol>
        </section>

        <section className="welcome-faq" id="perguntas" aria-labelledby="faq-title">
          <div><p className="eyebrow">ANTES DE COMEÇAR</p><h2 id="faq-title">Dúvidas frequentes</h2></div>
          <div className="faq-items">
            <details><summary>Preciso instalar algo para usar a versão web?</summary><p>Não. Você acessa o catálogo pelo navegador. O aplicativo desktop é uma opção para organizar e abrir arquivos que ficam no computador.</p></details>
            <details><summary>Os livros digitais são enviados para a nuvem?</summary><p>Não. A sincronização compartilha informações do catálogo e anotações. Os arquivos PDF e EPUB continuam no dispositivo desktop.</p></details>
            <details><summary>Quem pode ler as anotações?</summary><p>Em um catálogo sem compartilhamento, elas ficam no seu catálogo. Quando você convida alguém, as anotações daquele catálogo são compartilhadas entre os participantes.</p></details>
          </div>
        </section>

        <section className="welcome-closing" aria-label="Começar a usar">
          <div><p className="eyebrow">PRONTO PARA ORGANIZAR?</p><h2>Comece com o próximo livro.</h2><p>Crie seu catálogo na web ou conheça o aplicativo para computador.</p></div>
          <div className="closing-actions">
            <SignUpButton mode="modal"><button className="welcome-signup">Criar conta</button></SignUpButton>
            <a href={desktopDownloads}>Downloads desktop ↗</a>
          </div>
        </section>

        <footer className="welcome-foot">
          <PrivacyNotice />
          <span>Uma biblioteca com a sua história.</span>
          <a href={desktopDownloads}>Versão desktop: downloads e instalação ↗</a>
        </footer>
      </div>
    </main>
  );
}
