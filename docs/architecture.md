# Backend architecture

Pocket is a modular monolith. It deploys as one process today, while its code is
organized around business capabilities rather than technical layers or user
roles. This keeps the MVP operationally simple without coupling all features to
one service object.

## Dependency direction

```text
cmd/api
  -> httpapi contracts and handlers
  -> modules/* services
  -> access policies and shared platform packages
  -> PostgreSQL
```

Domain modules under `internal/modules` must not import one another. The rule is
enforced by `internal/architecture/modules_test.go`. Cross-domain composition
happens only in `cmd/api` and `httpapi`.

## Bounded contexts

| Module | Owns |
| --- | --- |
| `identity` | Accounts, credentials, sessions, login throttling |
| `venues` | Venue profile and lifecycle |
| `catalog` | Menu categories, items, prices, availability, images |
| `workforce` | Invitations, venue roles, staff lifecycle |
| `ordering` | Orders and order status transitions |
| `feedback` | Reviews and venue replies |
| `billing` | Payments and workspace subscriptions |
| `floorplan` | Editable venue layout document |
| `reporting` | Read-only owner dashboard projection |

Each module owns its request models, response models, validation, and SQL. The
HTTP layer depends on small interfaces declared in `httpapi/contracts.go`; it
does not depend on concrete service types.

## Shared policies

`access.VenueAuthorizer` is the current ownership policy for venue-scoped
operations. Modules depend only on its `RequireOwner` method. When a module is
extracted, this implementation can be replaced by signed claims, an RPC check,
or a local ownership projection.

`access.CapabilityReader` keeps identity from querying venue and workforce data
directly. It builds the current account capability projection behind an
interface consumed by the identity module.

`appfault` contains the small set of transport-independent errors shared by the
business modules. HTTP maps them to stable status codes and response envelopes.

## Database ownership

All modules currently share one PostgreSQL cluster because transactions and
operations are still local to the MVP. Tables nevertheless have a clear logical
owner. New code must access a table only from its owning module, except for
explicit read projections in `access` and `reporting`.

Do not add cross-module SQL joins to command paths. Reporting joins are allowed
for read models. As scale requires it, replace them with projections populated
from domain events.

## Extraction path

To turn a module into a microservice:

1. Give the module its own database schema and migration ownership.
2. Add an outbox for events produced by its state changes.
3. Move the module directory and its models into a new service repository.
4. Implement the existing `httpapi` interface with an HTTP or gRPC client.
5. Replace cross-domain reads with local projections built from events.
6. Route external traffic directly only after compatibility and observability
   are in place.

The public `/api/v1` routes and JSON envelopes are intentionally unchanged by
the modularization, so extraction can happen behind the existing API gateway.

## Adding functionality

- Put business rules and persistence in the module that owns the capability.
- Expose the smallest interface required by the transport layer.
- Keep HTTP parsing, cookies, status codes, and response envelopes in `httpapi`.
- Compose concrete implementations only in `cmd/api`.
- Add a new module instead of extending an unrelated service with more methods.
