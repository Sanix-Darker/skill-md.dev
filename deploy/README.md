# Skillf Deployment

## Files

- `skillf.service`: systemd unit template for `skillf` on server.
- `Caddyfile.skillf.example`: reverse-proxy template using `example.com` placeholders.

## Notes

- Do not commit real secrets or production hostnames in tracked files.
- Store sensitive values in `/etc/skillf/skillf.env` (for example `GITHUB_TOKEN`).
- Keep the public example files generic and operator-owned.
- Apply the real production hostname only in server-local config during rollout.
- The tracked systemd unit uses `StateDirectory=skillf`, so SQLite state and the SSH host key live under `/var/lib/skillf/`.
- The example binds the web process to `127.0.0.1:8082` so it can coexist with other local reverse-proxied services on the same host.

## Production rollout (manual)

1. Build a release binary for target platform and copy to `/usr/local/bin/skillf`.
2. Copy `deploy/skillf.service` to `/etc/systemd/system/skillf.service` and replace `example.com` with your real hostname before enabling it.
3. Create `/etc/skillf/skillf.env` as needed for optional API tokens.
4. Copy `deploy/Caddyfile.skillf.example` content into your Caddy config and replace `example.com` with your real hostname.
5. Enable and start service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now skillf.service
```

6. Verify:

```bash
curl -fsS http://127.0.0.1:8082/health
curl -fsS https://example.com/health
curl -fsS https://example.com/api/system
ssh -p 2222 example.com
```

If the service is not directly externally reachable for SSH, configure firewall and NAT as appropriate for your environment. If SSH access should be public, open `2222/tcp` on the host.
