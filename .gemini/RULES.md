# StreamingPlayer — AI Coding Rules

Always read and follow `/CODING_RULES.md` before writing any code.

## Quick Reference

### Backend (Go + Gin)
- MVC with interfaces: Model → Repository (interface) → Service → Controller → DTO
- Business logic lives ONLY in `internal/service/`
- Controllers are THIN: parse request → call service → send response
- Repository interfaces in `internal/repository/`, SQLite implementations in `internal/repository/sqlite/`
- Models in `internal/model/` have ZERO database or HTTP imports
- Constructor injection for all dependencies. Wire in `cmd/server/main.go`
- Use `log/slog`. NO log lines inside service happy paths
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- Use `modernc.org/sqlite` or `mattn/go-sqlite3`
- All SQL lives inside `repository/sqlite/` only

### Frontend (React TV + TypeScript)
- TypeScript everywhere. No `.js` files
- Functional components only, one per file
- StyleSheet.create() with theme tokens — no inline styles
- Custom hooks for data fetching (useMovies, etc.)
- All interactive elements must support D-pad / TV remote focus
- Design for 1080p TV. Min 18sp body text
- Keep Android TV compatibility in mind — avoid web-only APIs

### Code Quality
- Readability over micro-optimisation
- Functions under 40 lines
- Files under 300 lines
- Self-documenting names. No abbreviations
- No global mutable state. No init() functions
- No business logic in controllers
- No hardcoded values — use config / constants
