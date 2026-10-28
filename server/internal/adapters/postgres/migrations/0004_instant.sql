-- +goose Up
-- How an instant world ended, and its running totals (see game.WorldState).
alter table worlds add column state jsonb not null default '{}';

-- +goose Down
alter table worlds drop column state;
