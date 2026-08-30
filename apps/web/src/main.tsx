import React from "react";
import { createRoot } from "react-dom/client";

function App() {
  return (
    <main style={{ fontFamily: "system-ui", padding: 32 }}>
      <h1>HQuizlet Platform</h1>
      <p>Frontend is separated from the Go backend services.</p>
      <ul>
        <li>Gateway: http://localhost:8080</li>
        <li>Auth: http://localhost:8081</li>
        <li>Study: http://localhost:8082</li>
        <li>Quiz: http://localhost:8083</li>
      </ul>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);

