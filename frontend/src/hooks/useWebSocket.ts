import { createContext, createElement, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { config } from '../config';
import { useLogStore } from '../stores/logStore';

export interface WSMessage {
  type: string;
  payload: any;
}

export interface BotPresenceEvent {
  bot_id: string;
  login: string;
  state: 'online' | 'cooldown' | 'stopped' | 'error' | string;
  at: string;
}

type MessageHandler = (msg: WSMessage) => void;
type StatusListener = (connected: boolean) => void;

// ---------------------------------------------------------------------------
// Shared singleton connection
//
// The whole app uses exactly one WebSocket, no matter how many components
// subscribe. The singleton feeds the log store once per message and then
// fans out to all registered handlers.
// ---------------------------------------------------------------------------

const BASE_RECONNECT_DELAY_MS = 1_000;
const MAX_RECONNECT_DELAY_MS = 30_000;
const RECONNECT_JITTER_MS = 300;

const messageHandlers = new Set<MessageHandler>();
const statusListeners = new Set<StatusListener>();

let socket: WebSocket | null = null;
let connected = false;
let started = false;
let reconnectAttempts = 0;
let reconnectTimer: number | undefined;

function setConnected(next: boolean): void {
  connected = next;
  statusListeners.forEach((listener) => listener(next));
}

function handleStoreEvents(msg: WSMessage): void {
  const payload = msg.payload ?? {};

  if (msg.type === 'bot_log') {
    const botId = payload.bot_id || payload.botID || '';
    useLogStore.getState().addBotLog(botId, {
      botId,
      logType: payload.log_type || payload.logType || 'message',
      level: payload.level || 'info',
      message: payload.message,
      context: payload.context,
      timestamp: new Date(payload.timestamp),
    });
    return;
  }

  if (msg.type === 'bot_message_sent') {
    const botId = payload.bot_id || payload.botID || '';
    useLogStore.getState().addBotLog(botId, {
      botId,
      logType: 'message',
      level: 'success',
      message: payload.message,
      context: payload.channel ? { channel: payload.channel, personality: payload.personality } : undefined,
      timestamp: new Date(payload.timestamp || Date.now()),
    });
    return;
  }

  if (msg.type === 'settings_log') {
    useLogStore.getState().addSettingsLog({
      level: payload.level || 'info',
      message: payload.message,
      timestamp: new Date(payload.timestamp),
    });
    return;
  }

  if (msg.type === 'bot_presence_changed') {
    useLogStore.getState().addPresenceEvent({
      bot_id: payload.bot_id,
      login: payload.login,
      state: payload.state,
      at: payload.at,
    });
  }
}

function connect(): void {
  if (socket) {
    return; // connecting or already open
  }

  // The constructor throws synchronously on an invalid or blocked URL. Catch
  // it and use the same backoff as a dropped connection; the error must not
  // escape into the caller's render/effect path.
  let ws: WebSocket;
  try {
    ws = new WebSocket(`${config.wsUrl}/api/ws?api_key=${config.apiKey}`);
  } catch {
    scheduleReconnect();
    return;
  }
  socket = ws;

  ws.onopen = () => {
    reconnectAttempts = 0; // reset backoff after a successful open
    setConnected(true);
  };

  ws.onmessage = (event) => {
    let msg: WSMessage;
    try {
      msg = JSON.parse(event.data) as WSMessage;
    } catch {
      return; // malformed payload: drop silently
    }
    handleStoreEvents(msg);
    messageHandlers.forEach((handler) => handler(msg));
  };

  // Deliberately silent: payload and error logging may leak sensitive data.
  ws.onerror = () => {};

  ws.onclose = () => {
    if (socket === ws) {
      socket = null;
    }
    setConnected(false);
    scheduleReconnect();
  };
}

function scheduleReconnect(): void {
  if (!started || reconnectTimer !== undefined) {
    return;
  }

  const delay = Math.min(BASE_RECONNECT_DELAY_MS * 2 ** reconnectAttempts, MAX_RECONNECT_DELAY_MS);
  reconnectAttempts += 1;

  reconnectTimer = window.setTimeout(() => {
    reconnectTimer = undefined;
    connect();
  }, delay + Math.random() * RECONNECT_JITTER_MS);
}

/** Opens the shared socket if it is not running yet. Safe to call repeatedly. */
export function startWebSocket(): void {
  started = true;
  if (!socket && reconnectTimer === undefined) {
    connect();
  }
}

/** Registers a message handler on the shared socket; returns an unsubscribe function. */
export function subscribeWebSocket(handler: MessageHandler): () => void {
  messageHandlers.add(handler);
  startWebSocket();
  return () => {
    messageHandlers.delete(handler);
  };
}

/** Registers a connection-status listener; returns an unsubscribe function. */
export function onWebSocketStatus(listener: StatusListener): () => void {
  statusListeners.add(listener);
  return () => {
    statusListeners.delete(listener);
  };
}

export function getWebSocketConnected(): boolean {
  return connected;
}

// ---------------------------------------------------------------------------
// React bindings
// ---------------------------------------------------------------------------

interface WebSocketContextValue {
  connected: boolean;
}

const WebSocketContext = createContext<WebSocketContextValue | null>(null);

export const WebSocketProvider = ({ children }: { children: ReactNode }) => {
  const [connected, setConnected] = useState(getWebSocketConnected);

  useEffect(() => {
    startWebSocket();
    return onWebSocketStatus(setConnected);
  }, []);

  return createElement(WebSocketContext.Provider, { value: { connected } }, children);
};

/** Connection state of the shared socket. Works inside and without a provider. */
export const useWebSocketStatus = (): { connected: boolean } => {
  const context = useContext(WebSocketContext);
  const [connected, setConnected] = useState(getWebSocketConnected);

  useEffect(() => {
    if (context) {
      return; // the provider owns the status
    }
    return onWebSocketStatus(setConnected);
  }, [context]);

  return { connected: context ? context.connected : connected };
};

/** Subscribes a handler to the shared socket without opening a new one. */
export const useWebSocketSubscription = (onMessage: (msg: WSMessage) => void): void => {
  const onMessageRef = useRef(onMessage);

  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  useEffect(
    () =>
      subscribeWebSocket((msg) => {
        onMessageRef.current(msg);
      }),
    []
  );
};

/**
 * Compatibility wrapper for the pre-provider API. Subscribes to the shared
 * socket and reports its connection state. New code should use
 * `useWebSocketSubscription` + `useWebSocketStatus` instead.
 */
export const useWebSocket = (onMessage: (msg: WSMessage) => void): { connected: boolean } => {
  useWebSocketSubscription(onMessage);
  return useWebSocketStatus();
};
