# Chines backend

Go + chi + pgx API for a Chinese-language school. Students practise each day's
vocabulary (recognition, typing, spaced repetition) and teachers manage content
and monitor progress.

## Requirements

- Go 1.22+
- PostgreSQL 16 (or Docker)

## Run locally

```bash
docker compose up -d db
cp .env.example .env        # if provided, otherwise create .env
go run ./cmd/api            # listens on :8084
go run ./cmd/seed           # idempotent HSK 1 seed
```

Required env vars: `DATABASE_URL`, `JWT_SECRET`. Optional: `PORT` (8084),
`UPLOAD_DIR` (uploads), `SEED_TEACHER_EMAIL`, `SEED_TEACHER_PASSWORD`,
`SEED_TEACHER_NAME`, `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_PASSWORD`,
`BOOTSTRAP_ADMIN_NAME` (default `Admin`).

### First admin

Self-registration always creates a student, so the first admin must be
bootstrapped. Set `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` (8–72
characters) before starting the server; startup creates that admin or updates an
existing account to admin with the supplied password, re-enables it, and revokes
its old sessions. On Render, set both as service environment variables (the
blueprint declares them as `sync: false`) and redeploy. **Remove the password
variable after logging in if you do not want it reapplied on every deploy.**

For local development you can use the seed teacher account
(`teacher@chines.local` / `teach12345`) as the bootstrap admin by setting the
two variables to those values.

Pronunciation scoring: `SCORER=fake` (default) or `SCORER=azure`. Azure also
needs `AZURE_SPEECH_KEY` and `AZURE_SPEECH_REGION`. The fake scorer is
deterministic and needs no credentials, so the record → score flow works
locally. Azure accepts 16 kHz mono PCM WAV, which is what the mobile app records;
other formats are rejected with a clear error. Recordings are stored under
`UPLOAD_DIR/private` and are only reachable through the authenticated endpoint.

Migrations run automatically at startup (single-instance safe: each migration is
recorded in `schema_migrations` inside one transaction).

## Content

`content/hsk1.json` is the single source of truth for the HSK 1 catalog
(6 lessons / 42 words). `cmd/seed` imports it into a class named **HSK 1** owned
by the seed teacher. Running it again updates words in place.

## Auth

Access token: JWT, 15 min, role inside. Refresh token: rotating, single-use,
stored only as a hash. Roles: `student`, `teacher`, `admin`. Self-registration is
always `student`. Disabled users cannot log in or refresh, and disabling or
changing a role revokes all refresh tokens.

Unauthorized access to a resource you cannot see returns **404**, not 403.

## Endpoints

### Public
| Method | Path | Notes |
| --- | --- | --- |
| GET | `/health` | DB health |
| POST | `/v1/auth/register` | student self-registration |
| POST | `/v1/auth/login` | |
| POST | `/v1/auth/refresh` | rotates the refresh token |
| POST | `/v1/auth/logout` | revokes one refresh token |
| GET | `/media/audio/{name}` | public word audio |

### Authenticated
| Method | Path | Role | Notes |
| --- | --- | --- | --- |
| GET | `/v1/me` | any | |
| GET | `/v1/classes` | any | own classes |
| POST | `/v1/classes` | teacher/admin | admin may pass `teacher_id` to assign it |
| PATCH | `/v1/classes/{id}` | teacher/admin | `name` and/or `teacher_id` (admin only) |
| DELETE | `/v1/classes/{id}` | teacher/admin | cascades lessons/words/audio |
| GET | `/v1/classes/{id}/members` | teacher/admin | |
| POST | `/v1/classes/{id}/members` | teacher/admin | add by email |
| DELETE | `/v1/classes/{id}/members/{studentId}` | teacher/admin | |
| GET | `/v1/classes/{id}/lessons` | any with access | |
| POST | `/v1/classes/{id}/lessons` | teacher/admin | |
| GET | `/v1/lessons?date=YYYY-MM-DD` | any | local date |
| GET | `/v1/lessons/{id}` | any with access | includes words |
| PATCH | `/v1/lessons/{id}` | teacher/admin | title + date |
| DELETE | `/v1/lessons/{id}` | teacher/admin | cascades words/audio |
| POST | `/v1/lessons/{id}/words` | teacher/admin | |
| POST | `/v1/lessons/{id}/words/reorder` | teacher/admin | body `{"word_ids":[...]}` |
| PATCH | `/v1/words/{id}` | teacher/admin | |
| DELETE | `/v1/words/{id}` | teacher/admin | deletes audio |
| PUT | `/v1/words/{id}/audio` | teacher/admin | multipart `audio`, max 2 MB |
| DELETE | `/v1/words/{id}/audio` | teacher/admin | |
| GET | `/practice/due` | student | `?date=YYYY-MM-DD&limit=` |
| GET | `/practice/summary` | student | `?date=YYYY-MM-DD` |
| POST | `/practice/answer` | student | |
| POST | `/v1/pronunciation/{wordId}` | student | multipart `audio`, max 2 MB, 12/min |
| GET | `/v1/pronunciation/history?wordId=` | student | newest first, max 50 |
| GET | `/v1/pronunciation/{id}/audio` | owner, class teacher, admin | private stream |

### Teacher dashboard (teacher: own classes; admin: all)
| Method | Path | Notes |
| --- | --- | --- |
| GET | `/v1/teacher/overview` | class/student counts + active last 7 days |
| GET | `/v1/teacher/classes/{id}/progress` | per-student aggregates |
| GET | `/v1/teacher/classes/{id}/students/{studentId}` | lessons, hardest words, recent attempts |

`average_pronunciation_score` is the mean `overall_score` of the student's
pronunciation attempts in that class, or `null` when they have none.

### Pronunciation
| Method | Path | Notes |
| --- | --- | --- |
| POST | `/v1/pronunciation/{wordId}` | `multipart/form-data` field `audio`; magic-byte validation; rate limited per student |
| GET | `/v1/pronunciation/history?wordId=` | the student's own attempts |
| GET | `/v1/pronunciation/{id}/audio` | owner, the class teacher, or an admin |

Provider failures return **502**, a 2 MB overflow **413**, and more than 12
attempts in a minute **429**. A failed provider call deletes the uploaded file
and records nothing, so retrying is safe.

### Admin
| Method | Path | Notes |
| --- | --- | --- |
| GET | `/v1/admin/overview` | class, student, teacher, admin, and active-student counts |
| GET | `/v1/admin/users?role=&q=&page=&page_size=` | paginated |
| POST | `/v1/admin/users` | create teacher; returns `temporary_password` |
| PATCH | `/v1/admin/users/{id}/role` | cannot demote self or last admin |
| POST | `/v1/admin/users/{id}/disable` | revokes refresh tokens |
| POST | `/v1/admin/users/{id}/enable` | |

## Tests

```bash
go build ./...
go vet ./...
go test ./...
```
