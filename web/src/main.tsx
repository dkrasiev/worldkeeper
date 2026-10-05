import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { I18nProvider } from "./i18n";
import { JobsProvider } from "./jobs";
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <I18nProvider>
      <JobsProvider>
        <App />
      </JobsProvider>
    </I18nProvider>
  </StrictMode>,
);
