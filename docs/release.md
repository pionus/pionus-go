# Releases and container images

Pionus publishes multi-architecture container images to GitHub Container
Registry (GHCR) from `.github/workflows/release.yml`.

## Image

```text
ghcr.io/pionus/pionus-go
```

## Version tags

A stable release is a Git tag such as:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow publishes:

```text
0.1.0
0.1
0
v0.1.0
sha-<commit>
latest
```

`latest` is updated **only for stable tags** (`vX.Y.Z` without a pre-release
suffix) and always points to the newest release. Pre-release tags such as
`v0.2.0-rc1` do not move `latest`.

Pushes to `master` publish:

```text
edge
sha-<commit>
```

They do not move `latest`, so `latest` remains a stable release.

## Application version

The version is embedded at build time:

```bash
docker build \
  --build-arg VERSION=0.1.0 \
  --build-arg COMMIT=abc123 \
  --build-arg BUILD_DATE=2026-10-25T00:00:00Z \
  -t ghcr.io/pionus/pionus-go:local .
```

Check it with:

```bash
./pionus -version
```

The `/health` endpoint also reports the version.

## Deployment updates

`deploy/k8s/pionus.yaml` uses:

```yaml
image: ghcr.io/pionus/pionus-go:latest
imagePullPolicy: Always
```

After a new stable tag is published, restart the deployment to pull it:

```bash
kubectl -n lnmp rollout restart deploy/pionus
kubectl -n lnmp rollout status deploy/pionus
```

For reproducible rollbacks, deploy a version tag instead of `latest`:

```bash
kubectl -n lnmp set image deploy/pionus \
  pionus=ghcr.io/pionus/pionus-go:0.1.0
```

## Package visibility

GHCR packages default to private. For other people to pull the image without
credentials, set the package visibility to public in the GitHub package
settings. For a private package, the Kubernetes namespace needs an
`imagePullSecrets` entry (`ghcr-secrets` in the provided manifest).
