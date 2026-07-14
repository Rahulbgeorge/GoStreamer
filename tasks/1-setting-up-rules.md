# Task 1 — Setting Up Coding Rules

**Status:** ✅ Completed  
**Date:** 2026-06-13  

---

## Prompt

> Lets first write our coding rules for the backend
>
> Ensure that all the code is written in go and gin for the backend.
> Try to focus on Best practices and organize the code as per mvc, where the code is loosely coupled.
>
> Use a SQLITE db right now, let it be flexible.
>
> Frontend should be coded using React TV and also focus on maintaining future support for android app also.
>
> Write good rules.
>
> All the code should focus on being more readable over focusing on hyper efficiency in the beginning, when i say readable the business logic should be easier to read.
>
> The logs should not overpower over the actual business logic, hence try to keep it neat and readable.

---

## Plan

1. Create a comprehensive `CODING_RULES.md` at the project root covering:
   - Backend architecture (Go + Gin, MVC, loose coupling)
   - Database abstraction (SQLite behind repository interfaces)
   - Frontend conventions (React TV + TypeScript, Android TV ready)
   - Readability-first philosophy
   - Logging discipline (no logs in service happy paths)
2. Create `.gemini/RULES.md` as a quick-reference for AI-assisted coding sessions.

---

## What Was Done

### Files Created

| File | Purpose |
|---|---|
| `CODING_RULES.md` | Full coding rules & conventions document |
| `.gemini/RULES.md` | AI quick-reference rules summary |

### Key Decisions Made

| Decision | Rationale |
|---|---|
| **MVC with repository interfaces** | Keeps layers decoupled — swapping SQLite for Postgres means adding a new sub-package, not rewriting services |
| **Constructor injection, wired in `main.go`** | No global state, explicit dependency graph, easy to test |
| **`log/slog` only, no logs in service happy paths** | Business logic reads cleanly; logging handled at boundaries via middleware |
| **React Native with TypeScript for frontend** | Covers TV platforms (Android TV, Fire TV) from a single codebase |
| **Readability over micro-optimisation** | Functions < 40 lines, files < 300 lines, self-documenting names |
| **DTOs separate from Models** | API layer stays decoupled from domain; models never leak HTTP concerns |

---

## Project Structure Established

```
StreamingPlayer/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── model/
│   │   ├── repository/
│   │   │   └── sqlite/
│   │   ├── service/
│   │   ├── controller/
│   │   ├── middleware/
│   │   ├── dto/
│   │   └── config/
│   ├── pkg/
│   └── migrations/
├── frontend/
│   └── src/
│       ├── components/
│       ├── screens/
│       ├── hooks/
│       ├── services/
│       ├── context/
│       ├── navigation/
│       ├── utils/
│       ├── types/
│       └── assets/
├── tasks/                  ← Task tracking (this folder)
├── CODING_RULES.md
└── .gemini/RULES.md
```
