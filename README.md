# File Explorer

A full-stack file manager with folder hierarchies, drag-and-drop uploads, fuzzy search, and a trash/restore flow.
---

## Stack


| Layer        | Technology                                                                    |
| ------------ | ----------------------------------------------------------------------------- |
| Backend      | Go 1.25, `net/http` + chi, pgx/v5                                             |
| Database     | PostgreSQL 18                                                                 |
| Blob storage | Local disk (content-addressable), Railway persistent volume on hosted version |
| Frontend     | React 19, TypeScript, Vite                                                    |
| Test runner  | `go test -race -p 1` against a real Postgres instance                         |

Initially, I was choosing between Go and Python, but settled on Go because it's file handling is much simpler, especially for larger files when you need to stream them. 
The rest of the stack is pretty self explanatory, React/Postgres are obvious choices. 
In terms of storing the files, they are stored on disk currently, but they _are_ stored in a persistent volume inside of the Railway instance.

## Architecture
### Node model — adjacency list

```
nodes table
┌────┬───────────┬──────────────┬────────┬──────────────┬────────────┐
│ id │ parent_id │ name         │ type   │ content_hash │ deleted_at │
├────┼───────────┼──────────────┼────────┼──────────────┼────────────┤
│  1 │   NULL    │ Documents    │ folder │      —       │    NULL    │
│  2 │   NULL    │ Photos       │ folder │      —       │    NULL    │
│  3 │     1     │ Report.pdf   │ file   │   ab3d…      │    NULL    │
│  4 │     1     │ Notes.txt    │ file   │   9f2c…      │    NULL    │
│  5 │     2     │ Vacation.jpg │ file   │   e3b0…      │    NULL    │
└────┴───────────┴──────────────┴────────┴──────────────┴────────────┘

Represents:
/
├── Documents/        (id=1, parent=NULL)
│   ├── Report.pdf    (id=3, parent=1)
│   └── Notes.txt     (id=4, parent=1)
└── Photos/           (id=2, parent=NULL)
    └── Vacation.jpg  (id=5, parent=2)
```


In an Adjacency list, each row has a `parent_id` which points to the parent folder; This way, we can look up each folders children with a single lookup by `parent_id`. Additionally, moving a node to a new parent (moving folders) is a single field update. 

An Adjacency list was chosen because given the alternatives, it's performance was the best for this situation (move cost + breadcrumb cost + rename cost etc...).

### Content-addressable blob store
Files are stored on disk with the name of the SHA-256 hash of their contents. 


**Upload pipeline:**

```
HTTP multipart request
        │
        ▼
  data/tmp/<random>           ← write starts here
        │
        │  (SHA-256 computed in the same pass as writing, via TeeReader)
        │
        ▼
  os.Rename ──────────────►  data/blobs/ab/cd/abcd1234ef56...
  (atomic on POSIX)           └─ first 2 chars ─┘└─ next 2 ─┘└─ full hash ─┘
```

The two level directory sharding allows us to keep directory counts low, to support efficiency (millions of folders would degrade performance heavily).

This approach was also chosen because it allows us to avoid duplicate files: if two files with different names are uploaded, they will have the same hash. Then, if a blob with that hash already exists, the new database row will just point to the existing blob. 

### Soft delete and trash
The delete route `DELETE /nodes{id}` doesn't remove the row or delete the file. Instead, it just sets the `deleted_at` field on that node, and every descendant of that node to the current time. 

Soft-deleted nodes are hidden from the file explorere, but are shown in the trash bin `GET /trash`; 

In the trash bin, there is an option to permanently delete all files in the trash bin which will then remove nodes from the DB and will also remove files from the persistent volume. 

### Cycle Detection for moves
Moving a folder into one of its descendants will create a cycle. 
```
Before (valid tree):
  A (id=1)
  └── B (id=2)
      └── C (id=3)

Illegal move: make A a child of C

After (corrupted):
  C (id=3)
  └── A (id=1)   ← A's parent is now C
      └── B (id=2)
          └── C (id=3)  ← C's parent is still B → infinite loop
```

We have explicit checks for this, to make sure that a user is not moving a folder into a descendent of it's, as this would permanently corrupt the folder and the user would not be able to see their files. 


### HTTP server timeouts

The server sets a header timeout but intentionally omits a full request-body timeout. A body timeout would kill large uploads mid-stream (a 23 GB file upload could legitimately take minutes).


## Testing

Tests run against a real PostgreSQL instance — no mocks. Each test creates a connection pool, runs migrations (idempotent), and truncates the nodes table on cleanup. The `-p 1` flag is required because both test packages connect to the same database; without it, Go runs packages concurrently and their cleanup calls clobber each other's data mid-test.

The `-race` flag is always on. The concurrent stress tests deliberately exercise shared state under load; the race detector catches any unsynchronized access.

```bash
TEST_DATABASE_URL="postgres://user:password@localhost:5432/file-explorer" \
  go test -p 1 -race ./internal/... -v -count=1 -timeout 180s
```

### Test coverage highlights


| Test                                          | What it proves                                                                         |
| --------------------------------------------- | -------------------------------------------------------------------------------------- |
| `TestSafeMoveNode_ConcurrentRace`             | `SELECT FOR UPDATE` prevents cycles under concurrent moves                             |
| `TestStress_ConcurrentDedupUpload`            | 50 concurrent uploads of identical content → all 201, dedup is race-safe               |
| `TestStress_ConcurrentUniqueNameRace`         | 30 concurrent folder creates with the same name → exactly 1 wins, 29 get 409           |
| `TestStress_ConcurrentUploadDifferentFiles`   | 30 distinct files uploaded in parallel → all 201, all retrievable with correct content |
| `TestHardDeleteSubtree_FolderWithNestedFiles` | Regression: permanent delete of a folder with children no longer triggers FK violation |
| `TestPermanentDelete_FolderWithFiles`         | End-to-end regression for the same bug at the HTTP handler level                       |



## Running locally

```bash
# Start Postgres
docker compose up -d

# Start backend (auto-migrates on first run)
cd backend && go run ./cmd/server

# Start frontend dev server
cd frontend && bun install && bun dev
```

The backend reads from environment variables with sensible defaults:


| Variable       | Default                                                 |
| -------------- | ------------------------------------------------------- |
| `PORT`         | `8080`                                                  |
| `DATABASE_URL` | `postgres://user:password@localhost:5432/file-explorer` |
| `DATA_DIR`     | `./data`                                                |
| `FRONTEND_URL` | `http://localhost:5173`                                 |


## AI Usage
AI was used heavily throughout this project. For architecting the project, Fable (rip)/Opus was used, and for implementation, Sonnet was used. Claude Code was used through most, with Cursor here and there. 

In the architecting phase, lots of back and forth was had with the models + doing my own researching on the types of problems and approaches you could take - the most research was done on which hierarchy model would be best, but also researched how streaming files in Go works / why one should use Go in a case like this over Python.

Within the implementation phase, Sonnet handled most of the code generation; I built out the initial wireframe of the app (Frontend with bun & Backend in Go) by hand, and then handed it off to Claude to begin the implementation on the project. 
