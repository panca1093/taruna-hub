## Slice

<!-- e.g. P13, or P07b for part of a slice; "none" for a change outside the plan -->

## What changed

## Plan update

- [ ] `requirement/PLAN.md`: the slice's **Status** line is updated (`done`, `done, lean` or `in progress`, with what is deferred)
- [ ] Acceptance boxes this PR proves are ticked; every box left open has a one-line reason under it
- [ ] Decisions resolved in this PR are recorded in ARCHITECTURE §9

## Checks

- [ ] `cd api && make build vet test`
- [ ] `cd web && npm run gen:api && git diff --exit-code lib/api/schema.ts`
- [ ] `cd web && npx tsc --noEmit && npm run lint && npm run build`
- [ ] Browser → API → database demo (and DESIGN §10 for UI changes)
