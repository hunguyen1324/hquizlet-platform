// useProgressSave.ts — Dev 4
// P3-LEARN-01..03: Hook để lưu progress sau khi một learning session hoàn thành.
//
// Đặc tính:
//   - Chống double-submit qua savedRef (idempotent ở UI layer)
//   - Retry idempotent: cùng idempotencyKey → backend dedup
//   - 409 conflict được coi là success (đã lưu trước đó)
//   - Không block UI — save chạy async, UI nhận status qua state
//   - Token lấy từ useAuth, không từ prop

import React from "react";
import { useAuth } from "../auth/AuthContext";
import {
  saveProgress,
  makeIdempotencyKey,
  type CardResult,
  type ProgressSaveResponse,
  type ProgressSaveError,
} from "./progressContract";
import type { LearningMode } from "../../lib/api/client";

export type SaveStatus =
  | { state: "idle" }
  | { state: "saving" }
  | { state: "saved"; session: ProgressSaveResponse }
  | { state: "error"; error: ProgressSaveError; retryable: boolean };

export type UseProgressSaveOptions = {
  studySetId: number;
  mode: LearningMode;
};

export type UseProgressSaveResult = {
  status: SaveStatus;
  /** Gọi khi session hoàn thành. Chống double-call. */
  onSessionComplete: (args: {
    score: number;
    total: number;
    cardResults: CardResult[];
    startedAt: Date;
  }) => void;
  /** Reset về idle (cho Làm lại). */
  reset: () => void;
};

export function useProgressSave({
  studySetId,
  mode,
}: UseProgressSaveOptions): UseProgressSaveResult {
  const { token } = useAuth();
  const [status, setStatus] = React.useState<SaveStatus>({ state: "idle" });

  // Prevent double-submit: track whether save was already initiated.
  const savedRef = React.useRef(false);
  // Track the startedAt of the current session for idempotency key.
  const startedAtRef = React.useRef<string | null>(null);

  const onSessionComplete = React.useCallback(
    ({
      score,
      total,
      cardResults,
      startedAt,
    }: {
      score: number;
      total: number;
      cardResults: CardResult[];
      startedAt: Date;
    }) => {
      // Guard: only save once per session instance.
      if (savedRef.current) return;
      savedRef.current = true;

      const startedAtISO = startedAt.toISOString();
      startedAtRef.current = startedAtISO;
      const completedAtISO = new Date().toISOString();
      const idempotencyKey = makeIdempotencyKey(studySetId, mode, startedAtISO);

      setStatus({ state: "saving" });

      saveProgress(token, studySetId, {
        mode,
        score,
        total,
        startedAt: startedAtISO,
        completedAt: completedAtISO,
        cardResults,
        idempotencyKey,
      }).then((result) => {
        if (result.ok) {
          setStatus({ state: "saved", session: result.session });
        } else if (result.error.kind === "conflict") {
          // 409 = already saved → treat as success.
          // We don't have the session object, synthesize a minimal one.
          setStatus({
            state: "saved",
            session: {
              id: 0,
              mode,
              score,
              total,
              completedAt: completedAtISO,
              createdAt: completedAtISO,
            },
          });
        } else {
          const retryable =
            result.error.kind === "network" || result.error.kind === "server";
          setStatus({ state: "error", error: result.error, retryable });
          // Allow retry by resetting the guard on retryable errors.
          if (retryable) {
            savedRef.current = false;
          }
        }
      });
    },
    [token, studySetId, mode]
  );

  const reset = React.useCallback(() => {
    savedRef.current = false;
    startedAtRef.current = null;
    setStatus({ state: "idle" });
  }, []);

  return { status, onSessionComplete, reset };
}
