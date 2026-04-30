# Reform Lab

Reform Lab es una plataforma de conversión de archivos que intenta comportarse como un sistema serio.
Eso significa que no decide nada importante por la extensión del archivo, no inventa capacidades en el frontend y no confunde “subí un archivo” con “ya resolví un pipeline de procesamiento”. Parece obvio. No lo es tanto en este tipo de proyectos.

## Qué hace

- recibe archivos desde la web
- detecta el formato real
- extrae metadatos relevantes
- resuelve solo capacidades compatibles
- crea jobs asíncronos de conversión
- valida la salida antes de persistirla
- expone descargas, estados y trazabilidad

## Qué no hace

- no confía en nombres como `final-final-ahora-si.pdf`
- no pone lógica de producto en componentes UI con demasiada autoestima
- no mete conversiones pesadas en request/response porque el caos ya tiene demasiados fans
- no vende la fantasía de “soporta todo” cuando faltan motores o restricciones operativas

## Arquitectura en 20 segundos

```text
Upload
	-> Validate
	-> Detect real format
	-> Extract metadata
	-> Resolve capabilities
	-> Create conversion request
	-> Enqueue job
	-> Execute in worker
	-> Validate output
	-> Persist artifact
	-> Download / inspect status
```

La separación de capas no es decoración:

- `apps/web` presenta estados y consume contratos
- `apps/api` valida, autentica y orquesta
- `ingestion` detecta y clasifica
- `capabilities` decide qué está permitido
- `workers` ejecutan y validan salidas
- `storage` guarda originales, temporales y artefactos
- `observability` deja evidencia de lo que pasó cuando inevitablemente alguien pregunte “qué rompió esto”

## Stack real

- frontend: Next.js 15, React 19, Tailwind CSS 4, Vitest
- backend: Go 1.25, Chi, SQLite, JWT, cola en proceso o Redis
- motores y binarios: Poppler, Ghostscript, LibreOffice, librsvg, FFmpeg, libheif, Tesseract y otros según capacidad

No todos los motores son obligatorios para arrancar. Sí son obligatorios si esperas que una capacidad dependiente funcione y no solo salga muy bonita en una demo.

## Estructura del repositorio

- `apps/web`: UI, flujos de usuario, adaptadores HTTP y tests de componentes
- `apps/api`: API, auth, repositorios, orquestación, workers embebidos y tests
- `docs`: arquitectura, dominio, seguridad, testing, operación y ADRs
- `.github/instructions`: reglas específicas para agentes y asistentes

## Requisitos de desarrollo

- Node.js 20 o superior
- npm
- Go 1.25
- binarios del sistema si quieres cobertura real de conversiones avanzadas

## Arranque local

### 1. Prepara variables de entorno

```bash
cp .env.example .env
```

Edita al menos:

- `JWT_SECRET`
- `CORS_ORIGIN` si abres la web desde otra IP, hostname local o dispositivo externo
- `NEXT_PUBLIC_API_URL` si no quieres usar el API local en `4040`

### 2. Instala dependencias

```bash
npm install
cd apps/web && npm install
cd ../api && go mod download
cd ../..
```

### 3. Levanta el stack local

```bash
npm run dev
```

Servicios por defecto:

- web: `http://localhost:5050`
- api: `http://localhost:4040`
- health: `http://localhost:4040/api/health`

## Comandos útiles

| Comando | Qué hace |
| --- | --- |
| `npm run dev` | levanta frontend y API usando el `.env` raíz |
| `npm run release` | genera `releases/release.zip` con los archivos necesarios para desplegar con Docker Compose |
| `cd apps/web && npm test` | ejecuta la suite frontend |
| `cd apps/web && npm run build` | build de Next.js |
| `cd apps/api && go test ./...` | ejecuta la suite del backend |
| `cd apps/api && ENV_FILE=../../.env go run ./cmd/server` | arranca solo el API |
| `bash apps/api/scripts/docker-e2e-smoke.sh` | smoke test Docker del API |

## Despliegue con Docker Compose

El repositorio ahora incluye un `docker-compose.yml` en la raíz orientado a servidor.
Ese stack levanta:

- `web`: Next.js en `5050`
- `api`: backend Go en `8080`
- `worker`: worker standalone con Redis
- `redis`: cola persistente

Persistencia del despliegue:

- archivos originales, temporales, artefactos y SQLite quedan en `./runtime/data`
- Redis persiste en `./runtime/redis`
- el servicio one-shot `data-permissions` prepara la propiedad de `./runtime/data` antes de arrancar `api` y `worker`; los procesos runtime corren como usuario no-root sin capacidades añadidas

Eso significa que, si despliegas el proyecto en `/opt/reform-lab`, los archivos subidos quedarán físicamente bajo `/opt/reform-lab/runtime/data/originals` y los artefactos bajo `/opt/reform-lab/runtime/data/artifacts`.

### Pasos de despliegue

1. crear el archivo de entorno de producción:

```bash
cp .env.production.example .env
```

2. editar al menos:

- `JWT_SECRET`
- `SECRET_ENCRYPTION_KEY` si vas a guardar secretos admin-managed en SQLite como SMTP o firmas de webhook
- `CORS_ORIGIN`
- `NEXT_PUBLIC_API_URL`
- `APP_URL`
- `BOOTSTRAP_ADMIN_EMAILS` si quieres restringir el primer admin

Modelo recomendado de produccion:

- termina TLS en un reverse proxy delante de `web` y `api`
- deja `WEB_BIND_ADDRESS` y `API_BIND_ADDRESS` en loopback salvo que tengas una razon fuerte para exponerlos
- usa URLs publicas `https://...` en `CORS_ORIGIN`, `NEXT_PUBLIC_API_URL` y `APP_URL`

3. levantar el stack:

```bash
docker compose up -d --build
```

4. verificar salud del API:

```bash
curl http://127.0.0.1:8080/api/health
```

5. verificar el acceso publico a traves del proxy:

```bash
curl -I https://YOUR_SERVER_OR_DOMAIN
```

### Empaquetado de release

Para generar un ZIP listo para mover al servidor:

```bash
npm run release
```

El comando crea `releases/release.zip` en la raíz del repo.
Ese ZIP incluye únicamente los archivos necesarios para reconstruir y levantar el stack Docker de producción desde el servidor.

Notas operativas:

- si cambias `NEXT_PUBLIC_API_URL`, vuelve a construir la web con `docker compose up -d --build web`
- si `EXPOSE_METRICS=true`, configura también `METRICS_TOKEN` para no dejar `/metrics` abierto
- el compose de `apps/api/` sigue existiendo como stack de desarrollo y smoke del backend, no como despliegue full stack de servidor

## Variables de entorno

### Archivos canónicos

- `.env` en la raíz: archivo principal para desarrollo local full stack
- `.env.example` en la raíz: plantilla completa y sincronizada con el runtime real
- `.env.production.example` en la raíz: plantilla orientada al despliegue con el `docker-compose.yml` root
- `apps/web/.env.example`: plantilla mínima solo para arrancar el frontend por separado; no intenta documentar todo el sistema
- `ENV_FILE`: override opcional para que el API cargue otro archivo distinto

### Runtime del API

| Variable | Obligatoria | Default de código | Qué hace |
| --- | --- | --- | --- |
| `APP_ENV` | no | `development` | controla el modo general; `production` exige `REDIS_URL` y bloquea el auto-admin del primer registro |
| `PORT` | no | `8080` | puerto HTTP del API |
| `DATABASE_PATH` | no | `./data/reform.db` | ruta de la base SQLite |
| `MIGRATIONS_PATH` | no | `./migrations` | ruta de migraciones SQL |
| `STORAGE_BASE_PATH` | no | `./data` | base para originales, temporales y artefactos |
| `CORS_ORIGIN` | no, pero muy recomendable | `http://localhost:3000` | lista separada por comas con orígenes exactos permitidos para la web; en producción normal debería ser un origen `https://` del proxy público |
| `LOG_LEVEL` | no | `info` | nivel de logs estructurados |
| `JWT_SECRET` | sí | sin fallback válido | secreto para firmar sesión JWT; mínimo 32 caracteres y sin placeholders banales |
| `SECRET_ENCRYPTION_KEY` | no, pero muy recomendable si usas SMTP/webhooks configurados desde admin | vacío | clave para cifrar en reposo secretos persistidos por panel admin; acepta 32 bytes raw o base64 de 32 bytes |
| `REDIS_URL` | no en local, sí en producción | vacío | activa cola Redis; vacío usa cola en proceso |
| `BOOTSTRAP_ADMIN_EMAILS` | no | vacío | lista separada por comas con los emails autorizados a reclamar el primer admin en `production` |
| `REQUIRE_VERIFIED_EMAIL_FOR_SENSITIVE_ACTIONS` | no | `false` | si está activo, exige email verificado para mutaciones sensibles autenticadas como cambios admin, webhooks, usuarios y soporte |
| `APP_URL` | no | hereda `CORS_ORIGIN` | URL pública base usada por emails y links generados por backend; en producción debería ser la URL HTTPS pública del proxy |
| `MAX_ACTIVE_JOBS_PER_GUEST_SESSION` | no | `1` | limita cuántas conversiones activas puede mantener una sesión anónima simultáneamente |
| `EXPOSE_METRICS` | no | `false` | expone `/metrics` para Prometheus |
| `METRICS_TOKEN` | no | vacío | protege `/metrics` con bearer token cuando está configurado |
| `TRUST_PROXY_HEADERS` | no | `false` | usa headers tipo `X-Forwarded-*` al calcular IP y seguridad; actívalo solo si la API recibe tráfico exclusivamente desde un proxy de confianza que sobrescribe esas cabeceras |

### Concurrencia y cuotas

| Variable | Obligatoria | Default de código | Qué hace |
| --- | --- | --- | --- |
| `IN_PROCESS_WORKER_CONCURRENCY` | no | `2` | concurrencia del worker embebido cuando no hay Redis |
| `WORKER_CONCURRENCY` | no | `2` | concurrencia del worker standalone |
| `USER_UPLOADS_PER_MINUTE` | no | `12` | cuota por usuario o IP para subidas |
| `USER_UPLOAD_BURST` | no | `4` | burst permitido para subidas |
| `USER_CONVERSIONS_PER_MINUTE` | no | `6` | cuota por usuario o IP para conversiones |
| `USER_CONVERSION_BURST` | no | `3` | burst permitido para conversiones |
| `MAX_ACTIVE_JOBS_PER_USER` | no | `3` | límite de jobs activos por usuario |
| `GUEST_CUMULATIVE_QUOTA_BYTES` | no | `26214400` | cuota acumulada total por sesión anónima, en bytes |
| `REGISTERED_CUMULATIVE_QUOTA_BYTES` | no | `524288000` | cuota acumulada total por usuario registrado, en bytes |

### Retención y limpieza

| Variable | Obligatoria | Default de código | Qué hace |
| --- | --- | --- | --- |
| `ORIGINAL_RETENTION_HOURS` | no | `24` | cuánto se conservan originales |
| `TEMP_RETENTION_HOURS` | no | `6` | cuánto viven temporales de trabajo |
| `ARTIFACT_TTL_HOURS` | no | `24` | TTL base para artefactos |
| `ARTIFACT_TTL_HOURS_PDF` | no | hereda `ARTIFACT_TTL_HOURS` si no existe; local recomendado `48` | override para familia PDF |
| `ARTIFACT_TTL_HOURS_IMAGE` | no | hereda `ARTIFACT_TTL_HOURS` si no existe; local recomendado `12` | override para familia imagen |
| `ARTIFACT_TTL_HOURS_DOCUMENT` | no | hereda `ARTIFACT_TTL_HOURS` si no existe; local recomendado `24` | override para familia documento |
| `ARTIFACT_TTL_HOURS_AUDIO` | no | hereda `ARTIFACT_TTL_HOURS` si no existe; local recomendado `72` | override para familia audio |
| `ARTIFACT_TTL_HOURS_VIDEO` | no | hereda `ARTIFACT_TTL_HOURS` si no existe; local recomendado `96` | override para familia video |

### Feature flags

| Variable | Obligatoria | Default de código | Qué hace |
| --- | --- | --- | --- |
| `FEATURE_DISABLE_CAPABILITIES` | no | vacío | CSV de capability IDs deshabilitados |
| `FEATURE_DISABLE_ENGINES` | no | vacío | CSV de engines deshabilitados |

### Email y observabilidad

| Variable | Obligatoria | Default de código | Qué hace |
| --- | --- | --- | --- |
| `SMTP_HOST` | no | vacío | host SMTP; vacío desactiva el envío de emails |
| `SMTP_PORT` | no | `587` | puerto SMTP |
| `SMTP_USER` | no | vacío | usuario SMTP |
| `SMTP_PASSWORD` | no | vacío | password o token SMTP |
| `SMTP_FROM` | no | `noreply@example.com` | remitente usado por el sistema |
| `SMTP_USE_TLS` | no | `true` | activa TLS al conectar con SMTP |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | vacío | endpoint OTLP HTTP; vacío deja el tracing como no-op |

### Frontend

| Variable | Obligatoria | Default de código | Qué hace |
| --- | --- | --- | --- |
| `NEXT_PUBLIC_API_URL` | no, pero recomendada | `http://localhost:8080` | base URL del API consumido por Next.js; en producción detrás de proxy suele ser la misma origin pública `https://...` |
| `NEXT_PUBLIC_SENTRY_DSN` | no | vacío | activa Sentry en cliente, edge y server cuando existe |
| `SENTRY_ORG` | no | vacío | organización de Sentry usada en el build de Next cuando se habilita Sentry |
| `SENTRY_PROJECT` | no | vacío | proyecto de Sentry usado en el build de Next cuando se habilita Sentry |

Nota importante sobre `NEXT_PUBLIC_API_URL`:

- si apunta a loopback (`localhost` o `127.0.0.1`) y abres la web desde una URL LAN, el frontend reutiliza el hostname actual para no morir en un precioso “Load failed”
- eso no reemplaza `CORS_ORIGIN`; solo evita que el cliente intente hablar con el loopback equivocado

Notas sobre Sentry:

- `NEXT_PUBLIC_SENTRY_DSN` es la llave que realmente activa la integración
- `SENTRY_ORG` y `SENTRY_PROJECT` solo tienen sentido si estás construyendo la web con soporte Sentry habilitado
- la advertencia de build de Sentry/OpenTelemetry sobre dependencias dinámicas en instrumentación Prisma se acepta como no bloqueante mientras `npm run build`, tests y auditorías sigan pasando; revisarla al actualizar Sentry u OpenTelemetry

### Variables de despliegue Docker Compose

Estas variables no las consume directamente `config.go`; las usa el `docker-compose.yml` root para puertos, recursos y build de imágenes.

| Variable | Obligatoria | Default del compose | Qué hace |
| --- | --- | --- | --- |
| `WEB_BIND_ADDRESS` | no | `0.0.0.0` | IP de bind del contenedor `web` en el host |
| `WEB_HOST_PORT` | no | `5050` | puerto expuesto para la web |
| `API_BIND_ADDRESS` | no | `0.0.0.0` | IP de bind del contenedor `api` en el host |
| `API_HOST_PORT` | no | `8080` | puerto expuesto para la API |
| `API_MEMORY_LIMIT` | no | `1024m` | límite de memoria del contenedor `api` |
| `API_CPUS` | no | `1.00` | límite de CPU del contenedor `api` |
| `WORKER_MEMORY_LIMIT` | no | `2048m` | límite de memoria del contenedor `worker` |
| `WORKER_CPUS` | no | `2.00` | límite de CPU del contenedor `worker` |

### Variables operativas adicionales

| Variable | Dónde se usa | Qué hace |
| --- | --- | --- |
| `ENV_FILE` | scripts root y loader del API | fuerza al backend a cargar un archivo `.env` específico |
| `BASE_URL` | `apps/api/scripts/docker-e2e-smoke.sh` | redefine la URL base del smoke test Docker |

## Auditoría de `.env`

Revisión hecha sobre el estado actual del repo:

- el `.env` raíz es el archivo correcto para desarrollo local full stack
- el `.env.example` estaba desalineado con el runtime real y se corrigió
- `JWT_SECRET=dev-secret-change-me` era una trampa: el backend lo rechaza por validación
- `apps/web/.env.example` apuntaba a `8080`, pero el stack local de este repo usa `4040`
- `CORS_ORIGIN` ahora debe entenderse como lista de orígenes exactos separados por comas, no como una cadena única decorativa

## Testing

La estrategia del repo no es “si compila, ya veremos”. El baseline esperado cubre:

- unit tests para reglas y validaciones
- integration tests para infraestructura relevante
- contract tests para endpoints
- end-to-end y smoke tests para flujos críticos
- corpus de archivos reales, complejos y corruptos

Consulta también:

- `docs/testing/test-strategy.md`
- `docs/architecture/system-overview.md`
- `docs/architecture/repo-map.md`

## Docker

El compose del API vive en `apps/api/docker-compose.yml`.
Si activas Redis, el worker standalone deja de ser decoración y pasa a ser requisito.

## Problemas típicos

### “Load failed” al registrar o hacer login

Revisa en este orden:

- que `NEXT_PUBLIC_API_URL` apunte al API correcto
- que `CORS_ORIGIN` incluya el origen exacto desde el que abres la web
- que hayas reiniciado `npm run dev` después de tocar `.env`

### El motor aparece como unavailable

No es misterio, es una dependencia nativa ausente. Mira los logs de arranque del API y verifica el binario correspondiente.

### El frontend arranca, pero el API no

Lo normal es un `JWT_SECRET` inválido, una ruta de migraciones rota o una dependencia del sistema faltante. El repositorio no está poseído; casi siempre es configuración.

## Lectura recomendada

1. `AGENTS.md`
2. `docs/architecture/system-overview.md`
3. `docs/domain/glossary.md`
4. `docs/domain/capabilities-catalog.md`
5. `docs/security/file-handling.md`
6. `docs/testing/test-strategy.md`

## Cierre

Reform Lab no intenta ser mágico. Intenta ser entendible, auditable y difícil de romper por accidente.
Que en software eso suene ambicioso ya dice bastante del mercado.
