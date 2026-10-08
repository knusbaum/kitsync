go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

Running ksd requires the PostgreSQL connection settings in the environment:

```sh
export PGHOST=localhost
export PGUSER=kitsync
read -rsp 'Database password: ' PGPASSWORD; echo
export PGPASSWORD
go run ./cmd/ksd
```

All three variables (`PGHOST`, `PGUSER`, and `PGPASSWORD`) must be nonempty.
ksd connects to database `kitsync` on port `5432`.
