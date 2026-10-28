-- +goose Up
create table accounts (
  id bigserial primary key,
  username text not null,
  username_lower text not null unique,
  password_hash text not null,
  is_admin boolean not null default false,
  created_at timestamptz not null default now()
);

create table sessions (
  token_hash bytea primary key,
  account_id bigint not null references accounts on delete cascade,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  last_seen_at timestamptz not null default now()
);
create index sessions_account on sessions (account_id);

create table access_codes (
  id bigserial primary key,
  code text not null unique,
  note text not null default '',
  created_at timestamptz not null default now(),
  used_by bigint references accounts,
  used_at timestamptz,
  revoked_at timestamptz
);

create table worlds (
  id bigserial primary key,
  name text not null,
  status text not null check (status in ('open', 'closed')),
  seed bigint not null,
  config jsonb not null,
  started_at_ms bigint not null,
  paused_at_ms bigint,
  paused_total_ms bigint not null default 0,
  next_id bigint not null default 0,
  event_seq bigint not null default 0,
  created_at timestamptz not null default now()
);

create table players (
  world_id bigint not null references worlds on delete cascade,
  id text not null,
  account_id bigint references accounts,
  name text not null,
  is_human boolean not null,
  status text not null,
  points integer not null default 0,
  data jsonb not null,
  primary key (world_id, id),
  unique (world_id, account_id)
);
create index players_account on players (account_id);

create table bot_brains (
  world_id bigint not null references worlds on delete cascade,
  player_id text not null,
  data jsonb not null,
  primary key (world_id, player_id)
);

create table human_states (
  world_id bigint not null references worlds on delete cascade,
  player_id text not null,
  data jsonb not null,
  primary key (world_id, player_id)
);

create table villages (
  world_id bigint not null references worlds on delete cascade,
  id text not null,
  x integer not null,
  y integer not null,
  name text not null,
  owner_id text,
  points integer not null,
  data jsonb not null,
  primary key (world_id, id),
  unique (world_id, x, y)
);
create index villages_owner on villages (world_id, owner_id);

create table village_support (
  world_id bigint not null references worlds on delete cascade,
  host_id text not null,
  home_id text not null,
  primary key (world_id, host_id, home_id)
);
create index village_support_home on village_support (world_id, home_id);

create table commands (
  world_id bigint not null references worlds on delete cascade,
  id text not null,
  type text not null,
  origin_id text not null,
  target_id text not null,
  home_id text not null,
  arrives_at bigint not null,
  data jsonb not null,
  primary key (world_id, id)
);
create index commands_target on commands (world_id, target_id);
create index commands_home on commands (world_id, home_id);

create table events (
  world_id bigint not null references worlds on delete cascade,
  key text not null,
  due_at bigint not null,
  seq bigint not null,
  primary key (world_id, key)
);
create index events_due on events (world_id, due_at, seq);

create table reports (
  world_id bigint not null references worlds on delete cascade,
  recipient_id text not null,
  id text not null,
  time bigint not null,
  read boolean not null,
  data jsonb not null,
  primary key (world_id, recipient_id, id)
);
create index reports_recent on reports (world_id, recipient_id, time desc);

create table intel (
  world_id bigint not null references worlds on delete cascade,
  player_id text not null,
  village_id text not null,
  data jsonb not null,
  primary key (world_id, player_id, village_id)
);

create table scheduled_sends (
  world_id bigint not null references worlds on delete cascade,
  id text not null,
  owner_id text not null,
  send_at bigint not null,
  data jsonb not null,
  primary key (world_id, id)
);
create index scheduled_sends_owner on scheduled_sends (world_id, owner_id);

create table news (
  world_id bigint not null references worlds on delete cascade,
  id text not null,
  time bigint not null,
  data jsonb not null,
  primary key (world_id, id)
);
create index news_recent on news (world_id, time desc);

create table conquests (
  world_id bigint not null references worlds on delete cascade,
  seq bigserial,
  t bigint not null,
  data jsonb not null,
  primary key (world_id, seq)
);

-- +goose Down
drop table conquests, news, scheduled_sends, intel, reports, events, commands, village_support, villages,
  human_states, bot_brains, players, worlds, access_codes, sessions, accounts;
