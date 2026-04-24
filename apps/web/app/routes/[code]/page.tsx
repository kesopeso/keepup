import { RouteAuthStatus } from "./route-auth-status";

type RoutePageProps = {
  params: Promise<{
    code: string;
  }>;
};

export default async function RoutePage({ params }: RoutePageProps) {
  const { code } = await params;
  const routeCode = code.toUpperCase();

  return (
    <main className="page-shell">
      <section className="route-shell">
        <div className="form-header">
          <p className="eyebrow">Route</p>
          <h1>{routeCode}</h1>
        </div>
        <RouteAuthStatus code={routeCode} />
      </section>
    </main>
  );
}
