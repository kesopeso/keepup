import type { ViewingSocket } from './viewing-session.ts';
type NativeSocketConstructor = new (url: string, protocols: undefined, options: { headers: Record<string, string> }) => ViewingSocket;
// React Native otherwise inserts a browser-style Origin, which account native transport rejects.
// Empty Origin suppresses that default and supplies no origin authority to the server.
export function nativeAccountSocket(url: string, Socket = WebSocket as unknown as NativeSocketConstructor): ViewingSocket {
  return new Socket(url, undefined, { headers: { 'X-KeepUp-Client': 'native', Origin: '' } });
}
