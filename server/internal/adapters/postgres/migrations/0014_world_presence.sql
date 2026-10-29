-- +goose Up
-- The wall time a human last had the world open (a request or a live connection); instant worlds nobody has had
-- open for a while end early. 0 means never.
alter table worlds add column last_present_ms bigint not null default 0;
-- Games running at the deploy start their five minutes now.
update worlds set last_present_ms = (extract(epoch from now()) * 1000)::bigint;

-- +goose Down
alter table worlds drop column last_present_ms;
