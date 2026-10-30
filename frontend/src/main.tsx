import React from "react";
import { createRoot } from "react-dom/client";
import { ClerkProvider } from "@clerk/react";
import App from "./App";
import "./style.css";
const key = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY;
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    {key ? (
      <ClerkProvider publishableKey={key}>
        <App />
      </ClerkProvider>
    ) : (
      <main className="setup">
        <p className="eyebrow">CATÁLOGO DE LIVROS</p>
        <h1>Sua biblioteca, conectada.</h1>
        <p>Configure a autenticação para iniciar o serviço.</p>
        <p>As instruções de instalação estão no README do projeto.</p>
      </main>
    )}
  </React.StrictMode>,
);
