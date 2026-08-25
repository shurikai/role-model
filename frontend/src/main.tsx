import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// Self-hosted faces for the Kanagawa Lotus theme (see index.css @theme). Only
// the weights the design actually uses are imported — Space Grotesk 500/700,
// Inter 400/500, IBM Plex Mono 400/500 — so the rest never reach the bundle.
import "@fontsource/space-grotesk/500.css";
import "@fontsource/space-grotesk/700.css";
import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./index.css";
import App from "./App.tsx";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
