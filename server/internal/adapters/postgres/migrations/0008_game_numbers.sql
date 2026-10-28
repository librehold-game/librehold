-- +goose Up
-- Instant games are numbered from one sequence, so a number is never reused after its world is deleted.
create sequence game_numbers;
alter table worlds add column game_number int;
alter table instant_archive add column game_number int;

-- +goose Down
alter table instant_archive drop column game_number;
alter table worlds drop column game_number;
drop sequence game_numbers;
