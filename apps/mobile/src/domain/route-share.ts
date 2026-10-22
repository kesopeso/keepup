// Public links contain only the route code. The configured API origin is the fallback.
export function routeShareUrl(code: string, apiBase: string, publicOrigin?: string) {
  if (!/^[A-Z0-9]{6}$/.test(code)) throw new Error('Invalid route code');
  const url = new URL(publicOrigin || apiBase);
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) throw new Error('Invalid public route origin');
  const prefix = publicOrigin ? '' : url.pathname.replace(/\/+$/, '').replace(/\/api$/, '');
  url.pathname = `${prefix}/routes/${code}`; url.search = ''; url.hash = '';
  return url.toString();
}
export async function shareRouteLink(url: string, share: (message: { message: string; url: string }) => Promise<unknown>) {
  try { await share({ message: url, url }); return null; }
  catch { return `Could not open sharing. Copy this link: ${url}`; }
}
