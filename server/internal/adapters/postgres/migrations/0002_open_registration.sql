-- +goose Up
drop table access_codes;

-- +goose Down
create table access_codes (
  id bigserial primary key,
  code text not null unique,
  note text not null default '',
  created_at timestamptz not null default now(),
  used_by bigint references accounts,
  used_at timestamptz,
  revoked_at timestamptz
);
