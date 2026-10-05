// Production traffic goes through Nginx; local development uses the Go gateway.
export const gatewayUrl = (import.meta.env.VITE_GATEWAY_URL
  ?? (import.meta.env.DEV ? "http://localhost:8080" : "/api")).replace(/\/$/, "");
