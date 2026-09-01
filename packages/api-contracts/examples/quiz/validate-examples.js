#!/usr/bin/env node
/**
 * P4-CON-03 — Validate golden JSON examples for Phase 4 quiz contract.
 * Usage: node validate-examples.js
 * Exit 0 = PASS, Exit 1 = FAIL.
 */
const fs = require("fs");
const path = require("path");
const DIR = __dirname;
const errors = [];
function fail(file, msg) { errors.push(`  X ${file}: ${msg}`); }
function requireFields(obj, fields, ctx) {
  for (const f of fields) { if (!obj[f] && obj[f] !== 0) fail(ctx, `missing "${f}"`); }
}
function validateGenerate(resp, file) {
  requireFields(resp, ["mode", "seed", "contractVersion", "items"], file);
  if (resp.mode === "match") {
    if (!resp.items || typeof resp.items !== "object" || Array.isArray(resp.items))
      fail(file, "match items must be { subset, pairs }");
    else {
      if (typeof resp.items.subset !== "number") fail(file, "subset must be number");
      if (!Array.isArray(resp.items.pairs)) fail(file, "pairs must be array");
    }
  } else if (Array.isArray(resp.items)) {
    for (const item of resp.items) {
      if (typeof item.flashcardId !== "number") fail(file, "flashcardId must be number");
      if (resp.mode === "test") {
        if (!Array.isArray(item.choices) || item.choices.length < 2) fail(file, "test needs >=2 choices");
        if (typeof item.correctIndex !== "number") fail(file, "correctIndex must be number");
      }
    }
  }
}
function validateEvaluate(resp, file) {
  requireFields(resp, ["mode", "seed", "score", "total", "cardResults", "contractVersion"], file);
  if (!Array.isArray(resp.cardResults)) fail(file, "cardResults must be array");
  else for (const cr of resp.cardResults) {
    if (typeof cr.flashcardId !== "number") fail(file, "cardResult.flashcardId must be number");
    if (typeof cr.correct !== "boolean") fail(file, "cardResult.correct must be boolean");
  }
}
const files = fs.readdirSync(DIR).filter(f => f.endsWith(".json") && f !== "error-envelope.json");
let total = 0;
for (const file of files) {
  total++;
  try {
    const data = JSON.parse(fs.readFileSync(path.join(DIR, file), "utf-8"));
    if (data.response && data.response.body) {
      if (file.includes("-generate")) validateGenerate(data.response.body, file);
      else if (file.includes("-evaluate")) validateEvaluate(data.response.body, file);
    }
  } catch (e) { fail(file, `invalid JSON: ${e.message}`); }
}
total++;
try {
  const data = JSON.parse(fs.readFileSync(path.join(DIR, "error-envelope.json"), "utf-8"));
  for (const [k, v] of Object.entries(data)) {
    if (k === "_comment") continue;
    if (v.body) requireFields(v.body, ["code", "message", "requestId", "details"], `error:${k}`);
  }
} catch (e) { fail("error-envelope.json", `invalid JSON: ${e.message}`); }
process.stdout.write(`\nValidated ${total} golden example files.\n`);
if (errors.length > 0) { process.stderr.write(`\n${errors.length} error(s):\n${errors.join("\n")}\n`); process.exit(1); }
process.stdout.write("All golden examples PASS.\n");
process.exit(0);
