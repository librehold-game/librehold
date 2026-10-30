# Deployment

One server with Docker: Postgres, Valkey (the counters of the limits on logins, accounts and reports), the game
server and nightly database backups; optionally Caddy with HTTPS.

By default the game listens on `0.0.0.0:8080` (plain HTTP), and the database and Valkey are reachable only from the
other compose services — no port is exposed. Ports published by Docker bypass UFW, so on a server where HTTPS is handled by Caddy
(from this compose file or a system one), set `LH_BIND=127.0.0.1`.

The server runs the latest published image, `ghcr.io/librehold-game/librehold:latest`, built for amd64 and arm64
(a Raspberry Pi works too). What changed in each release: <https://github.com/librehold-game/librehold/releases>.

## First run

You need Docker with Docker Compose and the files of this directory. Cloning the repository is the easiest way:

```sh
git clone --depth 1 https://github.com/librehold-game/librehold.git
cd librehold/deploy
```

1. Copy `.env.example` to `.env` and set three passwords:
   - `POSTGRES_PASSWORD` and `VALKEY_PASSWORD`, letters and digits only;
   - `LH_ADMIN_PASSWORD` — the admin account (`LH_ADMIN_USERNAME`, `admin` by default) is created on the first
     start, only if the database has no admin yet.

   The game listens on port 8080; change `LH_BIND` and `LH_PORT` for another address or port.
2. Start:

   ```sh
   docker compose up -d
   ```

   Docker downloads the image on the first start.

3. Open `http://<server>:8080/` and log in as admin. Instant games start on their own; long worlds are created in
   the admin panel. Players sign up on the registration page or press **Play now** as guests.

Plain HTTP sends passwords in clear text — fine on a local network, use HTTPS on the internet.

## HTTPS with Caddy

In `deploy/.env` uncomment the HTTPS block (`LH_DOMAIN` with a domain pointing at the server, `LH_COOKIE_SECURE=true`,
`LH_BIND=127.0.0.1`) and start with the profile. The server trusts `X-Forwarded-For` from Caddy's fixed address in the
compose network (`LH_TRUSTED_PROXIES` defaults to `172.30.0.10`); set it yourself only for another proxy in front:

```sh
docker compose --profile https up -d
```

Caddy gets the certificate on its own; the game is at `https://<LH_DOMAIN>/`.

A proxy already running on the host (outside compose) reaches the published port from the compose network's
gateway, so set `LH_TRUSTED_PROXIES=172.30.0.1/32` and `LH_BIND=127.0.0.1` instead of using the profile. The proxy
must replace `X-Forwarded-For` with the visitor's address rather than pass on what the client sent.

## Updating

```sh
docker compose pull
docker compose up -d
```

This fetches the latest release and restarts the server with it; database migrations run on start. Read the
release notes first: an alpha release may reset worlds or need a change in `.env`. A `git pull` now and then brings
newer versions of this directory's files.

## Running your own build

To run a modified version, build the image and change the `server` image in `docker-compose.yml` to its tag:

```sh
docker build -t librehold:dev ..
# docker-compose.yml: image: librehold:dev
docker compose up -d server
```

## Limits

Logins, sign-ups, guest accounts and bug reports are limited per client address, and state reads and orders per
account; the counts live in Valkey. The server does not start while Valkey is unreachable, and while Valkey is down
those requests fail with a server error rather than go unlimited.

A LAN party works as it is: players on the server's own network (private and loopback addresses, a Docker gateway,
the server's own IPv6 /64) are not limited per address, so a room full of players sharing one gateway does not lock
itself out. The limit per account and the one per username still hold. An IPv6 client on the internet counts by its
/64.

## Database backups

The `backup` service writes `librehold-YYYYMMDD.dump` to the `backups` volume every day at 03:30 (Warsaw time)
and deletes backups older than 7 days. On-demand backup:

```sh
docker compose exec backup /bin/bash /backup.sh --once
docker compose exec backup ls -l /backups
```

Copying a backup off the server:

```sh
docker compose cp backup:/backups/librehold-20261019.dump .
```

## Restoring

Stop the server, restore the database from a backup and start it again:

```sh
docker compose stop server
docker compose cp librehold-20261019.dump db:/tmp/restore.dump
docker compose exec db pg_restore -U librehold -d librehold --clean --if-exists /tmp/restore.dump
docker compose start server
```

## Server variables

| variable | default | meaning |
|---|---|---|
| `LH_DATABASE_URL` | set by `docker-compose.yml` from `POSTGRES_PASSWORD` | Postgres connection |
| `LH_HTTP_ADDR` | `:8080` | listen address |
| `LH_WEB_DIR` | `/app/web` | directory with `account/` and `game/` |
| `LH_COOKIE_SECURE` | `true` | session cookie over HTTPS only |
| `LH_TRUSTED_PROXIES` | `172.30.0.10` in compose | addresses or CIDR ranges of reverse proxies whose `X-Forwarded-For` names the client |
| `LH_VALKEY_URL` | set by `docker-compose.yml` from `VALKEY_PASSWORD` | Valkey for the limits, shared by every server process; empty counts them in the one process |
| `LH_WORLD_REQUESTS_PER_MINUTE` | 300 | how many state reads and orders one account can send per minute, over all its worlds |
| `LH_ADMIN_USERNAME`, `LH_ADMIN_PASSWORD` | — | admin account on first start |
| `LH_PUBLIC_URL` | — | site address, e.g. `https://example.com`; empty = address from the request (canonical links, previews, sitemap) |

Every variable in `deploy/.env` reaches the server. The limits on sign-ups, guests, instant games and player worlds
are listed with their defaults in [`docs/development.md`](../docs/development.md#server).
