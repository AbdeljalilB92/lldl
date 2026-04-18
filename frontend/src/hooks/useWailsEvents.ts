import { useEffect, useRef, useState } from "react";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

// Subscribes to a named Wails event and returns the latest payload.
// Automatically unsubscribes on unmount or when eventName changes.
export function useWailsEvent<T = unknown>(eventName: string): T | null {
  const [data, setData] = useState<T | null>(null);
  const eventNameRef = useRef(eventName);
  eventNameRef.current = eventName;

  useEffect(() => {
    function handler(payload: T) {
      setData(payload);
    }

    // EventsOn signature: (eventName: string, callback: (...args: any[]) => void)
    EventsOn(eventNameRef.current, handler);

    return () => {
      EventsOff(eventNameRef.current);
    };
  }, [eventName]);

  return data;
}

// Subscribes to multiple Wails events and returns the latest payload for each.
// Keys are the event names; values are the most recent payload or null.
export function useWailsEvents<T extends Record<string, unknown>>(
  eventNames: string[],
): Partial<T> {
  const [events, setEvents] = useState<Partial<T>>({});

  useEffect(() => {
    const names = eventNames;

    const handlers: Array<{ name: string; handler: (payload: unknown) => void }> =
      names.map((name) => {
        const handler = (payload: unknown) => {
          setEvents((prev) => ({ ...prev, [name]: payload }));
        };
        EventsOn(name, handler);
        return { name, handler };
      });

    return () => {
      for (const { name } of handlers) {
        EventsOff(name);
      }
    };
  }, [eventNames.join(",")]);

  return events;
}
