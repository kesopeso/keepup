import { MobileApp } from './src/MobileApp';
import { AccountApp } from './src/auth/AccountApp';

export default function App() {
  return process.env.EXPO_PUBLIC_AUTH_ENABLED === 'true' ? <AccountApp /> : <MobileApp />;
}
