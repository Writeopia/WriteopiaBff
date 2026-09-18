# Writeopia BFF

A small, dependency-free Go reverse proxy that bridges the webapp's HttpOnly
session cookie into a bearer token, so that the GCP API Gateway in front of
Writeopia's Cloud Run services — which can only read a JWT from an
`Authorization` header or query param, never from a cookie — works for the
web client the same way it already does for the desktop and mobile apps.

## Why this exists

Traffic flow for the webapp is:

```
app.writeopia.io/api/*  -->  Load Balancer  -->  BFF  -->  API Gateway  -->  Cloud Run
```

The webapp logs in via `/api/auth/login/web`, which stores the access token
in an HttpOnly cookie (`writeopia_access`) instead of returning it in the
response body, so the webapp's own JS can never read it (XSS protection).
That also means the webapp can't attach it as an `Authorization` header
itself. This service is the bridge: for every request, if there is no
`Authorization` header already but there is a `writeopia_access` cookie, it
adds `Authorization: Bearer <cookie value>` and forwards everything else
(method, path, query, body, other headers/cookies, response) unchanged. It
performs **no JWT verification** of its own — that stays the responsibility
of API Gateway (see `writeopia-v2-api` in `WriteopiaScripts2/api-gateway-v2.yaml`).

Native/mobile clients already send a real `Authorization` header and hit API
Gateway directly (`writeopia.io/api/*`), bypassing the BFF entirely.

## Endpoints

- `GET /health` — liveness/startup probe, always `200 ok`, does not touch upstream.
- everything else — proxied to `UPSTREAM_BASE_URL` with the cookie-to-bearer
  bridge described above.

## Configuration

| Env var            | Required | Description                                                          |
|---------------------|----------|------------------------------------------------------------------------|
| `UPSTREAM_BASE_URL` | yes      | Base URL to proxy to (the API Gateway hostname, e.g. `https://writeopia-v2-gateway-xxxx.ew.gateway.dev`) |
| `PORT`              | no       | Port to listen on (default `8080`)                                    |
| `SERVICE_NAME`      | no       | Used only in startup logs (default `bff`)                             |

## Run locally

```bash
UPSTREAM_BASE_URL=https://writeopia-v2-gateway-86fiw5qt.ew.gateway.dev go run .
```

## Test

```bash
go test ./...
```

## Build & push the container image

Deployed by Terraform (`WriteopiaScripts2/cloud-run.tf`, `google_cloud_run_v2_service.bff`)
as `writeopia-bff`, image path
`${region}-docker.pkg.dev/${project}/${repository_name}/writeopia-bff:${cloudrun_bff_image_tag}`
— by default that's `europe-west4-docker.pkg.dev/writeopia/<repository_name>/writeopia-bff:<tag>`.

```bash
docker build -t europe-west4-docker.pkg.dev/writeopia/<repository_name>/writeopia-bff:<tag> .
docker push europe-west4-docker.pkg.dev/writeopia/<repository_name>/writeopia-bff:<tag>
```

Then bump `cloudrun_bff_image_tag` in `WriteopiaScripts2/terraform.tfvars` (or
pass `-var`) and `terraform apply`.

## Design notes

- No JWT library, no session store, no database: the service is stateless and
  trusts API Gateway to do the actual verification, which is why it can stay
  a ~150-line dependency-free Go binary and boot in milliseconds.
- Ingress for the `writeopia-bff` Cloud Run service is locked to
  `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER` — only this project's own load
  balancer can reach it (see `WriteopiaScripts2/cloud-run.tf` /
  `cloud-run-backends.tf`).
- Cookies and Authorization headers are deliberately never logged.
