-- +goose Up
-- A short world's history for its replay: entries while it runs, one packed blob after (kept with the archive).
create table timeline (
  world_id bigint not null references worlds on delete cascade,
  seq bigint not null,
  at bigint not null,
  entry jsonb not null,
  primary key (world_id, seq)
);
create table timeline_packed (
  world_id bigint primary key,
  entries integer not null,
  data bytea not null,
  packed_at_ms bigint not null
);

-- +goose Down
drop table timeline_packed;
drop table timeline;
