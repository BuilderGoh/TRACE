# TRACE local development

The development stack builds the frontend and Go server from this checkout. It
uses a separate Compose project (`trace-dev`), database and application volumes,
and localhost ports `18787`/`18788`. It does not use the baseline `data/`
directory or the baseline Compose project's PostgreSQL volume.

## Start

Copy the local-only environment template, then build and start the stack:

```sh
cp .env.trace-dev.example .env.trace-dev
docker compose --env-file .env.trace-dev -f docker-compose.dev.yml -p trace-dev up --build -d
docker compose --env-file .env.trace-dev -f docker-compose.dev.yml -p trace-dev ps
docker compose --env-file .env.trace-dev -f docker-compose.dev.yml -p trace-dev logs -f app
```

Open <http://127.0.0.1:18787>. On first start, the application prints a setup
token in the app logs unless `ARTEX_ADMIN_PASSWORD` is set. Set
`OPENAI_API_KEY` in `.env.trace-dev` to use the configured Kepler OpenAI-compatible
endpoint; the model and base URL can be overridden in that file.

After source changes, run `up --build -d` again to rebuild and replace the
development app container. The `skills/` directory is mounted for live edits.

## Stop and reset

Stop the development services while retaining their named volumes:

```sh
docker compose --env-file .env.trace-dev -f docker-compose.dev.yml -p trace-dev down
```

To explicitly delete only the development database and app data, use the
development project name and `-v`:

```sh
docker compose --env-file .env.trace-dev -f docker-compose.dev.yml -p trace-dev down -v
```

Do not change the project name when starting and stopping this stack; Compose
uses it to scope the development network and volumes.

## Notes

- The image compiles the current frontend and Go source; it does not pull the
  published ARTEX application image.
- The development container includes common command-line tools but is leaner
  than the published runtime image (for example, it does not install Chromium
  or Playwright).
- The application retains upstream ARTEX behavior, identifiers, and tools.
  TRACE's planned DFIR workflows and application-wide safety boundaries are
  not implemented yet. A separate synthetic-only analysis endpoint is
  available when `TRACE_RESTRICTED_DFIR_ENABLED=true`; it requires API
  authentication and accepts only explicitly labelled synthetic event data.
  See `TRACE_DEFENSIVE_EXECUTION.md` for its scope and limitations.
- Keep `.env.trace-dev` local. Never commit API keys, incident evidence,
  credentials, or client data.
