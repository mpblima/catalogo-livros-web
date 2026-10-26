import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
export default defineConfig({
  plugins: [
    react(),
    {
      name: "matching-backend-release",
      async buildStart() {
        if (process.env.VERCEL_ENV !== "production") return;
        const revision = process.env.VERCEL_GIT_COMMIT_SHA;
        const api = process.env.VITE_API_URL;
        if (
          !revision ||
          !/^[a-f0-9]{40}$/.test(revision) ||
          !api ||
          !api.startsWith("https://")
        )
          throw new Error(
            "Production requires a Git revision and HTTPS backend URL.",
          );
        const deadline = Date.now() + 12 * 60 * 1000;
        while (Date.now() < deadline) {
          try {
            const response = await fetch(api + "/healthz", {
              signal: AbortSignal.timeout(12000),
              cache: "no-store",
            });
            const health = await response.json();
            if (
              response.ok &&
              health.status === "ok" &&
              health.revision === revision
            )
              return;
          } catch {
            /* A cold or deploying instance is retried within the bounded window. */
          }
          console.log("Waiting for the matching healthy backend revision…");
          await new Promise((resolve) => setTimeout(resolve, 15000));
        }
        throw new Error(
          "The matching backend revision was not deployed. Check GitHub Actions and Render.",
        );
      },
      generateBundle() {
        this.emitFile({
          type: "asset",
          fileName: "release.json",
          source: JSON.stringify({
            revision:
              process.env.VERCEL_GIT_COMMIT_SHA ||
              process.env.GITHUB_SHA ||
              "local",
          }),
        });
      },
    },
  ],
});
