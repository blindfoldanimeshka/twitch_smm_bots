import { useEffect, useRef, useState } from 'react';
import { config } from '../config';
import { useLogStore } from '../stores/logStore';

export interface WSMessage {
  type: string;
  payload: any;
}

export const useWebSocket = (onMessage: (msg: WSMessage) => void) => {
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | undefined>(undefined);
  const onMessageRef = useRef(onMessage);
  const addBotLog = useLogStore((state) => state.addBotLog);
  const addSettingsLog = useLogStore((state) => state.addSettingsLog);

  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  useEffect(() => {
    const connect = () => {
      const wsUrl = `${config.wsUrl}/api/ws?api_key=${config.apiKey}`;
      const ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        console.log('WebSocket connected');
        setConnected(true);
      };

      ws.onmessage = (event) => {
        try {
          const msg: WSMessage = JSON.parse(event.data);

          // Handle bot logs (extended bot_message_sent)
          if (msg.type === 'bot_message_sent') {
            const payload = msg.payload;
            addBotLog(payload.bot_id || payload.botID, {
              botId: payload.bot_id || payload.botID,
              logType: payload.log_type || payload.logType || 'message',
              level: payload.level || 'info',
              message: payload.message,
              context: payload.context,
              timestamp: new Date(payload.timestamp),
            });
          }

          // Handle settings logs
          if (msg.type === 'settings_log') {
            const payload = msg.payload;
            addSettingsLog({
              level: payload.level,
              message: payload.message,
              changes: payload.changes,
              timestamp: new Date(payload.timestamp),
            });
          }

          // Pass to parent handler
          onMessageRef.current(msg);
        } catch (err) {
          console.error('Failed to parse WebSocket message:', err);
        }
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
      };

      ws.onclose = () => {
        console.log('WebSocket disconnected');
        setConnected(false);

        // Reconnect after 5 seconds
        reconnectTimeoutRef.current = window.setTimeout(() => {
          console.log('Reconnecting WebSocket...');
          connect();
        }, 5000);
      };

      wsRef.current = ws;
    };

    connect();

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [addBotLog, addSettingsLog]);

  return { connected };
};
