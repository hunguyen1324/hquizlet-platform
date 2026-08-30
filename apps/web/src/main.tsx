import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

type HealthStatus = "checking" | "live" | "offline";

type HealthResponse = {
  services: ServiceHealth[];
};

type ServiceHealth = {
  name: string;
  url: string;
  status: string;
};

type ApiPreview = {
  label: string;
  method: "GET" | "POST";
  path: string;
};

const gatewayUrl =
  import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ??
  "http://localhost:8080";

const apiPreviews: ApiPreview[] = [
  { label: "Current user", method: "GET", path: "/v1/auth/me" },
  { label: "Study sets", method: "GET", path: "/v1/study-sets" },
  { label: "Create live session", method: "POST", path: "/v1/live-sessions" },
];

function App() {
  const [status, setStatus] = useState<HealthStatus>("checking");
  const [health, setHealth] = useState<ServiceHealth[]>([]);
  const [apiResult, setApiResult] = useState(
    "Select an API call to inspect the gateway response.",
  );
  const liveServices = health.filter((service) => service.status === "ok").length;

  useEffect(() => {
    const controller = new AbortController();

    async function checkGateway() {
      setStatus("checking");

      try {
        const response = await fetch(`${gatewayUrl}/healthz/services`, {
          signal: controller.signal,
        });

        if (!response.ok) {
          throw new Error(`Gateway returned ${response.status}`);
        }

        const data = (await response.json()) as HealthResponse;
        setHealth(data.services);
        setStatus(
          data.services.some((service) => service.status === "ok")
            ? "live"
            : "offline",
        );
      } catch (error) {
        if (!controller.signal.aborted) {
          setHealth([]);
          setStatus("offline");
        }
      }
    }

    void checkGateway();
    const interval = window.setInterval(checkGateway, 5000);

    return () => {
      controller.abort();
      window.clearInterval(interval);
    };
  }, []);

  async function callApi(preview: ApiPreview) {
    setApiResult(`Calling ${preview.method} ${preview.path}...`);

    try {
      const response = await fetch(`${gatewayUrl}${preview.path}`, {
        method: preview.method,
      });
      const body = await response.json();
      setApiResult(
        JSON.stringify(
          {
            status: response.status,
            body,
          },
          null,
          2,
        ),
      );
    } catch (error) {
      setApiResult(
        JSON.stringify(
          {
            status: "offline",
            error: error instanceof Error ? error.message : "Unknown error",
          },
          null,
          2,
        ),
      );
    }
  }

  return (
    <main className="app-shell">
      <section className="hero">
        <div>
          <p className="eyebrow">Microservices dashboard</p>
          <h1>HQuizlet Platform</h1>
          <p className="hero-copy">
            Frontend React rieng, Go services rieng. Gateway la cua vao duy nhat
            de UI noi chuyen voi backend.
          </p>
        </div>
        <div className={`status-pill status-pill--${status}`}>
          <span />
          {statusLabel(status)}
        </div>
      </section>

      <section className="summary-grid">
        <article className="metric-card">
          <span>Services live</span>
          <strong>
            {liveServices} / {health.length || 4}
          </strong>
        </article>
        <article className="metric-card">
          <span>Gateway</span>
          <strong>{gatewayUrl}</strong>
        </article>
        <article className="metric-card">
          <span>Health refresh</span>
          <strong>5s</strong>
        </article>
      </section>

      <section className="content-grid">
        <article className="panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Backend status</p>
              <h2>Service health</h2>
            </div>
            <a href={`${gatewayUrl}/healthz/services`} target="_blank">
              JSON
            </a>
          </div>

          <div className="service-list">
            {health.length > 0 ? (
              health.map((service) => (
                <div className="service-row" key={service.name}>
                  <div>
                    <strong>{service.name}</strong>
                    <span>{service.url}</span>
                  </div>
                  <span
                    className={`badge badge--${toHealthStatus(service.status)}`}
                  >
                    {statusLabel(toHealthStatus(service.status))}
                  </span>
                </div>
              ))
            ) : (
              <div className="service-row">
                <div>
                  <strong>gateway</strong>
                  <span>{gatewayUrl}/healthz/services</span>
                </div>
                <span className={`badge badge--${status}`}>
                  {statusLabel(status)}
                </span>
              </div>
            )}
          </div>
        </article>

        <article className="panel">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Gateway API</p>
              <h2>Try routes</h2>
            </div>
          </div>

          <div className="action-list">
            {apiPreviews.map((preview) => (
              <button key={preview.path} onClick={() => void callApi(preview)}>
                <span>{preview.label}</span>
                <code>
                  {preview.method} {preview.path}
                </code>
              </button>
            ))}
          </div>

          <pre className="response-box">{apiResult}</pre>
        </article>
      </section>
    </main>
  );
}

function statusLabel(status: HealthStatus) {
  if (status === "checking") return "checking...";
  if (status === "live") return "live";
  return "offline";
}

function toHealthStatus(status: string): HealthStatus {
  return status === "ok" ? "live" : "offline";
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
