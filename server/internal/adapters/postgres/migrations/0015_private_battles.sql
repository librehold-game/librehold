-- +goose Up
-- Battles leave the public feed, which keeps only conquests; a world remembers only when its last battle was.
alter table worlds add column last_battle_at timestamptz;
update worlds w set last_battle_at = (select max(at) from public_events e where e.world_id = w.id);
delete from public_events where kind = 'battle';
alter table public_events drop column troops;
alter table public_events drop constraint public_events_kind_check;
alter table public_events add constraint public_events_kind_check check (kind = 'conquest');

-- +goose Down
alter table public_events drop constraint public_events_kind_check;
alter table public_events add constraint public_events_kind_check check (kind in ('conquest', 'battle'));
alter table public_events add column troops integer not null default 0;
alter table worlds drop column last_battle_at;
