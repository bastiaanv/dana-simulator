import React from "react";
import ReactDOM from "react-dom/client";
import App from "./dashboard/index.tsx";
import "./index.css";
import en from "./assets/en.json";
import i18n from "i18next";
import { initReactI18next } from "react-i18next";

i18n
  .use(initReactI18next) // passes i18n down to react-i18next
  .init({
    resources: {
      en: {
        translation: en,
      },
    },
    lng: "en",
    fallbackLng: "en",
    interpolation: { escapeValue: false },
  });

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
