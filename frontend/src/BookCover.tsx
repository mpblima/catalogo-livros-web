import { useAuth } from "@clerk/react";
import { useEffect, useRef, useState } from "react";
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
  const [visible, setVisible] = useState(false);
  const placeholder = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    const element = placeholder.current;
    if (!element) return;
    if (!("IntersectionObserver" in window)) {
      setVisible(true);
      return;
    }
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "300px 0px" },
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, [catalog, id]);
  useEffect(() => {
    setLocal("");
    if (!visible || !catalog || !id) return;
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
            "/cover?v=" +
            encodeURIComponent(version),
          { headers: { Authorization: "Bearer " + token } },
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
  }, [catalog, id, version, getToken, visible]);
  return local ? (
    <img src={local} alt={"Capa de " + title} loading="lazy" decoding="async" />
  ) : (
    <span ref={placeholder} aria-label={visible ? "Carregando capa" : "Capa carregada quando aparecer na tela"}>
      {visible ? "Carregando capa…" : ""}
    </span>
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
        loading="lazy"
        decoding="async"
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
