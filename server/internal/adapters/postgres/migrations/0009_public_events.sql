-- +goose Up
-- Battles and conquests anyone may see; they go with their world.
create table public_events (
  world_id bigint not null references worlds on delete cascade,
  seq bigserial,
  at timestamptz not null,
  kind text not null check (kind in ('conquest', 'battle')),
  attacker text not null,
  defender text not null,
  attacker_bot boolean not null,
  defender_bot boolean not null,
  troops integer not null,
  x integer not null,
  y integer not null,
  primary key (world_id, seq)
);
create index public_events_at on public_events (at desc);
create index public_events_world_at on public_events (world_id, at desc);

-- +goose Down
drop table public_events;
