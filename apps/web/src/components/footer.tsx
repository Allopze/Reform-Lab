"use client";

import { useEffect, useState } from "react";
import { getFooterMessage } from "@/lib/api";
import {
  DEFAULT_FOOTER_MESSAGE,
  FOOTER_MESSAGE_UPDATED_EVENT,
} from "@/lib/footer-message";

export default function Footer() {
  const [message, setMessage] = useState(DEFAULT_FOOTER_MESSAGE);

  useEffect(() => {
    let active = true;

    void getFooterMessage()
      .then((nextMessage) => {
        if (active) {
          setMessage(nextMessage);
        }
      })
      .catch(() => {
        // Keep the default footer when the public setting cannot be loaded.
      });

    const handleFooterUpdate = (event: Event) => {
      const customEvent = event as CustomEvent<string>;
      if (typeof customEvent.detail === "string" && customEvent.detail.trim() !== "") {
        setMessage(customEvent.detail);
      }
    };

    window.addEventListener(FOOTER_MESSAGE_UPDATED_EVENT, handleFooterUpdate);

    return () => {
      active = false;
      window.removeEventListener(FOOTER_MESSAGE_UPDATED_EVENT, handleFooterUpdate);
    };
  }, []);

  return (
    <footer className="px-5 pb-6 pt-8 sm:px-8">
      <p className="mx-auto text-center max-w-7xl text-xs text-stone-400">
        {message}
      </p>
    </footer>
  );
}
