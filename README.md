# Coder ARR Tracker

Pulls revenue contracts from Campfire, annualizes them to ARR, and displays a live dashboard.

## ARR Methodology

```
ARR = (Total Contract Value ÷ Contract Months) × 12
```

- Uses `total_contract_value` from Campfire (the full committed TCV)
- Consistent with the 85/15 SSP allocation used in ASC 606 recognition
- Non-USD contracts converted at the signing-date locked exchange rate (spot-rate methodology)
- Evergreen contracts with no end date will show ARR = 0 until an end date is set in Campfire

---

## Stack

| Layer      | Tool                                      |
|------------|-------------------------------------------|
| Backend    | Go 1.21 (stdlib only + lib/pq)            |
| Database   | Supabase (Postgres)                       |
| Frontend   | React + Vite                              |
| Hosting    | Railway / Render / Fly.io (backend)       |
|            | Vercel (frontend) or serve from Go static |
| Scheduler  | Built-in Go ticker (24hr)                 |

---

## Local Development

### Prerequisites
- Go 1.21+
- Node 18+
- A Supabase project (free tier works)
- A Campfire API key (Settings → API Keys → Create User [view only])

### 1. Clone and configure

```bash
cp .env.example .env
# Edit .env with your CAMPFIRE_API_KEY and DATABASE_URL
```

### 2. Start the Go backend

```bash
# Install dependencies
go mod tidy

# Run (auto-migrates DB on start, runs initial sync, starts 24hr scheduler)
go run main.go
```

The server starts on `http://localhost:8080`.

### 3. Start the React frontend (separate terminal)

```bash
cd web
npm install
npm run dev
```

Frontend runs on `http://localhost:5173` and proxies `/api` to `:8080`.

---

## API Reference

| Method | Endpoint              | Description                                     |
|--------|-----------------------|-------------------------------------------------|
| GET    | `/api/summary`        | Aggregated ARR, MRR, contract counts            |
| GET    | `/api/contracts`      | All contracts (`?status=ACTIVE\|CHURNED\|ALL`)  |
| POST   | `/api/sync`           | Incremental sync from Campfire                  |
| POST   | `/api/sync?full=true` | Full re-sync (all contracts)                    |
| GET    | `/api/health`         | Liveness check                                  |

---

## Production Deployment

### Option A — Railway (simplest, ~$5/mo)

```bash
# Install Railway CLI
npm install -g @railway/cli
railway login
railway init
railway up
```

Set environment variables in Railway dashboard:
- `CAMPFIRE_API_KEY`
- `DATABASE_URL` (your Supabase connection string)

### Option B — Render

1. Connect your GitHub repo in Render
2. New Web Service → Go → Build command: `go build -o server .` → Start: `./server`
3. Add environment variables

### Frontend on Vercel

```bash
cd web
npm run build
# Deploy dist/ to Vercel, or use:
npx vercel --prod
```

Set `VITE_API_BASE=https://your-backend-url.railway.app` in Vercel env vars.

---

## Supabase Setup

1. Create a new project at supabase.com
2. Go to Settings → Database → Connection string (URI mode)
3. Copy the connection string into `DATABASE_URL`
4. The Go server auto-creates tables on first run via `db.Migrate()`

---

## Sync Behavior

- **Auto-sync**: runs every 24 hours via a background Go goroutine
- **Incremental**: only fetches contracts modified since the last successful sync
  (uses Campfire's `last_modified_at__gte` filter)
- **Manual sync**: click "Sync Now" in the UI, or POST `/api/sync`
- **Full re-sync**: click "Full Sync" in the UI, or POST `/api/sync?full=true`
- All sync runs are logged in the `sync_log` table with timestamp, count, and any errors

---

## Database Schema

```sql
contracts (
  id                   SERIAL PRIMARY KEY,
  campfire_id          INTEGER UNIQUE,   -- Campfire contract ID
  client_name          TEXT,
  deal_name            TEXT,
  deal_id              TEXT,
  status               TEXT,             -- ACTIVE, CHURNED, etc.
  currency             TEXT,             -- source currency
  billing_frequency    TEXT,
  contract_start_date  DATE,
  contract_end_date    DATE,
  closed_date          DATE,
  total_contract_value NUMERIC(18,2),    -- full TCV in contract currency
  total_billed         NUMERIC(18,2),
  total_mrr            NUMERIC(18,2),    -- Campfire-computed MRR (support only)
  arr                  NUMERIC(18,2),    -- TCV/months*12 in contract currency
  arr_usd              NUMERIC(18,2),    -- ARR converted to USD at signing rate
  exchange_rate        NUMERIC(12,6),    -- locked at contract signing date
  contract_months      NUMERIC(8,2),
  is_evergreen         BOOLEAN,
  opportunity_id       TEXT,
  last_modified_at     TIMESTAMPTZ,
  synced_at            TIMESTAMPTZ
)

sync_log (
  id          SERIAL PRIMARY KEY,
  synced_at   TIMESTAMPTZ,
  upserted    INTEGER,
  total       INTEGER,
  incremental BOOLEAN,
  error_msg   TEXT        -- NULL on success
)
```
