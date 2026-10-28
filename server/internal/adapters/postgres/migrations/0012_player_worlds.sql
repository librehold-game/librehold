-- +goose Up
-- Player worlds: a host, the codes of the invitation and the results links, the host's state, and the admin's limits.
alter table worlds add column host_account_id bigint references accounts on delete set null;
alter table worlds add column invite_code text unique;
alter table worlds add column results_code text unique;
alter table worlds add column host jsonb;
create index worlds_host on worlds (host_account_id) where host_account_id is not null;

create table host_settings (
  id integer primary key check (id = 1),
  max_seats integer not null,
  max_bots integer not null
);

alter table instant_archive add column kind text not null default 'instant';
alter table instant_archive add column results_code text unique;
alter table instant_archive add column host_account_id bigint references accounts on delete set null;
alter table instant_archive add column host_name text not null default '';
create index instant_archive_host on instant_archive (host_account_id) where host_account_id is not null;

-- +goose Down
drop index instant_archive_host;
alter table instant_archive drop column host_name;
alter table instant_archive drop column host_account_id;
alter table instant_archive drop column results_code;
alter table instant_archive drop column kind;
drop table host_settings;
drop index worlds_host;
alter table worlds drop column host;
alter table worlds drop column results_code;
alter table worlds drop column invite_code;
alter table worlds drop column host_account_id;
