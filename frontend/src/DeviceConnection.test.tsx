// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeviceConnection, Devices } from "./DeviceConnection";
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
it("authorizes only the selected owned catalog after entering the desktop code", async () => {
  const api = vi.fn(async (path: string, method?: string) =>
    method ? {} : { device_name: "Meu desktop" },
  );
  render(
    <DeviceConnection
      pair="pair1"
      catalogs={[
        { id: "owned", name: "Meu catálogo", owner: true },
        { id: "shared", name: "Recebido", owner: false },
      ]}
      api={api as any}
      onClose={() => {}}
    />,
  );
  await screen.findByText("Dispositivo: Meu desktop");
  expect(screen.queryByRole("option", { name: "Recebido" })).toBeNull();
  await userEvent.type(
    screen.getByLabelText("Código exibido no desktop"),
    "abcd1234",
  );
  await userEvent.click(
    screen.getByRole("button", { name: "Autorizar este desktop" }),
  );
  await screen.findByText(/Conexão autorizada/);
  expect(api).toHaveBeenCalledWith("/devices/pair/pair1/approve", "POST", {
    user_code: "ABCD1234",
    catalog_id: "owned",
  });
});
it("revokes a desktop and refreshes its status", async () => {
  let revoked = false;
  const api = vi.fn(async (path: string, method?: string) => {
    if (method === "DELETE") {
      revoked = true;
      return {};
    }
    return [{ id: "d1", name: "Notebook", expires_at: "2026-11-01", revoked }];
  });
  vi.spyOn(window, "confirm").mockReturnValue(true);
  render(<Devices api={api as any} onClose={() => {}} />);
  await screen.findByText("Notebook");
  await userEvent.click(screen.getByRole("button", { name: "Revogar acesso" }));
  await waitFor(() =>
    expect(api).toHaveBeenCalledWith("/devices/d1", "DELETE"),
  );
  await screen.findByText("Revogado");
  expect(screen.queryByRole("button", { name: "Revogar acesso" })).toBeNull();
});
