-- +goose Up
-- Every world's action log while it runs; no foreign key to worlds, the log is packed before a world is deleted.
create table action_log (
  world_id bigint not null,
  seq bigint not null,
  at bigint not null,
  actor text not null,
  is_bot boolean not null,
  kind text not null,
  payload jsonb not null,
  primary key (world_id, seq)
);

create table action_rejections (
  world_id bigint not null,
  kind text not null,
  reason text not null,
  count integer not null,
  primary key (world_id, kind, reason)
);

create table action_log_packed (
  world_id bigint primary key,
  seed bigint not null,
  config jsonb not null,
  build text not null,
  rows integer not null,
  last_seq bigint not null,
  data bytea not null,
  packed_at_ms bigint not null
);

-- +goose Down
drop table action_log_packed;
drop table action_rejections;
drop table action_log;
