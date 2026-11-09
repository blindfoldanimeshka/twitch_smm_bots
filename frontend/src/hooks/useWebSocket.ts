import { createContext, createElement, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { config } from '../config';
import type { Stats } from '../api/client';

// ---------------------------------------------------------------------------
// Typed events
//
// Mirrors the backend DTOs in internal/api/dto.go (WSMessage + payload types)
// and the seven types emitted by internal/api/websocket.go. Timestamps arrive
// as Go time.Time, marshalled to RFC3339 strings.
// ---------------------------------------------------------------------------

export interface BotStatusPayload {
  bot_id: string;
  status: string;
}

export interface BotPresencePayload {
  bot_id: string;
  login: string;
  state: string;
  at: string;
}

export interface BotMessagePayload {
  bot_id: string;
  message: string;
  personality: string;
  channel?: string;
  timestamp: string;
}

export interface BotLogPayload {
  bot_id: string;
  log_type: string;
  level: string;
  message: string;
  context?: Record<string, unknown>;
  timestamp: string;
}

export interface SettingsLogPayload {
  level: string;
  message: string;
  changes?: Record<string, unknown>;
  timestamp: string;
}

export interface AudioContextPayload {
  context: string;
}

export interface WSPayloads {
  bot_status_changed: BotStatusPayload;
  bot_presence_changed: BotPresencePayload;
  bot_message_sent: BotMessagePayload;
  stats_updated: Stats;
  bot_log: BotLogPayload;
  settings_log: SettingsLogPayload;
  audio_context: AudioContextPayload;
}

/** Discriminated union: narrowing on `type` also types `payload`. */
export type WSMessage = { [K in keyof WSPayloads]: { type: K; payload: WSPayloads[K] } }[keyof WSPayloads];

export interface WebSocketStatus {
  connected: boolean;
  error: boolean;
}

type MessageHandler = (msg: WSMessage) => void;
type StatusListener = (connected: boolean, error: boolean) => void;

// ---------------------------------------------------------------------------
// Shared singleton connection
//
// The whole app uses exactly one WebSocket, no matter how many components
// subscribe. The singleton fans out each message to all registered handlers.
// ---------------------------------------------------------------------------

const BASE_RECONNECT_DELAY_MS = 1_000;
const MAX_RECONNECT_DELAY_MS = 30_000;
const RECONNECT_JITTER_MS = 300;

const messageHandlers = new Set<MessageHandler>();
const statusListeners = new Set<StatusListener>();

let socket: WebSocket | null = null;
let connected = false;
let socketError = false;
let started = false;
let reconnectAttempts = 0;
let reconnectTimer: number | undefined;

function notifyStatus(): void {
  statusListeners.forEach((listener) => listener(connected, socketError));
}

function setConnected(next: boolean): void {
  connected = next;
  notifyStatus();
}

function setSocketError(next: boolean): void {
  socketError = next;
  notifyStatus();
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
    setSocketError(false);
    setConnected(true);
  };

  ws.onmessage = (event) => {
    let msg: WSMessage;
    try {
      msg = JSON.parse(event.data) as WSMessage;
    } catch {
      return; // malformed payload: drop silently
    }
    messageHandlers.forEach((handler) => handler(msg));
  };

  // Deliberately silent: payload and error logging may leak sensitive data.
  // Only the flag matters to the UI: "connecting" must not linger forever
  // when the socket clearly failed.
  ws.onerror = () => {
    setSocketError(true);
  };

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

export function getWebSocketStatus(): WebSocketStatus {
  return { connected, error: socketError };
}

// ---------------------------------------------------------------------------
// React bindings
// ---------------------------------------------------------------------------

interface WebSocketContextValue {
  connected: boolean;
  error: boolean;
}

const WebSocketContext = createContext<WebSocketContextValue | null>(null);

export const WebSocketProvider = ({ children }: { children: ReactNode }) => {
  const [status, setStatus] = useState<WebSocketStatus>(getWebSocketStatus);

  useEffect(() => {
    startWebSocket();
    return onWebSocketStatus((nextConnected, nextError) =>
      setStatus({ connected: nextConnected, error: nextError })
    );
  }, []);

  return createElement(WebSocketContext.Provider, { value: status }, children);
};

/** Connection state of the shared socket. Works inside and without a provider. */
export const useWebSocketStatus = (): WebSocketStatus => {
  const context = useContext(WebSocketContext);
  const [status, setStatus] = useState<WebSocketStatus>(getWebSocketStatus);

  useEffect(() => {
    if (context) {
      return; // the provider owns the status
    }
    return onWebSocketStatus((nextConnected, nextError) =>
      setStatus({ connected: nextConnected, error: nextError })
    );
  }, [context]);

  return context ?? status;
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
