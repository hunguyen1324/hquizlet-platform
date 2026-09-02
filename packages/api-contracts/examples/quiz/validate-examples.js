#!/usr/bin/env node
/**
 * P4-CON-03 — Validate golden JSON examples for Phase 4 quiz contract.
 * Checks that examples match the OpenAPI QuizGeneratedItem / QuizAnswer schemas
 * and that no answer leaks a correct answer or uses obsolete fields.
 *
 * Usage: node validate-examples.js
 * Exit 0 = PASS, Exit 1 = FAIL.
 */
const fs = require("fs");
const path = require("path");
const DIR = __dirname;
const errors = [];

function fail(file, msg) {
  errors.push(`  X ${file}: ${msg}`);
}

function requireFields(obj, fields, ctx) {
  for (const f of fields) {
    if (obj[f] === undefined || obj[f] === null) fail(ctx, `missing required field "${f}"`);
  }
}

// ── QuizGeneratedItem schema check ────────────────────────────────────────────
function validateGeneratedItem(item, mode, file, idx) {
  const ctx = `${file} items[${idx}]`;
  requireFields(item, ["id", "flashcardId", "kind", "position"], ctx);
  if (typeof item.flashcardId !== "number") fail(ctx, "flashcardId must be number");
  if (typeof item.position !== "number") fail(ctx, "position must be number");
  if (typeof item.id !== "string") fail(ctx, "id must be string");

  // Security: no correctIndex should leak
  if ("correctIndex" in item) fail(ctx, "SECURITY: correctIndex must not be present in generate response");

  // Kind-specific validation
  if (mode === "test") {
    if (item.kind !== "question") fail(ctx, "test mode items must have kind=question");
    if (!item.term) fail(ctx, "test mode items must have term");
    if (!Array.isArray(item.choices) || item.choices.length < 2)
      fail(ctx, "test mode items must have >=2 choices");
  }
  if (mode === "match") {
    if (item.kind !== "term" && item.kind !== "definition")
      fail(ctx, "match mode items must have kind=term or kind=definition");
    if (!item.text) fail(ctx, "match mode items must have text");
    if (!item.pairId) fail(ctx, "match mode items must have pairId");
  }
  if (mode === "flashcards" || mode === "learn") {
    if (item.kind !== "question") fail(ctx, `${mode} mode items must have kind=question`);
    if (!item.term) fail(ctx, `${mode} mode items must have term`);
    if (item.kind === "question" && !item.definition)
      fail(ctx, `${mode} mode question items must have definition`);
  }
}

function validateGenerate(data, file) {
  const body = data.response ? data.response.body : data;
  requireFields(body, ["mode", "seed", "contractVersion", "items"], file);
  if (body.contractVersion !== "1.4.0") fail(file, `contractVersion must be "1.4.0", got "${body.contractVersion}"`);
  if (!Array.isArray(body.items)) {
    fail(file, "items must be an array (flat, not {subset, pairs})");
    return;
  }
  for (let i = 0; i < body.items.length; i++) {
    validateGeneratedItem(body.items[i], body.mode, file, i);
  }
}

// ── QuizEvaluateRequest check ──────────────────────────────────────────────────
function validateEvaluateAnswer(ans, mode, file, idx) {
  const ctx = `${file} answers[${idx}]`;
  requireFields(ans, ["flashcardId", "attempts"], ctx);
  if (typeof ans.flashcardId !== "number") fail(ctx, "flashcardId must be number");
  if (typeof ans.attempts !== "number" || ans.attempts < 1) fail(ctx, "attempts must be >= 1");

  // Security: no "correct" field in answer
  if ("correct" in ans) fail(ctx, "SECURITY: correct must not be in answer payload");

  // Obsolete fields
  if ("termIndex" in ans) fail(ctx, "obsolete field: termIndex — use pairId/matchedFlashcardId");
  if ("definitionIndex" in ans) fail(ctx, "obsolete field: definitionIndex — use pairId/matchedFlashcardId");
  if ("answer" in ans) fail(ctx, "ambiguous field: answer — use submitted (learn) or selectedIndex (test)");

  if (mode === "test") {
    if (!("selectedIndex" in ans)) fail(ctx, "test mode answers must have selectedIndex");
  }
  if (mode === "learn") {
    if (!("submitted" in ans)) fail(ctx, "learn mode answers must have submitted");
  }
  if (mode === "match") {
    if (!("pairId" in ans)) fail(ctx, "match mode answers must have pairId");
    if (!("matchedFlashcardId" in ans)) fail(ctx, "match mode answers must have matchedFlashcardId");
  }
}

function validateEvaluate(data, file) {
  const body = data.response ? data.response.body : data;
  requireFields(body, ["mode", "seed", "score", "total", "cardResults", "contractVersion"], file);
  if (body.contractVersion !== "1.4.0") fail(file, `contractVersion must be "1.4.0"`);
  if (!Array.isArray(body.cardResults)) {
    fail(file, "cardResults must be array");
    return;
  }
  for (let i = 0; i < body.cardResults.length; i++) {
    const cr = body.cardResults[i];
    if (typeof cr.flashcardId !== "number") fail(file, `cardResults[${i}].flashcardId must be number`);
    if (typeof cr.correct !== "boolean") fail(file, `cardResults[${i}].correct must be boolean`);
    if (typeof cr.attempts !== "number") fail(file, `cardResults[${i}].attempts must be number`);
  }

  // Validate the request answers too
  if (data.request && data.request.body) {
    const requestBody = data.request.body;
    requireFields(requestBody, ["mode", "seed", "limit", "answers"], `${file} request.body`);
    if (typeof requestBody.limit !== "number" || requestBody.limit < 1 || requestBody.limit > 100) {
      fail(`${file} request.body`, "limit must be number between 1 and 100");
    }
    if (requestBody.answers) {
      for (let i = 0; i < requestBody.answers.length; i++) {
        validateEvaluateAnswer(requestBody.answers[i], body.mode, file, i);
      }
    }
  }
}

// ── Error envelope check ──────────────────────────────────────────────────────
function validateErrorEnvelope(data, file) {
  for (const [k, v] of Object.entries(data)) {
    if (k === "_comment") continue;
    if (v.body) {
      requireFields(v.body, ["code", "message", "requestId", "details"], `error:${k}`);
    }
    if (v.status && typeof v.status !== "number") fail(`error:${k}`, "status must be number");
  }
}

// ── Main ──────────────────────────────────────────────────────────────────────
const files = fs.readdirSync(DIR).filter((f) => f.endsWith(".json") && f !== "error-envelope.json");
let total = 0;
for (const file of files) {
  total++;
  try {
    const data = JSON.parse(fs.readFileSync(path.join(DIR, file), "utf-8"));
    if (file.includes("-generate")) validateGenerate(data, file);
    else if (file.includes("-evaluate")) validateEvaluate(data, file);
  } catch (e) {
    fail(file, `invalid JSON: ${e.message}`);
  }
}
total++;
try {
  const data = JSON.parse(fs.readFileSync(path.join(DIR, "error-envelope.json"), "utf-8"));
  validateErrorEnvelope(data, "error-envelope.json");
} catch (e) {
  fail("error-envelope.json", `invalid JSON: ${e.message}`);
}

process.stdout.write(`\nValidated ${total} golden example files.\n`);
if (errors.length > 0) {
  process.stderr.write(`\n${errors.length} error(s):\n${errors.join("\n")}\n`);
  process.exit(1);
}
process.stdout.write("All golden examples PASS.\n");
process.exit(0);
