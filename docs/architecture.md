# Backend architecture

Pocket is a modular monolith organized around business domains. It deploys as
one process for the MVP, but each domain has explicit ports around persistence
and authorization. This keeps the current runtime simple and makes a later
service extraction a deployment decision rather than a full rewrite.

## Dependency direction

```text
cmd/api
  -> bootstrap (composition root)
    -> httpapi (chi routes and transport contracts)
    -> modules/* services (use cases and business rules)
      -> repository interfaces and policy interfaces
        <- PostgreSQL repository adapters and access policies
```

Dependencies point inward. A service can depend on a repository interface, but
it cannot depend on `pgx`, HTTP, cookies, or another domain module.

The rules are checked by `internal/architecture/modules_test.go`:

- domain modules cannot import one another;
- PostgreSQL imports inside a domain are limited to
  `repository_postgres.go`.

## Package responsibilities

### `cmd/api`

Owns process lifecycle only: configuration, database connection, HTTP server,
signals, and graceful shutdown.

### `internal/bootstrap`

Is the composition root. It constructs PostgreSQL adapters, policies, services,
and the HTTP handler. Concrete dependencies should be wired here rather than in
handlers or domain services.

### `internal/httpapi`

Owns the public HTTP boundary:

- `chi` route definitions and middleware;
- request decoding and response encoding;
- cookies, status codes, and stable error envelopes;
- small service interfaces used by handlers.

The `/api/v1` routes and JSON contracts remain unchanged when internal modules
are reorganized.

### `internal/modules/<domain>`

Each bounded context follows the same shape:

```text
model.go                  domain inputs and outputs
service.go                use cases, validation, business policy
repository.go             persistence port, when shared records are needed
repository_postgres.go    PostgreSQL adapter and transactions
<feature>.go              larger use cases split by capability
```

Repository interfaces live beside the service that consumes them. This follows
the dependency inversion rule and keeps a future remote adapter possible without
changing the service.

## Bounded contexts

| Module | Owns |
| --- | --- |
| `identity` | Accounts, encrypted personal data, credentials, sessions, login throttling |
| `venues` | Venue profile and lifecycle |
| `catalog` | Menu categories, items, prices, availability, images |
| `workforce` | Invitations, venue roles, staff lifecycle |
| `ordering` | Orders and order status transitions |
| `feedback` | Reviews and venue replies |
| `billing` | Payments and workspace subscriptions |
| `floorplan` | Editable venue layout document |
| `reporting` | Read-only owner dashboard projection |

Modules do not call one another directly. Cross-domain composition happens in
`bootstrap`, while shared authorization reads are exposed through narrow
interfaces.

## Shared policies

`access.VenueAuthorizer` implements the ownership policy for venue-scoped
operations. Modules consume only its `RequireOwner` method. It can later be
replaced by signed claims, an RPC client, or a local ownership projection.

`access.CapabilityReader` provides identity with an account capability
projection without making identity import venue or workforce packages.

`appfault` contains transport-independent errors shared by business modules.
The HTTP layer maps them to status codes and response envelopes.

## Database ownership

All modules currently share one PostgreSQL cluster. Tables still have a logical
owner, and command-side SQL belongs in that owner's PostgreSQL adapter.

Cross-domain joins are reserved for explicit read models such as `reporting` and
access projections. New command paths should communicate through ports or
events instead of adding cross-domain joins.

Transactions belong in repository adapters. Validation, password hashing,
encryption, and decisions such as subscription limits stay in services.

### Current integration seams

The MVP still has a few deliberate same-database integrations:

- venue onboarding promotes the account to `venue_owner`;
- billing reads the owner's active venue count before changing a plan;
- floor plans are stored in the venue settings document;
- reporting and access build cross-domain read projections.

These dependencies are behind repository or policy ports. During extraction,
replace the first three with events, a dedicated projection, or a remote adapter
without changing the calling service.

## Extraction path

To extract a domain into a microservice:

1. Give it migration ownership and, if needed, a dedicated schema.
2. Add an outbox for events produced by state changes.
3. Move the domain package and its PostgreSQL adapter into a service repository.
4. Add an HTTP or gRPC adapter implementing the existing repository/service
   port required by the modular monolith.
5. Replace shared reads with local projections populated from events.
6. Move traffic only after compatibility, retries, tracing, and operational
   ownership are in place.

## Adding functionality

- Put business decisions in the owning module's service.
- Put SQL and transaction boundaries in its repository adapter.
- Keep HTTP concerns in `httpapi`.
- Wire concrete implementations only in `bootstrap`.
- Introduce a new bounded context instead of growing an unrelated service.
- Add an architecture rule when a new dependency boundary matters.
