Snapshots of the Go rules: each test in `internal/game` that reads a file here recomputes its outputs from the
recorded inputs and compares. After a deliberate rule change, re-record with

    cd server && go test ./internal/game/ -update

and review the diff. `simbands.json` is a band, not an exact snapshot: `TestBotWorldStaysInBands` allows ±25 %.
