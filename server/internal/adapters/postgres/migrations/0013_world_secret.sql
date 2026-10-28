-- +goose Up
-- A world's secret keys the rolls players must not foresee (game.HiddenSeed); its seed only draws the map. Worlds
-- from before get a secret of their own; packed logs from before keep none and replay from the seed, as they ran.
alter table worlds add column secret bytea;
update worlds set secret = decode(replace(gen_random_uuid()::text || gen_random_uuid()::text, '-', ''), 'hex');
alter table worlds alter column secret set not null;
alter table worlds add constraint worlds_secret_length check (octet_length(secret) = 32);
alter table action_log_packed add column secret bytea not null default ''::bytea;
alter table action_log_packed alter column secret drop default;

-- +goose Down
alter table action_log_packed drop column secret;
alter table worlds drop column secret;
