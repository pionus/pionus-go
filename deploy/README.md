# Deployment

## Build and push

```bash
docker build -t ghcr.io/secbone/pionus:latest .
docker push ghcr.io/secbone/pionus:latest
```

## Kubernetes (existing `lnmp` namespace)

Create the secret first (use a long random value for `secret`):

```bash
kubectl -n lnmp create secret generic pionus-secrets \
  --from-literal=secret='change-this-long-random-value' \
  --from-literal=auth-token='change-this-api-token'

kubectl apply -f deploy/k8s/pionus.yaml
kubectl -n lnmp rollout status deploy/pionus
```

The deployment uses one replica and a `Recreate` strategy: SQLite and the
uploads directory live on one PVC and must not be mounted by multiple writers.

## Caddy

Replace the old PHP block for the blog (the existing `secbone.com` block with
`root * /www/blog` and `import fpm`) with:

```caddyfile
secbone.com, *.secbone.com {
    encode gzip
    tls {
        dns cloudflare {env.CLOUDFLARE_API_TOKEN}
    }
    reverse_proxy pionus-service:8087
}
```

Reload Caddy after changing the Caddyfile.

## First login

Open `https://<domain>/admin`. The imported owner has no password; run:

```bash
kubectl -n lnmp exec deploy/pionus -- ./pionus -config config.yaml \
  -create-admin Secbone -admin-password 'new-password'
```

Then set `site.url` in `/admin/settings` to the production URL. Feeds,
sitemap and canonical links use that value.

## Backups

`scripts/backup.sh` uses SQLite's online backup and archives uploads:

```bash
PIONUS_DB=/app/data/pionus.db PIONUS_UPLOADS=/app/data/uploads \
  ./scripts/backup.sh
```

For k8s, run it from a cron job with the PVC mounted, or copy the files out
first. Keep copies off the node; a PVC is not a backup.

## Migration

Run `pionus-migrate` before starting the new deployment, or from a pod:

```bash
kubectl -n lnmp exec -it deploy/pionus -- ./pionus-migrate \
  -config config.yaml -export /tmp/typecho.json \
  -source-uploads /tmp/usr/uploads
```

Mount or copy the Typecho JSON and uploads into the pod first.
