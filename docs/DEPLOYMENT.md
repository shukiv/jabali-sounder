# Deployment

Jabali Sounder can be deployed as a standalone VM service or as part of a
container stack. The current test deployment is a standalone VM-style install
on `10.0.3.14`.

## Build Artifacts

Backend binary:

```bash
make build
```

Output:

```text
./bin/jabali-sounder
```

Frontend bundle:

```bash
make ui-build
```

Output:

```text
manager-ui/dist/
```

## Filesystem Layout

Recommended new-install layout:

```text
/opt/jabali-sounder/bin/jabali-sounder
/opt/jabali-sounder/manager-ui/dist/
/etc/jabali-sounder/config.toml
/etc/jabali-sounder/secrets.key
```

Current test-server compatibility layout:

```text
/opt/jabali-manager/bin/jabali-sounder
/opt/jabali-manager/manager-ui/dist/
/etc/jabali-manager/config.toml
/etc/jabali-manager/secrets.key
```

## systemd Unit

Recommended new-install unit:

```ini
[Unit]
Description=Jabali Sounder - central control plane
After=network.target mariadb.service redis-server.service
Wants=mariadb.service redis-server.service

[Service]
Type=simple
ExecStart=/opt/jabali-sounder/bin/jabali-sounder serve
Environment=JABALI_SOUNDER_CONFIG=/etc/jabali-sounder/config.toml
WorkingDirectory=/opt/jabali-sounder
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Reload and start:

```bash
systemctl daemon-reload
systemctl enable --now jabali-sounder.service
```

## nginx

Example nginx shape:

```nginx
server {
    listen 8485;
    server_name _;

    root /opt/jabali-sounder/manager-ui/dist;
    index index.html;

    location /api/v1/ {
        proxy_pass http://127.0.0.1:8484/api/v1/;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /health {
        proxy_pass http://127.0.0.1:8484/health;
    }

    location / {
        try_files $uri /index.html;
    }
}
```

## Deployment Steps

1. Build backend and frontend:

   ```bash
   make build
   make ui-build
   ```

2. Back up current deployment:

   ```bash
   stamp=$(date -u +%Y%m%dT%H%M%SZ)
   mkdir -p /opt/jabali-sounder/backups/$stamp
   cp -a /opt/jabali-sounder/bin /opt/jabali-sounder/backups/$stamp/bin
   cp -a /opt/jabali-sounder/manager-ui/dist /opt/jabali-sounder/backups/$stamp/dist
   ```

3. Copy artifacts:

   ```bash
   install -m 775 ./bin/jabali-sounder /opt/jabali-sounder/bin/jabali-sounder
   rsync -az --delete manager-ui/dist/ /opt/jabali-sounder/manager-ui/dist/
   ```

4. Restart:

   ```bash
   systemctl restart jabali-sounder.service
   systemctl is-active jabali-sounder.service
   curl -fsS http://127.0.0.1:8484/health
   ```

## Current Test Server Deployment

The currently running test server uses:

```text
Host: 10.0.3.14
UI:   http://10.0.3.14:8485/
API:  127.0.0.1:8484
Unit: jabali-manager.service
Exec: /opt/jabali-manager/bin/jabali-sounder serve
```

Deploy there with the compatibility paths:

```bash
make build
make ui-build

stamp=$(date -u +%Y%m%dT%H%M%SZ)
ssh root@10.0.3.14 "set -e; mkdir -p /opt/jabali-manager/backups/$stamp; cp -a /opt/jabali-manager/bin /opt/jabali-manager/backups/$stamp/bin; cp -a /opt/jabali-manager/manager-ui/dist /opt/jabali-manager/backups/$stamp/dist"

rsync -az --delete manager-ui/dist/ root@10.0.3.14:/opt/jabali-manager/manager-ui/dist/
scp -q bin/jabali-sounder root@10.0.3.14:/tmp/jabali-sounder.new

ssh root@10.0.3.14 "set -e; systemctl stop jabali-manager.service; install -m 775 /tmp/jabali-sounder.new /opt/jabali-manager/bin/jabali-sounder; rm -f /tmp/jabali-sounder.new; systemctl start jabali-manager.service; sleep 1; systemctl is-active jabali-manager.service; curl -fsS http://127.0.0.1:8484/health"
```

Do not deploy to managed Panel servers for normal Sounder changes. Managed
servers only need Jabali Panel automation endpoints and automation tokens.
