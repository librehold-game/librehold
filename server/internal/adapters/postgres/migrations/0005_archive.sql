-- +goose Up
-- Ended instant worlds, kept after their world rows are deleted (no foreign key to worlds on purpose).
create table instant_archive (
  world_id bigint primary key,
  name text not null,
  started_at_ms bigint not null,
  ended_at_ms bigint not null,
  winner_name text not null,
  winner_is_bot boolean not null,
  summary jsonb not null
);
create index instant_archive_ended on instant_archive (ended_at_ms);

create table instant_leaderboard (
  world_id bigint not null references instant_archive on delete cascade,
  rank integer not null,
  player_id text not null,
  name text not null,
  is_bot boolean not null,
  account_id bigint references accounts on delete set null,
  points integer not null,
  villages integer not null,
  conquered integer not null,
  killed bigint not null,
  primary key (world_id, rank)
);
create index instant_leaderboard_account on instant_leaderboard (account_id) where account_id is not null;

-- +goose Down
drop table instant_leaderboard;
drop table instant_archive;
