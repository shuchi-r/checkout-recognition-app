# Checkout User Recognition App

Three-layer assignment implementation:

- `web/` — React + TypeScript frontend
- `api/` — Go HTTP API
- `db/` — PostgreSQL schema

## Local development

### 1. Database
Create a PostgreSQL database and run:

```bash
psql "$DATABASE_URL" -f db/schema.sql
```

### 2. API

```bash
cd api
cp .env.example .env
# edit DATABASE_URL, ALLOWED_ORIGIN, SESSION_SECRET

go mod tidy
go run .
```

API defaults to `http://localhost:8080`.

### 3. Frontend

```bash
cd web
npm install
cp .env.example .env
npm run dev
```

Frontend defaults to `http://localhost:5173`.

## Production deployment

Recommended split:

- React static site → Vercel
- Go API → Render
- PostgreSQL → Supabase

Set the frontend `VITE_API_BASE_URL` to the deployed API URL. Set the API `ALLOWED_ORIGIN` to the deployed frontend URL.

Never commit `.env` files or database passwords.
