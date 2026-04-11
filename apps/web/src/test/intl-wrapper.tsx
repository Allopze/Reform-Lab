import type { ReactNode } from "react";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../messages/es.json";

export function IntlWrapper({ children }: { children: ReactNode }) {
  return (
    <NextIntlClientProvider locale="es" messages={messages}>
      {children}
    </NextIntlClientProvider>
  );
}
