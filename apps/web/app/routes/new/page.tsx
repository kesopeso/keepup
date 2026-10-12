import type { Metadata } from "next";
import { CreateRouteForm } from "./create-route-form";

export const metadata: Metadata = {
  title: "Create a route | KeepUp",
};

export default function NewRoutePage() {
  return (
    <main className="page-shell">
      <CreateRouteForm />
    </main>
  );
}
