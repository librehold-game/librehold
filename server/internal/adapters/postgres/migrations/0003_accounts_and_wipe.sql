-- +goose Up
-- Pre-release: worlds are wiped instead of migrated (their stored text and names change format).
truncate worlds cascade;

alter table accounts
  add column locale text not null default 'en' check (locale in ('en', 'pl')),
  add column is_guest boolean not null default false,
  add column player_key text not null default md5(random()::text || clock_timestamp()::text),
  add column tutorial_done boolean not null default false,
  add column last_seen_at timestamptz not null default now();
alter table accounts add constraint accounts_player_key unique (player_key);
create index accounts_guests_seen on accounts (last_seen_at) where is_guest;

alter table players drop constraint players_account_id_fkey;
alter table players add constraint players_account_id_fkey
  foreign key (account_id) references accounts on delete set null;

-- +goose Down
alter table players drop constraint players_account_id_fkey;
alter table players add constraint players_account_id_fkey foreign key (account_id) references accounts;
drop index accounts_guests_seen;
alter table accounts drop constraint accounts_player_key;
alter table accounts drop column last_seen_at, drop column tutorial_done, drop column player_key,
  drop column is_guest, drop column locale;
