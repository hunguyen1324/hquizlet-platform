# quiz-core Golden Test Vectors — Phase 3

**Version:** quiz-core v0.1.0  
**PRNG:** xorshift64 (see `crates/quiz-core/src/lib.rs`)  
**Owner:** Dev 4  
**Status:** PINNED — không thay đổi không có ADR kèm theo

Các vectors này phải được implement đúng trong mọi port (TypeScript/WASM/Go binding).
Nếu một implementation trả giá trị khác → đó là bug trong implementation đó, không phải Rust.

---

## seeded_shuffle — Input `[0,1,2,3,4,5,6,7]`

| seed    | expected output           |
|---------|---------------------------|
| `1`     | `[4, 2, 5, 7, 0, 3, 6, 1]` |
| `42`    | `[6, 4, 5, 7, 1, 0, 3, 2]` |
| `12345` | `[7, 6, 4, 2, 0, 5, 3, 1]` |

**Edge cases:**
- Input rỗng `[]` → `[]`
- Input 1 phần tử `[x]` → `[x]`
- seed=0 được normalize thành 1 (không panic)

---

## score_answer

| expected   | submitted   | result  |
|------------|-------------|---------|
| `"Tokyo"`  | `"Tokyo"`   | `true`  |
| `"Tokyo"`  | `"tokyo"`   | `true`  |
| `"TOKYO"`  | `" tokyo "` | `true`  |
| `"hello"`  | `"  hello  "`| `true` |
| `"Tokyo"`  | `"Osaka"`   | `false` |
| `"Tokyo"`  | `""`        | `false` |

Normalize: `.trim().to_lowercase()` — không typo tolerance.

---

## score_session

| results                          | correct | total | pct |
|----------------------------------|---------|-------|-----|
| 5 correct                        | 5       | 5     | 100 |
| 5 wrong                          | 0       | 5     | 0   |
| 3 correct, 2 wrong               | 3       | 5     | 60  |
| empty                            | 0       | 0     | 0   |
| 110 results (100 correct + 10 wrong) | 100 | 100   | 100 |

> Contract: capped at 100 results, matching backend `total <= 100` constraint.

---

## generate_questions — seed=1, 4 cards `[{id:1..4, term:"term-N", def:"def-N"}]`

- Output: 4 questions
- Mỗi question: `choices[correct_index] == card.definition` ✓
- `qs[0].card.id == 1` (pinned deck order)
- Số choices: `min(4, cards.len())`
- Không có duplicate trong `choices`

---

## TypeScript reference implementation (cho Dev 3/Dev 5 validation)

```typescript
// xorshift64 — must produce identical output to Rust
function xorshift64(state: bigint): bigint {
  state ^= state << 13n;
  state ^= state >> 7n;
  state ^= state << 17n;
  // Mask to u64
  return state & 0xFFFFFFFFFFFFFFFFn;
}

function seededShuffle<T>(items: T[], seed: number): T[] {
  const result = [...items];
  if (result.length <= 1) return result;
  let state = BigInt(seed === 0 ? 1 : seed);
  for (let i = result.length - 1; i > 0; i--) {
    state = xorshift64(state);
    const j = Number(state % BigInt(i + 1));
    [result[i], result[j]] = [result[j], result[i]];
  }
  return result;
}
```

> ⚠️ JavaScript BigInt là signed 64-bit. Cần mask `& 0xFFFFFFFFFFFFFFFFn` sau mỗi XOR để giữ đúng u64 semantics.
