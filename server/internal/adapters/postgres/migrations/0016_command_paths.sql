-- +goose Up
-- The box around the leg each army is on, so that a player's state reads only the armies passing near their
-- villages, not every army on the map.
alter table commands add column path box;
update commands c set path = box(e.a, e.b)
from (
  select world_id, id,
    case
      when origin_id = '' and data ? 'point' then point((data->'point'->>'x')::float8, (data->'point'->>'y')::float8)
      when origin_id ~ '^v:[0-9]+:[0-9]+$' then point(split_part(origin_id, ':', 2)::float8, split_part(origin_id, ':', 3)::float8)
    end as a,
    case
      when (target_id = '' or type = 'intercept') and data ? 'point' then point((data->'point'->>'x')::float8, (data->'point'->>'y')::float8)
      when target_id ~ '^v:[0-9]+:[0-9]+$' then point(split_part(target_id, ':', 2)::float8, split_part(target_id, ':', 3)::float8)
    end as b
  from commands
) e
where c.world_id = e.world_id and c.id = e.id and e.a is not null and e.b is not null;
create index commands_path on commands using gist (path);

-- +goose Down
drop index commands_path;
alter table commands drop column path;
