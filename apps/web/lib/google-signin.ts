type GoogleIdentity = {
  initialize(config: { client_id: string; nonce: string; callback: (response: { credential: string }) => void; auto_select: boolean; ux_mode: string }): void;
  renderButton(element: HTMLElement, options: { theme: string; size: string; text: string; width: number }): void;
  cancel(): void;
  disableAutoSelect(): void;
};
declare global { interface Window { google?: { accounts: { id: GoogleIdentity } } } }
let loading: Promise<GoogleIdentity> | undefined;
export function loadGoogleIdentity(): Promise<GoogleIdentity> {
  if (window.google) return Promise.resolve(window.google.accounts.id);
  if (!loading) loading = new Promise((resolve, reject) => {
    const script = document.createElement("script");
    const fail = () => {
      clearTimeout(timer);
      script.remove();
      loading = undefined;
      reject(new Error("google_unavailable"));
    };
    const timer = window.setTimeout(fail, 15000);
    script.src = "https://accounts.google.com/gsi/client";
    script.async = true;
    script.referrerPolicy = "no-referrer";
    script.onload = () => {
      clearTimeout(timer);
      if (window.google) resolve(window.google.accounts.id);
      else fail();
    };
    script.onerror = fail;
    document.head.append(script);
  });
  return loading;
}
