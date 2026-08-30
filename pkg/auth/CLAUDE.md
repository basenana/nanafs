# pkg/auth

## Purpose

Authentication and authorization for NanaFS. Provides JWT (HS256) token generation/parsing with namespace-bound claims, plus a Google OAuth2 login/register service that manages users and namespaces through the metadata store.

## Key Files

| File | Responsibility |
|------|----------------|
| `claims.go` | `Claims` struct (Namespace, UID, GID, Email, GoogleID embedded in `jwt.RegisteredClaims`), `NewClaims()`/`NewClaimsWithUser()`, `GenerateToken()`, `ParseToken()` |
| `auth.go` | Sentinel errors: `ErrInvalidToken`, `ErrExpiredToken` |
| `google.go` | `GoogleOAuthConfig`, `GoogleUserInfo`, `GoogleAuthService` with the full OAuth2 flow and user/namespace management |

## Core Capabilities

**JWT:**
- `NewClaims(namespace string, uid, gid int64, duration time.Duration) *Claims`
- `NewClaimsWithUser(namespace string, uid, gid int64, email, googleID string, duration time.Duration) *Claims`
- `Claims.GenerateToken(secretKey string) (string, error)`
- `ParseToken(tokenString, secretKey string) (*Claims, error)`

**`GoogleAuthService` methods:**
- `GetAuthURL(state string) string`
- `ExchangeCode(ctx, code string) (*GoogleUserInfo, error)`
- `LoginOrRegister(ctx, userInfo *GoogleUserInfo) (*Claims, *types.User, error)`
- `CreateUserWithNamespace(ctx, userInfo *GoogleUserInfo, namespaceName string) (*Claims, *types.User, error)`
- `GenerateToken(claims *Claims) (string, error)`
- `GetUser` / `GetUserByNamespace` / `GetNamespace` / `ListNamespaces` / `DeleteNamespace`

**Sentinel errors:** `ErrInvalidToken`, `ErrExpiredToken`, `ErrUserRequiresNamespace`, `ErrNamespaceAlreadyExists`, `ErrEmailAlreadyLinked`.

## Upstream (Consumers)

- `cmd/apps/root.go`
- `cmd/apps/apis/rest/v1/base.go`, `cmd/apps/apis/rest/v1/user.go`
- `cmd/apps/apis/rest/common/auth.go` (JWT middleware)

## Downstream (Dependencies)

- `github.com/basenana/nanafs/pkg/metastore` (user/namespace persistence via `UserStore`/`NamespaceStore`)
- `github.com/basenana/nanafs/pkg/types`
- External: `github.com/golang-jwt/jwt/v5`, `golang.org/x/oauth2`, `golang.org/x/oauth2/google`

## Design Notes

- Hybrid auth: Google OAuth2 for the login flow, JWT for subsequent requests. Claims carry the namespace so every authenticated request is namespace-scoped (multi-tenancy).
- Two-phase registration: `LoginOrRegister` returns `ErrUserRequiresNamespace` when the Google identity exists but has no namespace; `CreateUserWithNamespace` completes registration.

## Testing

- `jwt_test.go`, `google_test.go` — standard Go `testing` with `testify/assert` and `testify/require` (no Ginkgo suite here).
