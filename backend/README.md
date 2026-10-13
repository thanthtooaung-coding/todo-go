# TodoList Backend API

**Architecture:** Feature-oriented layered monolith  
**Language:** Go  
**HTTP framework:** Gin  
**ORM:** GORM  
**Database:** PostgreSQL  
**API style:** RESTful HTTP API with JSON  
**Deployment:** One backend application

## Overview

The TodoList backend is a Go REST API for managing todo items. It uses Gin for HTTP handling, GORM for database access, and PostgreSQL for persistence.

The backend follows a **feature-oriented layered monolith**: one deployable application, organized into feature modules with separate technical packages. This is conventional layered architecture, not a requirement to use Domain-Driven Design (DDD).

This README defines the architecture and implementation conventions. The implemented contract is in [Implemented API](#implemented-api). Authentication and user ownership are not part of that contract.

## Technology stack

| Concern | Choice |
|---|---|
| Language | Go |
| HTTP framework | Gin |
| ORM | GORM |
| Database | PostgreSQL |
| API | REST / JSON |
| Configuration | Environment variables |
| Schema changes | Versioned database migrations |
| Testing | Go unit tests, PostgreSQL integration tests, HTTP tests |
| Deployment | One backend application; Docker may be used |

Use the project's existing libraries where possible. Select supporting libraries for validation, logging, migrations, authentication, and observability based on actual requirements. Avoid adding overlapping dependencies without a clear reason.

## Project structure

Use this as the intended starting point. Create packages and files when they are needed; do not add empty folders or placeholder files solely to match the tree.

```text
todo-list-backend/
├── cmd/
│   └── server/
├── internal/
│   ├── bootstrap/
│   ├── config/
│   ├── database/
│   ├── middleware/
│   ├── security/
│   ├── logging/
│   ├── shared/
│   │   ├── errors/
│   │   ├── responses/
│   │   └── validation/
│   └── modules/
│       ├── todo/
│       │   ├── handler/
│       │   ├── mapper/
│       │   ├── models/
│       │   ├── repository/
│       │   ├── request/
│       │   ├── response/
│       │   ├── routes/
│       │   └── service/
│       └── user/
│           ├── handler/
│           ├── mapper/
│           ├── models/
│           ├── repository/
│           ├── request/
│           ├── response/
│           ├── routes/
│           └── service/
├── migrations/
├── api-doc/
├── deployments/
├── scripts/
├── test/
├── go.mod
├── go.sum
├── Dockerfile
├── compose.yaml
├── .env.example
└── README.md
```

`todo` is the primary example module. Keep the `user` module only if user accounts, ownership, or authentication are part of the agreed scope. Do not implement authentication just because a user module appears in this example tree.

## Module package responsibilities

| Package | Responsibility |
|---|---|
| `routes/` | Register module endpoints and connect routes to handlers |
| `handler/` | Handle Gin requests, invoke services, and map outcomes to HTTP responses |
| `request/` | Define request DTOs and request-specific validation structures |
| `response/` | Define response DTOs and API response structures |
| `mapper/` | Convert between request/response DTOs, models, and service data where useful |
| `service/` | Implement application logic and coordinate repository operations |
| `repository/` | Encapsulate GORM queries and persistence operations |
| `models/` | Define module data structures and GORM persistence models where appropriate |

Keep the package names `models`, `request`, `response`, `mapper`, `repository`, `service`, `handler`, and `routes`. Do not rename `models/` to `model/`, merge request and response DTOs into a generic `dto/` package, or remove/merge these layers without a concrete reason and approval.

Not every operation needs a mapper or a unique type for every layer. Use mapping when it improves clarity, protects API/database boundaries, or avoids coupling. Avoid redundant conversions that add no value.

## Request flow

A typical request follows this path:

```text
Module routes
     |
     v
Module handler <---- request DTO
     |
     v
Mapper (when useful)
     |
     v
Module service
     |
     v
Module repository
     |
     v
GORM / PostgreSQL

Service result -> mapper (when useful) -> response DTO -> handler -> HTTP response
```

These are responsibilities, not mandatory sequential stages for every operation.

### `routes/`

- Register todo endpoints and attach required middleware.
- Keep routing declarations focused on HTTP wiring.
- Do not put application logic or database access in route registration.

### `handler/`

- Parse path parameters, query parameters, and request bodies.
- Validate transport input, invoke the service, and return the appropriate HTTP response.
- Keep handlers thin.
- Do not execute GORM queries directly.
- Keep `*gin.Context` and HTTP-specific concerns in this layer.

### `request/`

- Define request DTOs for creating, updating, filtering, or changing todo state as required.
- Validate incoming data before application use.
- Do not use request DTOs as GORM persistence models.
- Do not allow clients to update server-managed fields unintentionally.

### `response/`

- Define response DTOs for todo items, lists, and API errors according to the agreed API contract.
- Return only fields intended for the client.
- Do not expose internal database fields or sensitive user information.
- Do not reuse request DTOs as response DTOs merely to reduce the number of types.

### `mapper/`

- Convert between request DTOs, service data, persistence models, and response DTOs when useful.
- Keep mapping functions focused on data transformation.
- Do not put HTTP behavior, database access, or application logic in mappers.

### `service/`

- Implement application operations such as creating, listing, retrieving, updating, completing, and deleting todos when these operations are in scope.
- Coordinate repository calls and apply agreed application rules.
- Return results and errors rather than writing HTTP responses.
- Do not depend on Gin types.
- Keep service methods cohesive and testable.

### `repository/`

- Encapsulate GORM and PostgreSQL operations.
- Support required retrieval, filtering, sorting, and pagination behavior.
- Handle persistence-specific errors and return errors to services.
- Use parameterized query APIs; never concatenate untrusted input into SQL.

### `models/`

- Define todo persistence models and related data structures.
- Use GORM tags intentionally.
- Keep API DTOs separate from persistence models when combining them would expose database details or create unwanted coupling.
- Do not require separate domain and persistence representations everywhere.

## Suggested Todo API capabilities

Finalize routes, field names, status codes, and validation rules before treating this list as the API contract.

| Capability | Typical HTTP method | Example path |
|---|---|---|
| Create todo | `POST` | `/api/v1/todos` |
| List todos | `GET` | `/api/v1/todos` |
| Retrieve a todo | `GET` | `/api/v1/todos/{id}` |
| Update a todo | `PATCH` or `PUT` | `/api/v1/todos/{id}` |
| Mark complete/incomplete | `PATCH` | `/api/v1/todos/{id}/completion` |
| Delete a todo | `DELETE` | `/api/v1/todos/{id}` |

These are proposed REST conventions, not a claim that these routes already exist. Prefer a consistent API versioning and error-response convention.

### List behavior

If the application needs list controls, define them explicitly. Common options include:

- Pagination using `page` and `page_size` or cursor-based pagination.
- Filtering by completion status.
- Searching by title.
- Sorting by creation date, due date, or title.

Validate pagination and sorting inputs. Never interpolate untrusted sort columns directly into SQL; map accepted sort options to known columns.

## Data model considerations

The exact schema depends on requirements. A todo item may need fields such as an identifier, title, description, completion state, optional due date, and creation/update timestamps. User ownership should be added only if the application supports accounts or multiple users.

Do not assume or invent additional fields, status transitions, permissions, or deletion behavior without confirming requirements. Use database constraints and indexes where they support agreed integrity rules and access patterns.

## Dependency rules

Preferred request dependency flow:

`Routes → Handler → Service → Repository → GORM/PostgreSQL`

- Lower layers must not import higher layers.
- Services and repositories must not depend on Gin.
- Handlers must not bypass services/repositories to query the database.
- Avoid circular imports.
- If multiple modules are implemented, avoid importing another module's repository implementation directly; use a narrow application/service interface where appropriate.
- Wire concrete dependencies explicitly during bootstrap rather than using global service locators.

## Go coding conventions

- Format Go code with `gofmt`; use `goimports` if adopted by the repository.
- Follow idiomatic Go naming and package conventions.
- Keep functions focused and control flow readable.
- Prefer early returns when they improve clarity.
- Handle errors deliberately; wrap errors with `%w` when preserving the underlying error is useful.
- Do not use `panic` for ordinary application errors or silently ignore errors.
- Comment exported declarations and non-obvious behavior.
- Avoid unnecessary global mutable state, magic values, premature optimization, and needless abstractions.
- Follow existing project conventions before introducing a new pattern.

## Database and migrations

- Use PostgreSQL as the primary database and GORM for persistence.
- Configure database connections, pool settings, and timeouts centrally.
- Use transactions for operations that must be atomic.
- Manage schema changes through reviewed, version-controlled migrations.
- Do not automatically apply unreviewed schema changes during production startup.
- Add indexes and constraints based on confirmed requirements and query patterns.
- Test important repository behavior against PostgreSQL, especially constraints and transactions.
- Never commit database credentials or production secrets.

## HTTP errors and security

- Use consistent JSON response and error conventions.
- Map application errors to HTTP status codes at the handler boundary.
- Distinguish invalid input, not found, conflict, unauthorized/forbidden, and unexpected infrastructure failures where applicable.
- Never expose stack traces, raw SQL, credentials, or internal implementation details to clients.
- Validate all untrusted request data.
- If todos belong to users, enforce ownership/authorization in the service or an appropriate authorization boundary; do not rely only on a todo ID being difficult to guess.
- Keep secrets in environment-specific configuration or a secret manager.
- If authentication is implemented, use established security practices and do not store plaintext passwords.
- Redact tokens, passwords, and sensitive personal data from logs.

## Configuration and observability

- Load configuration from environment variables or the project's established configuration mechanism.
- Validate required configuration at startup and fail clearly when required values are missing or invalid.
- Keep `.env.example` limited to variable names and safe example values.
- Use structured logging and request IDs where available.
- Use context cancellation and timeouts for operations that may block.
- Add health/readiness checks and graceful shutdown.
- Add metrics or tracing only when required by operational needs.

## Testing strategy

- **Unit tests:** test service behavior, validation, and error paths.
- **Repository integration tests:** test GORM queries, PostgreSQL constraints, migrations, and transactions.
- **HTTP tests:** test routes, request validation, status codes, and response formats.
- Test successful and failure paths.
- Keep tests deterministic and independent of production data.
- Run relevant tests and formatting checks after changes; report checks that could not be run.

## Development and deployment

Finalize the supported Go version, dependency versions, environment variable names, and local commands in the implementation repository.

1. Initialize the Go module and choose the supported Go version.
2. Establish configuration loading and validation.
3. Configure logging and PostgreSQL/GORM.
4. Choose and document the migration process.
5. Set up Gin routes, middleware, and standardized errors.
6. Implement the todo module's routes, handlers, request/response DTOs, mappers, services, and repositories as needed.
7. Wire dependencies in bootstrap code.
8. Add unit, integration, and HTTP tests.
9. Add reproducible build and Docker/local development configuration.
10. Document environment variables, migrations, and operational tasks.

Deploy the API as one Go application. PostgreSQL may run as a separate infrastructure service. Do not split each module into its own service unless the architecture is explicitly changed.

## Scope

Authentication and user ownership are out of scope. The running API below is the implemented contract for a single shared todo list.

## Implemented API

Go 1.26. Run commands from the `backend` directory.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/todos` | Create a todo |
| `GET` | `/api/v1/todos` | List todos |
| `GET` | `/api/v1/todos/{id}` | Retrieve a todo |
| `PATCH` | `/api/v1/todos/{id}` | Update title, description, or due date |
| `PATCH` | `/api/v1/todos/{id}/completion` | Mark complete or incomplete |
| `DELETE` | `/api/v1/todos/{id}` | Permanently delete a todo |
| `GET` | `/healthz` | Liveness |
| `GET` | `/readyz` | Readiness, including a database ping |

A todo has `id`, `title`, `description`, `completed`, `due_date`, `created_at`, and `updated_at`. `title` is required and at most 200 characters. `description` is at most 2000 characters. `due_date` is an optional RFC3339 timestamp. Completion changes only through the completion endpoint. Clients cannot set `id`, `completed`, `created_at`, or `updated_at` on create or update.

List query parameters:

| Parameter | Default | Rule |
|---|---|---|
| `page` | `1` | Integer from 1 to 10000 |
| `page_size` | `20` | Integer from 1 to 100 |
| `completed` | unset | `true` or `false` |
| `q` | unset | Case-insensitive title search, at most 200 characters |
| `sort` | `created_at` | `created_at`, `due_date`, or `title` |
| `order` | `desc` | `asc` or `desc` |

Successful responses use `{"data": ...}`. Lists also include `pagination`. Errors use `{"error":{"code","message","details"}}`. Request bodies are limited to 1 MiB. The API is open: it does not authenticate callers.

### Local development

1. Copy `.env.example` to `.env` and adjust it if needed. Set `DATABASE_URL` to use a hosted Postgres URL instead of the separate `DB_*` fields.
2. Start PostgreSQL, or run only the database service: `docker compose up postgres`.
3. Apply migrations: `go run ./cmd/migrate up`.
4. Start the API: `go run ./cmd/server`.

`docker compose up --build` starts PostgreSQL, applies migrations, and then starts the API on port 8080. The server does not apply schema changes on startup. If `public.todos` is missing, the process exits with an error.

Migration files live in `migrations/` and are recorded in `schema_migrations`. `go run ./cmd/migrate down` rolls back the latest migration. `go run ./cmd/migrate version` prints the latest applied version.

### Tests

```text
gofmt -l .
go test ./...
```

Repository tests that need PostgreSQL:

```text
TODO_TEST_DATABASE_URL=postgres://todo:todo@localhost:5432/todo?sslmode=disable go test -tags=integration ./internal/modules/todo/repository
```

The OpenAPI document is `api-doc/openapi.yaml`. Environment variables are listed in `.env.example`.
