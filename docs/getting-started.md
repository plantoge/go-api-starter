# Getting Started (Windows development)

## 1. Install prerequisites

- Go 1.25+
- PostgreSQL, running locally
- `air` for hot reload: `go install github.com/air-verse/air@latest`

## 2. Create the databases

```powershell
psql -U postgres -c "CREATE ROLE app LOGIN PASSWORD 'changeme';"
psql -U postgres -c "CREATE DATABASE appdb OWNER app;"
psql -U postgres -c "CREATE DATABASE app_test OWNER app;"
```

## 3. Configure environment

```powershell
copy .env.example .env
copy .env.test.example .env.test
```
Adjust `DB_PASSWORD` in both files if you used a different password above.

## 4. Run migrations and create your first admin

```powershell
go run ./cmd/cli migrate platform up
go run ./cmd/cli admin create --email=you@example.com --name="Your Name"
```
Copy the printed password — it's shown once.

## 5. Create your first tenant

```powershell
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acme.test
```
Copy the printed owner password too.

## 6. Run the API

```powershell
air
```
or without hot reload:
```powershell
go run ./cmd/api
```

## 7. Try it

```powershell
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"tenant_code\":\"acme_corp\",\"email\":\"owner@acme.test\",\"password\":\"<owner password>\"}"
```

## Running tests

```powershell
go test ./...
```
Integration tests connect to `app_test` using `.env.test` and are skipped
automatically if PostgreSQL isn't reachable.
