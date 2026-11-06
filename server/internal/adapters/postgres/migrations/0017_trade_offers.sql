-- +goose Up
-- Trade offers: player to player resource deals posted on the public board. The whole offer is one row of data;
-- the expiry runs as a world event, so nothing here is time-indexed.
create table trade_offers (
  world_id bigint not null references worlds on delete cascade,
  id text not null,
  from_id text not null,
  expires_at bigint not null,
  data jsonb not null,
  primary key (world_id, id)
);
create index trade_offers_from on trade_offers (world_id, from_id);

-- +goose Down
-- Hand back whatever the open offers were holding. Posting reserves the give side out of the origin village straight
-- away, so without this a rollback would destroy those resources. The refund is capped at the storehouse, like every
-- runtime refund (AddResourcesCapped), and anything above it is lost rather than left sitting out of the economy.
--
-- The capacity mirrors game.StorageCapacity: level 0 stores 500, otherwise round(1000 * 1.2294^(level-1)), times 1.5
-- for a village with the storage bonus. Those three pairs are RESOURCE_BALANCE.storageLevel0 / storageBase /
-- storageFactor and BONUS_BALANCE.storageCapacity; a test pins this copy to the Go function.
update villages v set data = jsonb_set(v.data, '{resources}', (
    select jsonb_build_object(
      'wood', greatest(0, least(c.cap, coalesce((v.data->'resources'->>'wood')::float8, 0) + g.wood)),
      'clay', greatest(0, least(c.cap, coalesce((v.data->'resources'->>'clay')::float8, 0) + g.clay)),
      'iron', greatest(0, least(c.cap, coalesce((v.data->'resources'->>'iron')::float8, 0) + g.iron)))
    from (
      -- Rounded twice, exactly as game.StorageCapacity does: the level's base first, then the bonus product.
      select round(
               (case when coalesce((v.data->'buildings'->>'storage')::int, 0) > 0
                     then round(1000 * power(1.2294, coalesce((v.data->'buildings'->>'storage')::int, 0) - 1))
                     else 500 end)
               * (case when v.data->'bonus'->>'kind' = 'storage' then 1.5 else 1 end)) as cap
    ) c,
    (
      select coalesce(sum((o.data->'give'->>'wood')::float8), 0) as wood,
             coalesce(sum((o.data->'give'->>'clay')::float8), 0) as clay,
             coalesce(sum((o.data->'give'->>'iron')::float8), 0) as iron
      from trade_offers o where o.world_id = v.world_id and o.data->>'originId' = v.id
    ) g
  ), true)
where exists (select 1 from trade_offers o where o.world_id = v.world_id and o.data->>'originId' = v.id);
drop table trade_offers;
