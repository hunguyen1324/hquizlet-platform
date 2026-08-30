import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";
import "./login.css";
import "./register.css";

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

type AuthMode = "login" | "register";

const gatewayUrl =
  import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ??
  "http://localhost:8080";

const apiPreviews: ApiPreview[] = [
  { label: "Current user", method: "GET", path: "/v1/auth/me" },
  { label: "Study sets from PostgreSQL", method: "GET", path: "/v1/study-sets" },
  { label: "Create live session", method: "POST", path: "/v1/live-sessions" },
];

function App() {
  const [status, setStatus] = useState<HealthStatus>("checking");
  const [health, setHealth] = useState<ServiceHealth[]>([]);
  const [authMode, setAuthMode] = useState<AuthMode>("login");
  const [name, setName] = useState("Demo User");
  const [email, setEmail] = useState("demo@hquizlet.local");
  const [password, setPassword] = useState("");
  const [loginMessage, setLoginMessage] = useState(
    "Create a demo account or check the auth service through gateway.",
  );
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
          data.services.every((service) => service.status === "ok")
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

  async function handleLogin(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (authMode === "register") {
      await handleRegister();
      return;
    }

    setLoginMessage("Checking auth service through gateway...");

    try {
      const response = await fetch(`${gatewayUrl}/v1/auth/me`);
      const body = await response.json();
      setLoginMessage(
        body.authenticated
          ? `Signed in as ${body.user?.email ?? email}`
          : "Auth service is reachable. Real login API will be added next.",
      );
    } catch (error) {
      setLoginMessage(
        error instanceof Error
          ? `Cannot reach auth service: ${error.message}`
          : "Cannot reach auth service.",
      );
    }
  }

  async function handleRegister() {
    setLoginMessage("Creating account through auth service...");

    try {
      const response = await fetch(`${gatewayUrl}/v1/auth/register`, {
        body: JSON.stringify({ name, email, password }),
        headers: {
          "Content-Type": "application/json",
        },
        method: "POST",
      });
      const body = await response.json();

      if (!response.ok) {
        setLoginMessage(body.error ?? "Registration failed.");
        return;
      }

      setLoginMessage(`Registered ${body.user.email}. User id: ${body.user.id}`);
      setApiResult(JSON.stringify({ status: response.status, body }, null, 2));
    } catch (error) {
      setLoginMessage(
        error instanceof Error
          ? `Cannot reach auth service: ${error.message}`
          : "Cannot reach auth service.",
      );
    }
  }

  return (
    <main className="login-shell">
      <section className="login-hero">
        <p className="eyebrow">HQuizlet Platform</p>
        <h1>Study faster, ship smarter.</h1>
        <p>
          A new separated frontend for the Go and Rust microservices platform.
          Login UI is ready, and the auth service can be wired behind it next.
        </p>

        <div className="service-strip">
          <strong>
            {liveServices} / {health.length || 4}
          </strong>
          <span>backend services live</span>
          <span className={`badge badge--${status}`}>{statusLabel(status)}</span>
        </div>
      </section>

      <section className="login-card">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Welcome back</p>
            <h2>{authMode === "login" ? "Dang nhap" : "Dang ky"}</h2>
          </div>
          <span className={`status-dot status-dot--${status}`} />
        </div>

        <div className="auth-tabs" role="tablist" aria-label="Auth mode">
          <button
            className={authMode === "login" ? "auth-tab auth-tab--active" : "auth-tab"}
            onClick={() => setAuthMode("login")}
            type="button"
          >
            Dang nhap
          </button>
          <button
            className={authMode === "register" ? "auth-tab auth-tab--active" : "auth-tab"}
            onClick={() => setAuthMode("register")}
            type="button"
          >
            Dang ky
          </button>
        </div>

        <form className="login-form" onSubmit={handleLogin}>
          {authMode === "register" ? (
            <label>
              Name
              <input
                autoComplete="name"
                onChange={(event) => setName(event.target.value)}
                placeholder="Your name"
                type="text"
                value={name}
              />
            </label>
          ) : null}

          <label>
            Email
            <input
              autoComplete="email"
              onChange={(event) => setEmail(event.target.value)}
              placeholder="you@example.com"
              type="email"
              value={email}
            />
          </label>

          <label>
            Password
            <input
              autoComplete="current-password"
              onChange={(event) => setPassword(event.target.value)}
              placeholder="Enter your password"
              type="password"
              value={password}
            />
          </label>

          <button className="primary-button" type="submit">
            {authMode === "login" ? "Dang nhap" : "Tao tai khoan"}
          </button>
        </form>

        <p className="login-message">{loginMessage}</p>

        <div className="mini-dashboard">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Dev tools</p>
              <h2>Gateway routes</h2>
            </div>
            <a href={`${gatewayUrl}/healthz/services`} target="_blank">
              Health JSON
            </a>
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
        </div>
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
