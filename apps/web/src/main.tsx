import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";

type HealthStatus = "checking" | "live" | "offline";

type HealthResponse = {
  services: ServiceHealth[];
};

type ServiceHealth = {
  name: string;
  url: string;
  status: string;
};

const gatewayUrl =
  import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ??
  "http://localhost:8080";

function App() {
  const [status, setStatus] = useState<HealthStatus>("checking");
  const [health, setHealth] = useState<ServiceHealth[]>([]);

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

  return (
    <main style={{ fontFamily: "system-ui", padding: 32, maxWidth: 720 }}>
      <h1>HQuizlet Platform</h1>
      <p>Frontend is separated from the Go backend services.</p>

      <section
        style={{
          border: "1px solid #ddd",
          borderRadius: 8,
          marginTop: 24,
          padding: 20,
        }}
      >
        <h2 style={{ marginTop: 0 }}>Backend Status</h2>
        <div style={{ display: "grid", gap: 12 }}>
          {health.length > 0 ? (
            health.map((service) => (
              <div
                key={service.name}
                style={{
                  alignItems: "center",
                  display: "flex",
                  justifyContent: "space-between",
                }}
              >
                <span>
                  <strong>{service.name}</strong>
                  <span style={{ color: "#555" }}> - {service.url}</span>
                </span>
                <strong
                  style={{ color: statusColor(toHealthStatus(service.status)) }}
                >
                  {statusLabel(toHealthStatus(service.status))}
                </strong>
              </div>
            ))
          ) : (
            <p style={{ margin: 0 }}>
              Gateway:{" "}
              <strong style={{ color: statusColor(status) }}>
                {statusLabel(status)}
              </strong>
            </p>
          )}
        </div>
        <p style={{ color: "#555", marginBottom: 0 }}>
          Health endpoint: {gatewayUrl}/healthz/services
        </p>
        {health.length > 0 ? (
          <pre
            style={{
              background: "#f6f6f6",
              borderRadius: 6,
              marginTop: 16,
              padding: 12,
            }}
          >
            {JSON.stringify(health, null, 2)}
          </pre>
        ) : null}
      </section>

      <ul>
        <li>Gateway: http://localhost:8080</li>
        <li>Auth: http://localhost:8081</li>
        <li>Study: http://localhost:8082</li>
        <li>Quiz: http://localhost:8083</li>
      </ul>
    </main>
  );
}

function statusLabel(status: HealthStatus) {
  if (status === "checking") return "checking...";
  if (status === "live") return "live";
  return "offline";
}

function statusColor(status: HealthStatus) {
  if (status === "checking") return "#9a6700";
  if (status === "live") return "#1a7f37";
  return "#cf222e";
}

function toHealthStatus(status: string): HealthStatus {
  return status === "ok" ? "live" : "offline";
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
