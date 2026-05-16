# Multiple Groups Example

This example demonstrates Sablier's **multiple groups per instance** feature. An instance can belong to more than one group by using a comma-separated `sablier.group` label.

## Services

| Service | Group(s) | Description |
|---------|----------|-------------|
| `frontend` | `team-a` | Exclusive to team-a |
| `backend` | `team-b` | Exclusive to team-b |
| `shared-api` | `team-a,team-b` | Central service shared by both teams |

When a session is requested for `team-a`, Sablier starts `frontend` **and** `shared-api`.  
When a session is requested for `team-b`, Sablier starts `backend` **and** `shared-api`.

## Label Format

```yaml
# Single group
- "sablier.group=team-a"

# Multiple groups — comma-separated
- "sablier.group=team-a,team-b"
```

## Running the Example

```bash
# Start Sablier with all managed services stopped
make up

# Trigger a team-a session (frontend + shared-api start)
make demo-team-a

# Trigger a team-b session (backend + shared-api start)
make demo-team-b

# Tear down
make down
```

## Manual Steps

```bash
docker compose up -d
docker compose stop frontend backend shared-api

# Request team-a — blocks until frontend and shared-api are ready
curl 'http://localhost:10000/api/strategies/blocking?group=team-a&timeout=30s'

# Request team-b — blocks until backend and shared-api are ready
curl 'http://localhost:10000/api/strategies/blocking?group=team-b&timeout=30s'
```

## Notes

- `shared-api` is started independently for each group session; Sablier is idempotent — if it is already running it is counted as ready immediately.
- Sessions expire after 30 seconds of inactivity. When both group sessions have expired, all three services are stopped.
- Spaces around commas in the label value are trimmed. `"team-a , team-b"` is equivalent to `"team-a,team-b"`.
