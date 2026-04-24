import { JoinRouteScreen } from "./join-route-screen";

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
      <JoinRouteScreen code={routeCode} />
    </main>
  );
}
