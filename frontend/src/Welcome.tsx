import { PrivacyNotice } from "./PrivacyNotice";
import { SignInButton, SignUpButton } from "@clerk/react";
import "./welcome.css";

export function Welcome() {
  return (
    <main className="welcome">
      <div className="welcome-shell">
        <div className="welcome-nav">
          <div className="brand">
            ▤ <span>Catálogo de Livros</span>
          </div>
          <a href="#sua-biblioteca">Conheça a biblioteca</a>
        </div>
        <section className="welcome-hero" aria-labelledby="welcome-title">
          <div className="welcome-copy">
            <p className="eyebrow">SEU ACERVO PESSOAL, AGORA COMPARTILHADO</p>
            <h1 id="welcome-title">
              Uma casa para
              <br />
              todos os seus livros.
            </h1>
            <p>
              Organize livros físicos e digitais, compartilhe seu catálogo e
              guarde as ideias de cada leitura.
            </p>
            <div className="actions">
              <SignUpButton mode="modal">
                <button className="welcome-signup">Criar conta</button>
              </SignUpButton>
              <SignInButton mode="modal">
                <button className="welcome-signin">Já tenho conta</button>
              </SignInButton>
            </div>
          </div>
          <div className="welcome-image">
            <img
              src="/images/biblioteca.webp"
              alt="Livros sobre uma mesa, com um exemplar aberto e um marcador verde"
              width="1122"
              height="1402"
              fetchPriority="high"
            />
          </div>
        </section>
        <section
          className="welcome-features"
          id="sua-biblioteca"
          aria-label="Sua biblioteca, do seu jeito"
        >
          <div>
            <h2>Da estante ao catálogo.</h2>
            <p>
              Livros físicos e digitais, organizados por assunto e sempre fáceis
              de encontrar.
            </p>
          </div>
          <div>
            <h2>As ideias ficam com você.</h2>
            <p>
              Marque favoritos, reúna anotações e acompanhe os livros que está
              lendo.
            </p>
          </div>
          <div>
            <h2>Boas leituras se compartilham.</h2>
            <p>
              Convide outras pessoas para conhecer seu catálogo e trocar
              anotações.
            </p>
          </div>
        </section>
        <div className="welcome-foot">
          <PrivacyNotice />
          <span>Uma biblioteca com a sua história.</span>
          <span>Seus arquivos digitais continuam no seu computador.</span>
        </div>
      </div>
    </main>
  );
}
