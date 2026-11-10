import { useEffect, useState } from "react";
import { fetchHealth } from "./api";
import type { HealthStatus, ServiceHealth } from "../types";

/** Status is informational: background tabs do not fan out service probes. */
export function useServiceHealth() {
  const [healthStatus, setHealthStatus] = useState<HealthStatus>("checking");
  const [services, setServices] = useState<ServiceHealth[]>([]);
  useEffect(() => startServiceHealthPolling(result => {
    setServices(result);
    setHealthStatus(result.every(service => service.status === "ok") ? "live" : "offline");
  }, () => setHealthStatus("offline")), []);
  return { healthStatus, services };
}

export function startServiceHealthPolling(onResult: (services: ServiceHealth[]) => void, onOffline: () => void) {
    let active = true;
    let controller: AbortController | undefined;
    let timer: number | undefined;
    let checkedAt = 0;
    async function check() {
      if (!active || document.hidden || controller) return;
      controller = new AbortController();
      try {
        const result = await fetchHealth(controller.signal);
        if (active && !controller.signal.aborted) onResult(result);
      } catch {
        if (active && !controller.signal.aborted) onOffline();
      } finally {
        checkedAt = Date.now();
        controller = undefined;
        if (active && !document.hidden) timer = window.setTimeout(check, 60_000);
      }
    }
    function visibilityChanged() {
      window.clearTimeout(timer);
      if (document.hidden) controller?.abort();
      else if (!controller) {
        const remaining = Math.max(0, 60_000 - (Date.now() - checkedAt));
        timer = window.setTimeout(check, remaining);
      }
    }
    void check();
    document.addEventListener("visibilitychange", visibilityChanged);
    return () => { active = false; window.clearTimeout(timer); controller?.abort(); document.removeEventListener("visibilitychange", visibilityChanged); };
}

