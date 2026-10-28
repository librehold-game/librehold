-- +goose Up
-- A human row's player key outlives the account: totals keep counting deleted players, and forgetting an account
-- finds the rows of games it left before the end. Never served publicly.
alter table instant_leaderboard add column player_key text not null default '';
create index instant_leaderboard_key on instant_leaderboard (player_key) where player_key <> '';

-- +goose Down
drop index instant_leaderboard_key;
alter table instant_leaderboard drop column player_key;
