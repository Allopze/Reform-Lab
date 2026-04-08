# Estado Actual Del Proyecto

Fecha de revision: 2026-04-08 (sexta pasada)

## Resumen Ejecutivo

El proyecto avanzó significativamente en esta sexta sesión con foco en observabilidad operativa, trazabilidad administrativa, resiliencia del flujo de conversión y control de rollout. Además de lo ya resuelto en las pasadas anteriores, ahora el backend puede desactivar capacidades o engines concretos mediante feature flags de entorno sin duplicar la lógica entre endpoints, resolución y workers.

Hoy el proyecto ya tiene:

- autenticación con roles `admin` y `user`
- primer usuario registrado promovido automáticamente a `admin`
- ownership en archivos, jobs y artefactos
- endpoints sensibles protegidos por autenticación
- control de acceso por recurso
- dashboards reales enriquecidos para usuario y admin conectados a API
- frontend consumiendo capacidades reales del backend para las salidas disponibles
- limpieza periódica de artefactos expirados
- detección dinámica de engines disponibles
- **rate limiting per-IP** (100 req/s, burst 200) con limpieza automática de IPs inactivas
- **security headers** (X-Content-Type-Options, X-Frame-Options, Referrer-Policy, etc.)
- cancelación de jobs vía `POST /api/jobs/{jobId}/cancel` con soporte en frontend
- retención configurable global y por familia (`ARTIFACT_TTL_HOURS`, `ARTIFACT_TTL_HOURS_PDF`, `..._IMAGE`, `..._DOCUMENT`, `..._AUDIO`, `..._VIDEO`) expuesta en `/api/health`
- endpoint admin de engines `GET /api/admin/engines`
- errores de conversión clasificados con mensajes orientados al usuario final
- **tests E2E automatizados** — 10 tests httptest cubriendo registro, login, upload, capacidades, conversión, ownership, dashboards, admin restrictions, cancelación y retry de fallidos
- **progreso granular de jobs** — de 0→50→100 hardcoded a 10→20→30→40→70→80→95→100 con pasos significativos
- **dashboards enriquecidos** — admin con succeeded/cancelled counts, usuario con progress bars y expiración de artefactos
- **panel admin enriquecido** — overview con `successRatePct`, `averageDurationSec`, disponibilidad de engines, uso por engine y auditoría reciente filtrable
- **retry backend de jobs fallidos** — nuevo endpoint `POST /api/jobs/{jobId}/retry` con ownership, reuso de capability original y auditoría `job_retried`
- **retención diferenciada por familia** — TTL configurable por `pdf`, `image`, `document`, `audio` y `video`, expuesto también en `/api/health`
- **retry UX de jobs fallidos** — el panel de usuario ya permite reintentar jobs `failed` y refresca el historial al crear el nuevo intento
- **feature flags backend para capacidades** — `FEATURE_DISABLE_CAPABILITIES` y `FEATURE_DISABLE_ENGINES` ya afectan resolución, creación, retry, worker y visibilidad operativa en `health`
- **suite de tests: 46 tests** en 7 packages con tests

## Realizado En Esta Pasada (Sexta)

### Feature Flags De Capacidades (Backend)

- Nuevo runtime de feature flags en `internal/capabilities` como parte de la fuente de verdad de resolución
- `Resolve()` e `IsEligible()` ya filtran capabilities o engines deshabilitados por entorno
- `POST /api/conversions` y `POST /api/jobs/{jobId}/retry` heredan el comportamiento al seguir usando `capabilities.IsEligible()`
- El worker también valida flags antes de ejecutar para evitar que jobs ya encolados ignoren una desactivación operativa
- `/api/health` ya expone `featureFlags.disabledCapabilities` y `featureFlags.disabledEngines`
- Los endpoints admin de engines y overview usan disponibilidad efectiva, combinando runtime probe + flags operativas
- `.env.example` y `apps/api/docker-compose.yml` ya documentan `FEATURE_DISABLE_CAPABILITIES` y `FEATURE_DISABLE_ENGINES`
- Se añadieron tests de contrato y unitarios para capabilities deshabilitadas por flag y engines deshabilitados operativamente

## Realizado En Esta Pasada (Quinta)

### Reintentos Y Retención Diferenciada (Backend)

- Nuevo endpoint `POST /api/jobs/{jobId}/retry` con validación de ownership y restricción a jobs en estado `failed`
- `orchestrator.Service.RetryFailedJob()` crea un nuevo job usando el mismo archivo y la misma capability, evitando reescrituras ad hoc en handlers
- Nuevo evento de auditoría `job_retried` para enlazar el nuevo intento con el job fallido original
- `config.Config` ahora soporta `ARTIFACT_TTL_HOURS_PDF`, `ARTIFACT_TTL_HOURS_IMAGE`, `ARTIFACT_TTL_HOURS_DOCUMENT`, `ARTIFACT_TTL_HOURS_AUDIO` y `ARTIFACT_TTL_HOURS_VIDEO`
- El worker calcula la expiración del artefacto según la familia real del formato de salida y `/api/health` expone tanto el TTL default como el mapa por familia
- `.env.example` y `apps/api/docker-compose.yml` ya incluyen las variables nuevas para que la política opere igual en dev y contenedores

### Retry UX En Panel De Usuario Y Cobertura

- `apps/web/src/components/user-dashboard.tsx` ahora muestra acción `Reintentar` para jobs en estado `failed`
- El panel refresca el historial tras crear el nuevo intento y evita navegación manual para volver a encolar
- `apps/web/src/lib/api.ts` expone `retryJob()` y el contrato de health ahora incluye `artifactTTLHoursByFamily`
- `apps/web/src/components/admin-dashboard.tsx` incorpora el filtro y la etiqueta `job_retried` en la auditoría reciente
- `internal/api/api_e2e_test.go` suma cobertura de retry real simulando un job fallido y `internal/workers/handler_test.go` valida el TTL por familia

### Métricas Operacionales Y Auditoría Admin (Backend)

- `AdminDashboardData` ahora expone `successRatePct`, `averageDurationSec`, `availableEngines`, `totalEngines`, `unavailableEngines`, `engineUsage` y `recentAudit`
- `DashboardHandler.AdminOverview()` compone datos de SQL + disponibilidad real de engines desde `capabilities.DefaultProber`
- `AuditRepository` ahora soporta `ListRecent()` para listar eventos recientes de auditoría
- El overview admin ya devuelve eventos auditables reales sin depender del scrape externo de Prometheus

### Auditoría Más Completa Del Ciclo De Vida De Jobs

- Nuevo tipo de evento `job_cancelled`
- `orchestrator.transitionJob()` ahora registra eventos de auditoría para `job_started`, `job_completed`, `job_failed` y `job_cancelled`
- El worker registra `artifact_created` al persistir el artefacto final
- Se eliminó la duplicación anterior del evento `job_completed` en el worker para mantener una única fuente de verdad del estado del job

### Panel Admin Conectado A Observabilidad Operativa

- `apps/web/src/components/admin-dashboard.tsx` ahora muestra indicadores operativos reales: tasa de éxito, duración media y disponibilidad de engines
- El panel expone uso por engine agregado desde backend
- Se añadió un feed de auditoría reciente con filtros por tipo de evento (`upload`, `job_created`, `job_completed`, `job_failed`, `job_cancelled`, `job_retried`, `artifact_created`)
- La UI ya no depende de leer `/metrics`; consume contratos explícitos del backend admin

## Realizado En Esta Pasada (Tercera)

### Per-IP Rate Limiting (P1.4 → Resuelto)

- Reescritura completa de `internal/api/middleware/ratelimit.go`
- Rate limiter ahora es per-IP con `sync.Mutex` + `map[string]*ipLimiter`
- Goroutine de limpieza cada 3 minutos elimina IPs inactivas >5 min
- Función `realIP()` extrae IP real de `X-Forwarded-For` → `X-Real-IP` → `RemoteAddr`
- Tests expandidos a 5: burst, rechazo, aislamiento per-IP, respeto a X-Forwarded-For, security headers

### Tests E2E Automatizados Con httptest (P1.2 → Resuelto)

- Nuevo archivo `internal/api/api_e2e_test.go` con 11 tests E2E
- Setup completo: SQLite temporal, migraciones reales, router completo, httptest.Server
- Helpers: `doPost()`, `doGet()`, `uploadPNG()` (genera PNG válido en memoria)
- Comparte métricas Prometheus via `sync.Once` para evitar panic por doble registro
- Tests cubiertos:
  - Health endpoint
  - Registro + login + primer usuario admin
  - Upload de archivo + resolución de capacidades
  - Creación de conversión → job creado
  - Acceso no autenticado bloqueado
  - Aislamiento de ownership entre usuarios
  - Dashboard con datos reales
  - Restricción admin (usuario normal no accede)
  - Cancelación de jobs
  - Retry de jobs fallidos
  - Capability deshabilitada por feature flag

### Progreso Granular De Jobs (P1.3 → Resuelto)

- `UpdateProgress(ctx, jobID, percent)` nuevo método en orchestrator
- Worker handler reporta progreso en cada paso:
  - 10% → job iniciado (MarkRunning)
  - 20% → preparando workspace
  - 30% → resolviendo engine
  - 40% → ejecutando conversión
  - 70% → validando output
  - 80% → guardando artefacto
  - 95% → finalizando
  - 100% → completado (MarkSucceeded)
- Frontend ya consumía `progress` — ahora recibe valores reales intermedios

### Dashboards Enriquecidos (P2.1 y P2.2 → Resueltos)

**Admin dashboard:**
- Nuevos conteos: `succeededJobs`, `cancelledJobs` en `AdminDashboardData`
- Frontend muestra: Exitosos, Fallidos, Cancelados además de En cola/En ejecución

**User dashboard:**
- `progress` añadido a `UserDashboardJob`
- `succeededJobs` y `failedJobs` añadidos a `UserDashboardData`
- Frontend: barra de progreso visual con porcentaje para jobs activos
- Frontend: fecha de expiración mostrada junto al botón de descarga
- Sidebar: conteo de exitosos y fallidos

### Corrección De Tests Rotos

- Eliminadas declaraciones `package` duplicadas en 3 archivos de test (ratelimit_test.go, engines_test.go, job_test.go)

## Realizado En Pasadas Anteriores

### Detección Dinámica De Engines (P1.1 → Resuelto)

- Nuevo archivo `internal/capabilities/engines.go` con `EngineProber`
- Probe ejecutado una vez al startup vía `exec.LookPath` para cada binario requerido
- `Resolve()` e `IsEligible()` ahora filtran capacidades cuyo engine no está disponible
- Nuevo endpoint admin `GET /api/admin/engines` expone el estado de cada engine
- Engines mapeados: `libreoffice`, `poppler` (pdftoppm, pdftotext), `ffmpeg`, `go-image` (siempre disponible)
- Logs al startup muestran el resultado del probe de cada engine

### Rate Limiting Y Security Hardening (P1.4 → Resuelto)

- Nuevo middleware `RateLimit(rps, burst)` basado en `golang.org/x/time/rate`
- Nuevo middleware `MaxBodySize(n)` usando `http.MaxBytesReader`
- Nuevo middleware `SecurityHeaders` con cabeceras OWASP recomendadas
- Aplicados globalmente en el router: 100 req/s burst 200, 500 MB body, headers de seguridad
- Tests de middleware: burst, rechazo, headers

### Cancelación De Jobs (P1.3 → Parcialmente Resuelto)

- Nuevo método `CancelJob()` en el orchestrator
- Nuevo handler `POST /api/jobs/{jobId}/cancel` con control de ownership
- Manejo de estado `cancelled` en `transitionJob()`
- Frontend: botón "Cancelar" en el estado de conversión
- Frontend: polling detecta status `cancelled` y resetea la UI
- API client: `cancelJob()` agregado

### Retención Como Contrato Explícito (P1.2 → Resuelto)

- Nuevo campo de config `ArtifactTTLHours` (env: `ARTIFACT_TTL_HOURS`, default: 24)
- Worker handler usa el TTL configurado en vez de hardcoded 24h
- Health endpoint ahora responde con `{ status, retention: { artifactTTLHours } }`
- Docker-compose expone `ARTIFACT_TTL_HOURS` como variable configurable
- API client: `getHealthInfo()` para consumir la política desde frontend

### Mejor Manejo De Errores De Conversión (P1.6 → Resuelto)

- Función `classifyError()` en el worker handler
- Mensajes clasificados por tipo: engine no disponible, timeout, resultado inválido, error de storage, fallo del motor
- Mensajes orientados al usuario final en español

### Suite De Tests Expandida (P0.2 → Resuelto)

Tests antes: 3 (auth: 1, retention: 1, build-only verification)
Tests segunda pasada: 23 en 5 packages
Tests sexta pasada: 46 en 7 packages con tests:

| Package | Tests | Cobertura |
| --- | --- | --- |
| `api` (E2E) | 11 | Health, register/login, upload/capabilities, conversion, unauthorized, ownership, dashboards, admin restrictions, cancel, retry, feature flags |
| `capabilities` | 19 | Engine prober, disponibilidad efectiva, resolver (filtros por formato, engine, tamaño, protección, flags), eligibilidad, ByID |
| `domain` | 4 | Transiciones de job (válidas e inválidas), IsTerminal, IsSourceSupported |
| `orchestrator` | 5 | CreateAndEnqueue, lifecycle success, lifecycle failure, purge de retención, transición inválida |
| `middleware` | 5 | Rate limit (burst, rechazo, per-IP isolation, X-Forwarded-For), security headers |
| `auth` | 1 | Primer admin / siguiente user |
| `workers` | 1 | TTL diferenciado por familia de salida |

### Frontend

- API client: `cancelJob()`, `retryJob()`, `getHealthInfo()` con mapa `artifactTTLHoursByFamily` y snapshot de feature flags
- Conversion card: botón de cancelar durante conversión
- Polling: manejo de status `cancelled`
- User dashboard: acción `Reintentar` para jobs fallidos
- Categories: documentado que `targetFormats` son hints de UI, no fuente de verdad

### Docker

- Dockerfiles actualizados de `golang:1.23-alpine` a `golang:1.25-alpine`
- Docker-compose: `JWT_SECRET`, `ARTIFACT_TTL_HOURS`, overrides por familia y feature flags como env vars explícitas

## Validación

- `go test ./...`: 46 tests pasando en 7 packages con tests (incluyendo 11 E2E)
- `go build ./...`: compilación limpia
- `npx tsc --noEmit`: sin errores de TypeScript
- `npx next lint`: sin errores; Next.js muestra warnings informativos por deprecación de `next lint` y detección de múltiples lockfiles

## Estado Por Área

| Área | Estado | Observación |
| --- | --- | --- |
| Backend API | Funcional y endurecido | Ownership, roles, rate limiting per-IP, security headers, body limits |
| Ingestion | Parcialmente funcional | Detección real implementada; metadata PDF depende de binarios externos |
| Capabilities | Completamente funcional | Detección dinámica de engines, resolución real con filtros y feature flags |
| Workers | Funcional con progreso granular | Engines registrados, errores clasificados, TTL configurable por familia, progreso 10→100 |
| Auth | Funcional | Login/register/me, roles, ownership, JWT |
| Persistencia | Mejorada | SQLite + storage local con ownership y purge de expirados |
| Frontend | Funcional para usuario autenticado | Capacidades del backend, cancelación, retry de fallidos, progress bars, expiración visible |
| Panel usuario | Enriquecido | Progreso visual, conteo exitosos/fallidos, expiración de artefactos y retry de jobs fallidos |
| Panel admin | Enriquecido | Conteos, indicadores operativos, engines y auditoría filtrable |
| Observabilidad | Básica | Logs, auditoría, métricas, probe al startup y snapshot de flags en health |
| Testing | Sólida | 46 tests: E2E completos, dominio, capabilities, orchestrator, middleware, auth y workers |
| Docker/operación | Mejorado | Go 1.25, env vars de seguridad, falta validar flujo E2E en contenedores |

## Lo Faltante

### Prioridad P0

1. **Validar flujo end-to-end en contenedores Docker.**
   Dockerfiles corregidos, env vars documentadas, pero falta ejecutar y verificar: register → upload → capabilities → conversion → download dentro de Docker.

### Prioridad P1

1. **Validar conversiones PDF reales en el entorno de desarrollo.**
  Los binarios `pdftoppm` y `pdftotext` ya están presentes localmente; falta ejecutar el flujo real de PDF → capacidades → conversión → descarga y verificar outputs.

### Prioridad P2

1. **Corpus de fixtures** para testing de formatos (PDFs, imágenes, audio, video reales y corruptos).
2. **Tests frontend** para componentes críticos (`conversion-card`, `user-dashboard`, `admin-dashboard`) y contratos visibles de UI.

## Diagnóstico Final

> MVP funcional con autenticación, ownership, detección dinámica de engines, rate limiting per-IP, cancelación y retry de jobs, retención configurable por familia, feature flags operativas para capacidades, progreso granular, panel admin con auditoría y métricas operativas, dashboards enriquecidos y 46 tests automatizados (incluyendo 11 E2E). El camino para producción requiere validación E2E en Docker, validación PDF real en dev, corpus de fixtures y tests frontend para la UI crítica.