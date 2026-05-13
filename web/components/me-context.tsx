"use client";

import { createContext, useContext } from "react";
import { Me } from "@/lib/types";

// MeContext делает текущего пользователя доступным всем authenticated-страницам
// без повторного запроса /api/auth/me. Провайдер ставится в (authenticated)/layout.tsx.

export const MeContext = createContext<Me | null>(null);

export function useMe(): Me {
  const me = useContext(MeContext);
  if (!me) {
    throw new Error("useMe() called outside MeContext (нужен (authenticated)/layout.tsx)");
  }
  return me;
}
