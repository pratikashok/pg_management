# PG Management System API

This project is a small Go API for PG tenant management using PostgreSQL.

## Endpoints

- `GET /health`
- `GET /api/v1/tenants`
- `POST /api/v1/tenants`

## Tenant payload

```json
{
  "full_name": "Rahul Sharma",
  "phone": "+919876543210",
  "email": "rahul@example.com",
  "room_number": "A-203",
  "monthly_rent": 8500,
  "security_deposit": 15000,
  "check_in_date": "2026-04-16"
}
```

## Local setup

1. Create a PostgreSQL database named `pg_management`.
2. Copy `.env.example` values into your environment.
3. Run:

```bash
go mod tidy
go run .
```

The app auto-creates the `tenants` table on startup.

## Render deployment

Render is the simplest fit for what you asked now:

- connect GitHub
- deploy on every push
- use PostgreSQL
- accept that the free web service sleeps and the free database expires later

### Current free-tier limits on Render

As of April 16, 2026:

- Free web services spin down after 15 minutes of inactivity.
- A request wakes the app back up, usually with a short delay.
- Free Render Postgres expires 30 days after creation.
- After expiry, Render gives a 14-day grace period before deleting the database.

Official docs:

- https://render.com/docs/free
- https://render.com/docs/deploy-go-nethttp

### Fastest way to deploy

1. Push this project to a GitHub repository.
2. Sign in to Render with GitHub.
3. In Render, choose `New +` -> `Blueprint`.
4. Select your repository.
5. Render will detect [render.yaml](<d:\Go for it\start\render.yaml:1>) and create:
   - one free Go web service
   - one free PostgreSQL database
6. Wait for the first deploy to finish.
7. Open the generated app URL:

```text
https://your-service-name.onrender.com/health
```

### Auto deploy behavior

After the first setup, every push to your connected GitHub branch triggers a new deploy automatically.

### Manual Render setup if you don't want Blueprint

1. Create a new `Postgres` database on Render, plan `Free`.
2. Create a new `Web Service` from your GitHub repo.
3. Use:

```text
Environment: Go
Build Command: go build -tags netgo -ldflags '-s -w' -o app .
Start Command: ./app
```

4. Add env var `DATABASE_URL` from the Render Postgres connection string.

## Local Docker

Build and run with Docker:

```bash
docker build -t pg-management-system .
docker run --env-file .env -p 8080:8080 pg-management-system
```

Or use Docker Compose with PostgreSQL:

```bash
docker compose up --build
```

## Example requests

Create tenant:

```bash
curl -X POST http://localhost:8080/api/v1/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "full_name":"Rahul Sharma",
    "phone":"+919876543210",
    "email":"rahul@example.com",
    "room_number":"A-203",
    "monthly_rent":8500,
    "security_deposit":15000,
    "check_in_date":"2026-04-16"
  }'
```

List tenants:

```bash
curl http://localhost:8080/api/v1/tenants
```

## Postman

Import `postman/pg-management.postman_collection.json` and set `base_url` to your Render URL, for example:

```text
https://your-service-name.onrender.com
```
