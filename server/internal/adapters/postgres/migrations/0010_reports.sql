-- +goose Up
-- Messages to the admin from players and visitors. world_id has no foreign key: worlds are purged, reports stay.
create table problem_reports (
  id bigserial primary key,
  created_at timestamptz not null default now(),
  kind text not null check (kind in ('bug', 'player', 'question', 'other')),
  text text not null,
  email text not null default '',
  account_id bigint references accounts on delete set null,
  world_id bigint,
  world_name text not null default '',
  screen text not null default '',
  build text not null default '',
  user_agent text not null default '',
  status text not null default 'new' check (status in ('new', 'read', 'done')),
  closed_at timestamptz
);
create index problem_reports_created_at on problem_reports (created_at desc);

-- +goose Down
drop table problem_reports;
