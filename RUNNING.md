# Running the ARR Tracker

## First-time setup (do this once)

1. Extract the project and navigate to it:
   ```bash
   tar -xzf arr-tracker.tar.gz
   cd arr-tracker
   ```

2. Create your `.env` file from the template:
   ```bash
   cp .env.example .env
   nano .env
   ```
   Fill in your `CAMPFIRE_API_KEY` and `DATABASE_URL`. Save with `Ctrl+O`, `Enter`, `Ctrl+X`.

3. Install Go dependencies (one time only):
   ```bash
   go mod tidy
   ```

4. Install frontend dependencies (one time only):
   ```bash
   cd web
   npm install
   cd ..
   ```

---

## Every time you want to run it

You need **two terminal windows**. Open each one and navigate to the project first.

**Terminal 1 — Go backend:**
```bash
cd ~/arr-tracker
export $(cat .env | grep -v '#' | xargs)
go run main.go
```

You should see:
```
Database connected and migrated
Server listening on :8080
Scheduler: running initial sync...
```

**Terminal 2 — React frontend:**
```bash
cd ~/arr-tracker/web
npm run dev
```

> `~` is a shortcut for your home directory (e.g. `/Users/dan`). If you placed the project somewhere else, replace `~/arr-tracker` with the full path to where your project lives.

Then open your browser to **http://localhost:5173**

---

## Stopping the app

- In each terminal press `Ctrl+C` to stop the process.

---

## Common issues

**"FATAL: required environment variable CAMPFIRE_API_KEY is not set"**
You opened a new terminal and the env vars weren't loaded. Run:
```bash
export $(cat .env | grep -v '#' | xargs)
```

**"dial tcp ... no route to host"**
Your DATABASE_URL is using the direct connection instead of the Session Pooler.
Go to Supabase → Settings → Database → Connection String → Method: Session Pooler → copy that URL into your `.env`.

**ARR numbers look wrong after a Campfire update**
Hit **Full Sync** in the dashboard (not Sync Now). Full Sync re-fetches everything regardless of last modified date.

**Dashboard shows stale data after a change**
Hard refresh the browser: `Cmd + Shift + R`

---

## Notes on .env

- **Never commit `.env` to GitHub** — it contains your secrets. The `.gitignore` already excludes it.
- `.env.example` is the safe template committed to the repo — it has placeholder values only.
- If you lose your `.env`, recreate it from `.env.example` and re-enter your keys.
