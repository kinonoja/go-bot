"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui";

export function ThemeToggle() {
  const [light, setLight] = useState(false);
  useEffect(() => {
    const saved = localStorage.getItem("gobot-theme");
    const useLight = saved === "light";
    setLight(useLight);
    document.documentElement.dataset.theme = useLight ? "light" : "dark";
  }, []);
  function toggle() {
    const value = !light;
    setLight(value);
    document.documentElement.dataset.theme = value ? "light" : "dark";
    localStorage.setItem("gobot-theme", value ? "light" : "dark");
  }
  return <Button variant="ghost" onClick={toggle} aria-label={`Switch to ${light ? "dark" : "light"} theme`}>{light ? "☾" : "☼"}</Button>;
}
