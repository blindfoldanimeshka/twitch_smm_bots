export const config = {
  apiUrl: import.meta.env.VITE_API_URL || 'http://localhost:8080',
  wsUrl: import.meta.env.VITE_WS_URL || 'ws://localhost:8080',
  apiKey: import.meta.env.VITE_API_KEY || '',
};
