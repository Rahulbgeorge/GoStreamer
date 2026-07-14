# StreamingPlayer — Coding Rules & Conventions

> These rules govern all code written in this project.
> Every contributor (human or AI) must follow them.

---

## 1. Philosophy

| Principle | What it means here |
|---|---|
| **Readability first** | Business logic should read almost like plain English. Optimise for the next developer's comprehension, not for nanoseconds. |
| **Loose coupling** | Every layer talks through interfaces. Swapping SQLite for Postgres should never touch a controller. |
| **Thin logging** | Logs exist to help debug — not to drown the reader. Keep log lines out of the happy-path business logic wherever possible. |
| **Progressive complexity** | Start simple. Add abstraction only when a second concrete need appears. |

---

## 2. Project Structure

```
StreamingPlayer/
├── backend/                    # Go + Gin backend
│   ├── cmd/
│   │   └── server/
│   │       └── main.go         # Entry point — wires everything together
│   ├── internal/
│   │   ├── model/              # Domain structs & validation (no DB imports)
│   │   ├── repository/         # Data-access interfaces + implementations
│   │   │   └── sqlite/         # SQLite-specific implementations
│   │   ├── service/            # Business logic (operates on interfaces)
│   │   ├── controller/         # HTTP handlers (thin — parse, delegate, respond)
│   │   ├── middleware/         # Gin middleware (auth, logging, recovery)
│   │   ├── dto/                # Request / Response structs for the API layer
│   │   └── config/             # App configuration loading
│   ├── pkg/                    # Shared utilities (logger, errors, etc.)
│   ├── migrations/             # SQL migration files
│   ├── go.mod
│   └── go.sum
│
├── frontend/                   # React TV application
│   ├── src/
│   │   ├── components/         # Reusable UI components
│   │   ├── screens/            # Screen-level components (TV pages)
│   │   ├── hooks/              # Custom React hooks
│   │   ├── services/           # API client layer
│   │   ├── context/            # React context providers
│   │   ├── navigation/         # Focus & navigation management
│   │   ├── utils/              # Pure helper functions
│   │   ├── types/              # TypeScript type definitions
│   │   └── assets/             # Images, fonts, etc.
│   ├── android/                # Android TV native shell (future)
│   └── package.json
│
├── CODING_RULES.md             # ← You are here
└── README.md
```

---

## 3. Backend Rules (Go + Gin)

### 3.1 Language & Framework

- **Go** (latest stable) with **Gin** as the HTTP framework.
- Use Go modules (`go.mod`) — no vendoring unless explicitly decided later.
- Target **Go 1.22+** for standard library improvements.

### 3.2 MVC Layers & Responsibilities

#### Model (`internal/model/`)
```
- Pure domain structs.
- Contains validation methods (e.g., `func (m *Movie) Validate() error`).
- ZERO imports from database, HTTP, or framework packages.
- Models represent business concepts, NOT database rows.
```

#### Repository (`internal/repository/`)
```
- Define one interface per aggregate root (e.g., `MovieRepository`).
- Interface lives in its own file: `internal/repository/movie_repository.go`.
- Concrete implementation lives in a sub-package: `internal/repository/sqlite/movie_repo.go`.
- Repositories accept and return model structs — never raw SQL rows in public API.
- Every repository method that can fail returns `(result, error)`.
```

**Example interface:**
```go
// internal/repository/movie_repository.go
package repository

import "streamingplayer/internal/model"

type MovieRepository interface {
    FindByID(id string) (*model.Movie, error)
    FindAll(limit, offset int) ([]model.Movie, error)
    Create(movie *model.Movie) error
    Update(movie *model.Movie) error
    Delete(id string) error
}
```

#### Service (`internal/service/`)
```
- Houses ALL business logic.
- Depends on repository INTERFACES, never concrete implementations.
- Receives dependencies via constructor injection.
- Methods should read like a business narrative.
- NO direct HTTP concepts (no gin.Context, no status codes).
```

**Example:**
```go
// internal/service/movie_service.go
package service

type MovieService struct {
    movies repository.MovieRepository
}

func NewMovieService(movies repository.MovieRepository) *MovieService {
    return &MovieService{movies: movies}
}

func (s *MovieService) GetMovieDetails(id string) (*model.Movie, error) {
    movie, err := s.movies.FindByID(id)
    if err != nil {
        return nil, fmt.Errorf("fetch movie %s: %w", id, err)
    }

    if movie == nil {
        return nil, ErrMovieNotFound
    }

    return movie, nil
}
```

#### Controller (`internal/controller/`)
```
- Thin HTTP handlers — their ONLY job:
    1. Parse & validate the request (extract params, bind JSON).
    2. Call the appropriate service method.
    3. Map the result to an HTTP response.
- One controller struct per domain area.
- Controllers register their own routes via a `RegisterRoutes(rg *gin.RouterGroup)` method.
- Never contain business logic. If an `if` checks a business rule, it belongs in the service.
```

#### DTO (`internal/dto/`)
```
- Separate structs for API requests and responses.
- Named clearly: `CreateMovieRequest`, `MovieResponse`.
- Contains JSON tags and binding/validation tags.
- Mapping between DTOs and Models is explicit (helper functions or methods).
- DTOs are the ONLY structs serialised to/from HTTP.
```

### 3.3 Dependency Injection

```
- Use constructor injection everywhere. No global variables for dependencies.
- Wire everything in `cmd/server/main.go`.
- The main function is the ONLY place that knows about concrete implementations.
```

**Wiring example (`main.go`):**
```go
func main() {
    cfg := config.Load()
    db  := sqlite.Connect(cfg.DatabasePath)

    // Repositories (concrete)
    movieRepo := sqlite.NewMovieRepository(db)

    // Services (depend on interfaces)
    movieSvc := service.NewMovieService(movieRepo)

    // Controllers
    movieCtrl := controller.NewMovieController(movieSvc)

    // Router
    router := gin.Default()
    api := router.Group("/api/v1")
    movieCtrl.RegisterRoutes(api)

    router.Run(cfg.ServerAddress)
}
```

### 3.4 Database Rules

```
- Use SQLite via `modernc.org/sqlite` (pure Go, no CGO) or `mattn/go-sqlite3`.
- All SQL lives inside the `repository/sqlite/` package — nowhere else.
- Use parameterised queries always. Never concatenate user input into SQL.
- Database schema changes go into numbered migration files under `migrations/`.
- The repository interface is DB-agnostic. Adding Postgres means adding
  `repository/postgres/` and implementing the same interfaces.
```

### 3.5 Error Handling

```go
// Define domain errors in the service layer.
// internal/service/errors.go
var (
    ErrMovieNotFound   = errors.New("movie not found")
    ErrInvalidInput    = errors.New("invalid input")
)
```

```
- Wrap errors with context using `fmt.Errorf("doing X: %w", err)`.
- Return errors up the stack — let the controller decide HTTP status.
- Controller maps domain errors to HTTP status codes in ONE place.
- Never panic for expected error conditions.
- Never silently swallow errors (no empty `if err != nil {}` blocks).
```

### 3.6 Logging

> **Guiding rule:** A reader should be able to understand the business logic
> *without* reading a single log line.

```
- Use `log/slog` (Go 1.21+ structured logger) — no third-party logging libraries.
- Log at function boundaries (entry/exit of controller handlers, repository calls)
  using middleware — NOT inline with business logic.
- Business logic in services should have ZERO log lines on the happy path.
  Errors may be logged, but prefer returning them.
- Use these levels:
    DEBUG  → detailed tracing (SQL queries, cache hits)
    INFO   → significant lifecycle events (server started, migration applied)
    WARN   → recoverable issues (retry, fallback used)
    ERROR  → failures that need attention (DB connection lost)
- HTTP request/response logging happens in middleware, once, not in every handler.
- Log format: JSON in production, text in development. Controlled by config.
```

**Bad — log lines obscure the logic:**
```go
func (s *MovieService) GetMovieDetails(id string) (*model.Movie, error) {
    slog.Info("GetMovieDetails called", "id", id)                    // ❌
    movie, err := s.movies.FindByID(id)
    slog.Debug("FindByID returned", "movie", movie, "err", err)      // ❌
    if err != nil {
        slog.Error("failed to find movie", "id", id, "err", err)     // ❌
        return nil, fmt.Errorf("fetch movie %s: %w", id, err)
    }
    slog.Info("GetMovieDetails success", "id", id)                   // ❌
    return movie, nil
}
```

**Good — clean logic, logging handled elsewhere:**
```go
func (s *MovieService) GetMovieDetails(id string) (*model.Movie, error) {
    movie, err := s.movies.FindByID(id)
    if err != nil {
        return nil, fmt.Errorf("fetch movie %s: %w", id, err)
    }

    if movie == nil {
        return nil, ErrMovieNotFound
    }

    return movie, nil
}
```

### 3.7 API Conventions

```
- RESTful routes: `/api/v1/movies`, `/api/v1/movies/:id`
- Use versioned API groups (`/api/v1/...`).
- Standard JSON response envelope:
    Success: { "data": <payload> }
    Error:   { "error": { "code": "<MACHINE_CODE>", "message": "<human message>" } }
- Use plural nouns for resources (`/movies` not `/movie`).
- HTTP methods: GET (read), POST (create), PUT (full update), PATCH (partial), DELETE.
- Return proper status codes: 200, 201, 204, 400, 404, 500.
```

### 3.8 Naming Conventions (Go)

```
- Package names: lowercase, single word (`model`, `service`, `sqlite`).
- Interfaces: noun or verb-noun (MovieRepository, not IMovieRepository).
- Constructors: NewXxx (NewMovieService).
- Files: snake_case (movie_service.go, not movieService.go).
- Variables: camelCase, descriptive. `movieRepo` not `mr`.
- Acronyms: keep casing consistent (ID not Id, URL not Url, HTTP not Http).
- Test files: xxx_test.go alongside the source file.
```

### 3.9 Testing (Backend)

```
- Table-driven tests for service logic.
- Mock repositories using interfaces — no test DB needed for unit tests.
- Integration tests can use an in-memory SQLite database.
- Test file lives next to the source: `movie_service_test.go`.
- Test function naming: TestMovieService_GetMovieDetails_ReturnsNotFound.
- Use testify/assert for cleaner assertions (optional, standard library is fine too).
```

---

## 4. Frontend Rules (React TV)

### 4.1 Framework & Platform

```
- React Native with a TV-focused setup.
- TypeScript everywhere — no plain JavaScript files.
- Target: Android TV (primary future target), with architecture
  that supports Fire TV and potentially Apple TV.
```

### 4.2 Architecture

```
- Screens/: Top-level screens mapped to navigation routes.
- Components/: Reusable, focusable UI components.
- Hooks/: Custom hooks encapsulate data-fetching, focus logic, etc.
- Services/: API client modules (one per backend resource).
- Context/: React context for global state (auth, player, theme).
- Types/: Shared TypeScript interfaces and type aliases.
```

### 4.3 Component Rules

```
- Functional components only. No class components.
- One component per file. File name matches component name (PascalCase).
- Props defined as a TypeScript interface: `interface MovieCardProps { ... }`.
- Keep components small (< 150 lines). Extract sub-components when it grows.
- Separate presentational components from container/screen components.
- All interactive elements must support D-pad / remote navigation (focus management).
```

### 4.4 State Management

```
- Start with React Context + useReducer for global state.
- Local state via useState for component-specific UI state.
- Data fetching via custom hooks (e.g., useMovies, useMovieDetails).
- No Redux unless complexity explicitly demands it later.
```

### 4.5 API Client Layer

```
- Centralised API client in services/api.ts with base URL config.
- Each resource gets its own service file (services/movieService.ts).
- Service functions return typed responses.
- Handle loading, error, and empty states in every screen.
```

### 4.6 Styling

```
- Use React Native StyleSheet.create() — no inline styles.
- Define a theme file (theme.ts) with colours, spacing, typography tokens.
- All components reference theme tokens — no magic numbers or hex codes inline.
- Design for 1080p (Full HD) TV screens. Scale-aware spacing.
- Large, legible text sizes. Minimum 18sp for body text on TV.
```

### 4.7 Navigation & Focus

```
- Use a navigation library compatible with TV (React Navigation with TV extensions).
- Every interactive element must be focusable and visually indicate focus.
- Focus order should be logical and predictable (left-right, top-bottom).
- Handle back button / remote menu consistently across screens.
```

### 4.8 Naming Conventions (Frontend)

```
- Components & Screens: PascalCase (MovieCard.tsx, HomeScreen.tsx).
- Hooks: camelCase, prefixed with "use" (useMovies.ts).
- Services: camelCase (movieService.ts).
- Types/Interfaces: PascalCase (Movie, MovieListResponse).
- Constants: UPPER_SNAKE_CASE (API_BASE_URL).
- Props interfaces: ComponentNameProps (MovieCardProps).
```

### 4.9 Android Compatibility

```
- Avoid web-only APIs. Use React Native primitives.
- Test focus behaviour on Android TV emulator.
- Keep platform-specific code in .android.tsx / .tv.tsx files when needed.
- Use react-native-video or equivalent for media playback (cross-platform).
```

---

## 5. Cross-Cutting Rules

### 5.1 Git Conventions

```
- Conventional commits: feat:, fix:, refactor:, docs:, test:, chore:
- Branch naming: feature/xxx, fix/xxx, refactor/xxx
- Keep commits atomic — one logical change per commit.
```

### 5.2 Configuration

```
- All environment-specific values come from config, never hardcoded.
- Backend: load from environment variables or a .env file via config package.
- Frontend: use .env files with REACT_APP_ / EXPO_PUBLIC_ prefix.
- Secrets never committed. Add .env to .gitignore.
```

### 5.3 Code Readability Checklist

Before considering any piece of code done, verify:

- [ ] Can a new developer understand the business intent in under 60 seconds?
- [ ] Are variable and function names self-documenting?
- [ ] Is the happy path clearly visible without scrolling past error handling?
- [ ] Are there zero log lines cluttering the business logic flow?
- [ ] Are magic numbers and strings replaced with named constants?
- [ ] Is the function under 40 lines? If not, can it be split?

### 5.4 Documentation

```
- Every exported Go function/type has a doc comment.
- Every React component has a brief JSDoc comment describing its purpose.
- README.md has setup instructions, prerequisites, and how to run.
- API endpoints are documented (initially in README, later in OpenAPI/Swagger).
```

---

## 6. What We Explicitly Avoid

| Avoid | Why |
|---|---|
| Global mutable state | Makes testing hard, hides dependencies |
| `init()` functions | Implicit execution order, hard to trace |
| Putting business logic in controllers | Violates MVC, makes logic untestable without HTTP |
| Logging inside service happy paths | Clutters readability — use middleware & error wrapping |
| Hardcoded DB queries outside repository | Breaks the abstraction boundary |
| `interface{}` / `any` without reason | Defeats type safety |
| Premature optimisation | Readability > speed at this stage |
| Giant files (> 300 lines) | Split by responsibility |

---

*These rules are a living document. Update them as the project evolves.*
