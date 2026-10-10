# Sluice

Percentage-based money allocation with reconciliation.

You set rules once — "every deposit: 25% to savings, 33% to Roth, 10% to the credit card" — and Sluice computes the split for each paycheck, tells you exactly what to transfer, then checks next sync whether it actually happened.

**Read-only. Sluice never moves money.** Moving funds between institutions you don't own is money transmission, which requires licensing and a bank partnership. Sluice reads balances and transactions, does the math, and tells you what to do. You tap through the transfers yourself.

---

## Why this exists

Fixed-dollar budgeting breaks on irregular income. Seasonal work, internships, variable hours — the paycheck changes, so "put $200 in savings" is either too much or leaves money idle. Percentages scale automatically.

The part no other budgeting app does: **check whether the plan survived contact with reality.** Everyone tells you the plan. Almost nobody reconciles it.

---

## Core concepts

**Allocation rule** — a percentage and a destination. Rules belong to a ruleset; a ruleset's percentages sum to 100.

**Expected transfer** — what Sluice computed you should move after a deposit landed. Has a target account, an amount, and a due date.

**Reconciliation** — matching expected transfers against transactions that actually appeared. The gap between the two is the product.

**Drift** — cumulative difference between where your money should be and where it is. The number on the dashboard.

---

## Build order

Frontend first, against mock data. Backend as it goes.

The reason: the data model is easier to get right once you've built the screens that consume it. Designing the ledger in the abstract leads to tables you refactor twice. Build the UI against fake JSON, find out what shape you actually need, then make it real.

### Phase 1 — Frontend on mocks

No backend. No database. No Plaid. Hardcoded JSON in the repo.

- [x] Vite + React + TypeScript, Tailwind
- [x] `src/mocks/` — fake accounts, deposits, rules, expected transfers
- [ ] Rules editor — add/remove/reweight allocations, enforce 100% sum
- [ ] Dashboard — accounts, balances, drift
- [ ] Deposit view — "$300 landed, here's the split"
- [ ] Transfer checklist — what to move, where, by when
- [ ] Reconciliation view — expected vs actual, flag the misses
- [ ] Projections — compound growth on savings, payoff curve on the card

By the end of this phase the app is fully usable with fake data, and the TypeScript types in `src/types/` *are* the API contract. Backend implements them.

### Phase 2 — Backend, in-memory

Go + chi (or FastAPI if Go is fighting you — decide once, don't switch midway).

- [x] Serve the Phase 1 types from real endpoints
- [x] Allocation engine — deposit in, expected transfers out
- [x] Rounding: cents must sum exactly to the deposit, no lost penny
- [x] Still no DB — in-memory store behind an interface

### Phase 3 — Persistence

- [ ] Postgres, migrations
- [ ] Double-entry ledger (see below)
- [ ] Rules versioned — editing a rule must not rewrite history

### Phase 4 — Real data

- [ ] Plaid Link, sandbox institutions first
- [ ] Nightly balance + transaction sync
- [ ] Webhook handler, idempotent — Plaid will deliver twice
- [ ] Deposit detection: which incoming transactions count as income?

### Phase 5 — Reconciliation

The actual product. Everything before this is setup.

- [ ] Match expected transfers to observed transactions
- [ ] Fuzzy matching — amounts differ by fees, dates by days
- [ ] Drift tracking over time
- [ ] Roth YTD contribution cap, with overflow redirect
- [ ] Nag on missed transfers

### Phase 6 — Deploy

- [ ] Docker, compose for local
- [ ] CI: test, lint, build
- [ ] Deploy somewhere cheap
- [ ] Uptime check + alerting
- [ ] Structured logs, basic metrics

---

## Non-negotiables

**Money is integer cents or `NUMERIC`. Never a float.** `0.1 + 0.2 != 0.3`, and a ledger that doesn't balance is a weekend you don't get back. Go: `shopspring/decimal`. Python: `Decimal`. JS: integer cents, format at render.

**Double-entry.** Every movement is two rows, debit and credit. A check constraint enforcing that they sum to zero catches entire categories of bug at the database layer instead of in production.

**Rounding is explicit.** A $300 deposit split 33/33/34 has to allocate exactly 30000 cents. Pick a strategy — largest remainder is the usual one — and test it.

**Idempotency everywhere.** Webhooks retry. Sync jobs overlap. Every write path takes a key and no-ops on replay.

**Immutable history.** Changing a rule today must not alter what Sluice told you to do last month. Rules get versions; expected transfers reference the version that produced them.

---

## Stack

| | |
|---|---|
| Frontend | React, TypeScript, Vite, Tailwind |
| Backend | Go + chi |
| DB | Postgres |
| Bank data | Plaid (read-only, sandbox first) |
| Charts | Recharts |

---

## Running

```bash
# Frontend (http://localhost:5173)
cd web
npm install
npm run dev
```

```bash
# Backend (http://localhost:8080)
cd backend
go run .
go test ./...
```

Endpoints: `GET /api/health`. Set `PORT` to override the default.

---

## Scope

**In:** allocation rules, split computation, transfer checklists, reconciliation, drift, projections, contribution caps.

**Out:** moving money. Budgeting categories. Expense tracking. Bill negotiation. Anything requiring a license.