"use client";

import { Bell, CheckCircle2, Info, TriangleAlert, XCircle } from "lucide-react";
import { createContext, type ReactNode, useCallback, useContext, useMemo, useRef, useState } from "react";
import { createClientId } from "../lib/client-id";

type NotificationTone = "success" | "info" | "warning" | "error";

type NotificationItem = {
  id: string;
  title: string;
  message: string;
  tone: NotificationTone;
  createdAt: number;
};

type NotificationContextValue = {
  notify: (item: Omit<NotificationItem, "id" | "createdAt">) => void;
};

const NotificationContext = createContext<NotificationContextValue | null>(null);

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const [activeItems, setActiveItems] = useState<NotificationItem[]>([]);
  const [historyItems, setHistoryItems] = useState<NotificationItem[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const timersRef = useRef<Map<string, number>>(new Map());

  const notify = useCallback((item: Omit<NotificationItem, "id" | "createdAt">) => {
    const next: NotificationItem = {
      ...item,
      id: createClientId("notification"),
      createdAt: Date.now(),
    };
    setActiveItems((current) => [next, ...current].slice(0, 8));
    setHistoryItems((current) => [next, ...current].slice(0, 100));
    const timer = window.setTimeout(() => {
      setActiveItems((current) => current.filter((entry) => entry.id !== next.id));
      timersRef.current.delete(next.id);
    }, 5000);
    timersRef.current.set(next.id, timer);
  }, []);

  const value = useMemo(() => ({ notify }), [notify]);
  const visibleToast = activeItems[0] ?? null;

  return (
    <NotificationContext.Provider value={value}>
      {children}
      <div className="notification-hub">
        <button className="notification-bell" onClick={() => setIsOpen((current) => !current)} type="button">
          <Bell size={18} />
          {historyItems.length > 0 ? <span className="notification-bell__count">{historyItems.length}</span> : null}
        </button>

        {visibleToast ? (
          <div className={`notification-toast notification-toast--${visibleToast.tone}`}>
            <div className="notification-toast__icon">{iconForTone(visibleToast.tone)}</div>
            <div>
              <strong>{visibleToast.title}</strong>
              <p>{visibleToast.message}</p>
            </div>
          </div>
        ) : null}

        {isOpen ? (
          <div className="notification-panel">
            <div className="notification-panel__head">
              <strong>Notifications</strong>
              <button
                className="studio-button studio-button--ghost studio-button--inline"
                onClick={() => {
                  setActiveItems([]);
                  setHistoryItems([]);
                }}
                type="button"
              >
                Clear
              </button>
            </div>
            <div className="notification-panel__list">
              {historyItems.length === 0 ? <p className="empty-copy">No notifications yet.</p> : null}
              {historyItems.map((item) => (
                <article className={`notification-entry notification-entry--${item.tone}`} key={item.id}>
                  <div className="notification-toast__icon">{iconForTone(item.tone)}</div>
                  <div>
                    <strong>{item.title}</strong>
                    <p>{item.message}</p>
                  </div>
                </article>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </NotificationContext.Provider>
  );
}

export function useNotifications() {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error("useNotifications must be used within NotificationsProvider");
  }
  return context;
}

function iconForTone(tone: NotificationTone) {
  switch (tone) {
    case "success":
      return <CheckCircle2 size={18} />;
    case "warning":
      return <TriangleAlert size={18} />;
    case "error":
      return <XCircle size={18} />;
    default:
      return <Info size={18} />;
  }
}
