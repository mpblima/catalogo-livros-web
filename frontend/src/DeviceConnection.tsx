import { useEffect, useState } from "react";
type Api = <T>(path: string, method?: string, body?: unknown) => Promise<T>;
export function DeviceConnection({
  pair,
  catalogs,
  api,
  onClose,
}: {
  pair: string;
  catalogs: { id: string; name: string; owner: boolean }[];
  api: Api;
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [catalog, setCatalog] = useState(
    catalogs.find((c) => c.owner)?.id || "",
  );
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    let cancelled = false;
    api<{ device_name: string }>("/devices/pair/" + encodeURIComponent(pair))
      .then((data) => {
        if (!cancelled) setName(data.device_name);
      })
      .catch((e) => {
        if (!cancelled) setError(e.message);
      });
    return () => {
      cancelled = true;
    };
  }, [pair, api]);
  return (
    <div className="overlay">
      <section
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="device-title"
      >
        <button className="close" aria-label="Fechar conexão" onClick={onClose}>
          ×
        </button>
        <h2 id="device-title">Conectar seu catálogo desktop</h2>
        {done ? (
          <>
            <p>
              Conexão autorizada. Volte ao desktop e use “Sincronizar catálogo”
              quando quiser enviar e receber alterações.
            </p>
            <button className="primary" onClick={onClose}>
              Concluir
            </button>
          </>
        ) : (
          <>
            <p>
              {name
                ? "Dispositivo: " + name
                : "Consultando a solicitação de conexão…"}
            </p>
            <p>
              Digite o código exibido no aplicativo desktop. A conexão permite
              sincronizar o catálogo escolhido; os arquivos dos livros e a
              posição do leitor permanecem no computador.
            </p>
            <form
              onSubmit={async (e) => {
                e.preventDefault();
                setBusy(true);
                setError("");
                try {
                  await api(
                    "/devices/pair/" + encodeURIComponent(pair) + "/approve",
                    "POST",
                    { user_code: code, catalog_id: catalog },
                  );
                  setDone(true);
                } catch (e) {
                  setError((e as Error).message);
                } finally {
                  setBusy(false);
                }
              }}
            >
              <label>
                Código exibido no desktop
                <input
                  autoComplete="off"
                  required
                  maxLength={8}
                  value={code}
                  onChange={(e) => setCode(e.target.value.toUpperCase())}
                />
              </label>
              <label>
                Seu catálogo
                <select
                  required
                  value={catalog}
                  onChange={(e) => setCatalog(e.target.value)}
                >
                  {catalogs
                    .filter((c) => c.owner)
                    .map((c) => (
                      <option value={c.id} key={c.id}>
                        {c.name}
                      </option>
                    ))}
                </select>
              </label>
              <button className="primary" disabled={busy || !name || !catalog}>
                Autorizar este desktop
              </button>
            </form>
          </>
        )}
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
      </section>
    </div>
  );
}
export function Devices({ api, onClose }: { api: Api; onClose: () => void }) {
  const [devices, setDevices] = useState<
    { id: string; name: string; expires_at: string; revoked: boolean }[]
  >([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let cancelled = false;
    api<typeof devices>("/devices")
      .then((data) => {
        if (!cancelled) setDevices(data);
      })
      .catch((e) => {
        if (!cancelled) setError(e.message);
      });
    return () => {
      cancelled = true;
    };
  }, [api]);
  return (
    <div className="overlay">
      <section
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="devices-title"
      >
        <button
          className="close"
          aria-label="Fechar dispositivos"
          onClick={onClose}
        >
          ×
        </button>
        <h2 id="devices-title">Dispositivos conectados</h2>
        <p>
          A conexão expira em até 30 dias e pode ser revogada a qualquer
          momento. Revogar mantém os livros e arquivos já existentes no desktop.
        </p>
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <ul className="access-list">
          {devices.map((d) => (
            <li key={d.id}>
              <span>
                {d.name}
                <small>
                  {d.revoked
                    ? "Revogado"
                    : "Válido até " +
                      new Date(d.expires_at).toLocaleDateString("pt-BR")}
                </small>
              </span>
              {!d.revoked && (
                <button
                  disabled={busy}
                  onClick={async () => {
                    if (!window.confirm("Desconectar " + d.name + "?")) return;
                    setBusy(true);
                    setError("");
                    try {
                      await api("/devices/" + d.id, "DELETE");
                      setDevices(await api<typeof devices>("/devices"));
                    } catch (e) {
                      setError((e as Error).message);
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  Revogar acesso
                </button>
              )}
            </li>
          ))}
        </ul>
        {!devices.length && (
          <p>
            Nenhum desktop conectado. Inicie a conexão pelo aplicativo desktop.
          </p>
        )}
      </section>
    </div>
  );
}
