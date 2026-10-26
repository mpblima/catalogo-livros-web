import { useAuth } from "@clerk/react";
import { useEffect, useState } from "react";
type Props = {
  url: string;
  stored: boolean;
  catalog: string;
  id: string;
  version: number;
  title: string;
  onUnavailable?: () => void;
};
function StoredCover({ catalog, id, version, title, onUnavailable }: Props) {
  const { getToken } = useAuth();
  const [local, setLocal] = useState("");
  useEffect(() => {
    setLocal("");
    if (!catalog || !id) return;
    let cancelled = false;
    let objectURL = "";
    (async () => {
      try {
        const token = await getToken();
        const response = await fetch(
          (import.meta.env.VITE_API_URL || "") +
            "/api/catalogs/" +
            catalog +
            "/books/" +
            id +
            "/cover",
          { headers: { Authorization: "Bearer " + token }, cache: "no-store" },
        );
        if (!response.ok) throw Error("Capa indisponível");
        const blob = await response.blob();
        if (cancelled) return;
        objectURL = URL.createObjectURL(blob);
        setLocal(objectURL);
      } catch {
        if (!cancelled) onUnavailable?.();
      }
    })();
    return () => {
      cancelled = true;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [catalog, id, version, getToken]);
  return local ? (
    <img src={local} alt={"Capa de " + title} />
  ) : (
    <span>Sem capa</span>
  );
}
export function BookCover(props: Props) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [props.url, props.id, props.version]);
  if (props.url && !failed)
    return (
      <img
        src={props.url}
        alt={"Capa de " + props.title}
        referrerPolicy="no-referrer"
        onError={() => {
          setFailed(true);
          props.onUnavailable?.();
        }}
      />
    );
  if (props.stored) return <StoredCover {...props} />;
  return <span>Sem capa</span>;
}
