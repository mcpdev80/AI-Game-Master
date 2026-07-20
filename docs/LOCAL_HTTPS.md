# Local HTTPS for LAN demo

This setup is for internal/LAN-only hosting with a self-signed certificate. It is intended for local device testing of microphone, camera, and audio flows over HTTPS.

It does not create a public internet deployment.

## 1. Choose an internal hostname and host IP

Example:

- hostname: `dungeon-master.local`
- host IP: `192.168.178.50`

Add that hostname on every client device that should open the app:

- Linux/macOS: `/etc/hosts`
- Windows: `C:\Windows\System32\drivers\etc\hosts`

Example entry:

```text
192.168.178.50 dungeon-master.local
```

## 2. Provide a certificate or generate a self-signed one

From the repository root:

### Option A: use an existing certificate

```bash
chmod +x scripts/generate_local_https_cert.sh
./scripts/generate_local_https_cert.sh \
  dungeon-master.local 192.168.178.50 \
  --cert /path/to/fullchain.pem \
  --key /path/to/privkey.pem
```

This copies the provided files into:

- `docker/certs/local-cert.pem`
- `docker/certs/local-key.pem`

### Option B: generate a self-signed certificate

```bash
chmod +x scripts/generate_local_https_cert.sh
./scripts/generate_local_https_cert.sh dungeon-master.local 192.168.178.50 30
```

This generates:

- `docker/certs/local-cert.pem`
- `docker/certs/local-key.pem`

These files are ignored by Git.

## 3. Start the normal stack plus the HTTPS proxy

```bash
docker compose --profile https up -d --build --wait
```

Default local HTTPS ports:

- redirect/http: `3080`
- https: `3443`

Open:

```text
https://dungeon-master.local:3443
```

## 4. Trust the certificate on test devices

If you generated a self-signed certificate, the browser will otherwise warn or block secure features.

For real microphone/camera testing with a self-signed cert, import and trust `docker/certs/local-cert.pem` on the desktop or phone you use for the session.

If you copied an already trusted certificate chain, use your normal trust process for that certificate instead.

## 5. Operational notes

- The HTTPS proxy only forwards to the local `web` service.
- API calls still stay same-origin via the Next.js `/api/*` rewrite path.
- PostgreSQL and Redis remain internal Docker services and are not exposed through the proxy.
- If the host IP changes, regenerate the certificate with the new IP.
