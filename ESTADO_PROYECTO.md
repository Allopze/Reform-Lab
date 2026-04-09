# Estado Actual Del Proyecto

Fecha de revision: 2026-04-09 (undecima pasada)

## Resumen Ejecutivo

El proyecto avanzó en esta undecima sesión con foco en cerrar el siguiente lote concreto de media sobre el stack actual. Además de lo ya resuelto en pasadas anteriores, el backend ahora cubre perfiles web de imagen por tamaño, nuevas salidas AAC/FLAC/M4A/Opus, formatos HEIC/SVG y una primera capa real para presentaciones y hojas de cálculo sin duplicar la lógica entre catálogo, resolución y workers. En esta misma pasada también se endurecieron los flujos reales con fixtures E2E, SVG->PDF vectorial y un contrato frontend/backend más honesto para artefactos ZIP o multiarchivo.

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
- **cobertura ampliada de conversiones** — ahora existen capacidades reales para `pdf-compress`, `doc-to-html`, `txt-to-pdf`, `markdown-to-html`, `markdown-to-pdf`, `image-to-webp`, compresión de imágenes y thumbnails
- **lote no-OCR ampliado** — ahora también existen `pdf-to-html-preview`, `html-to-pdf`, `docx-to-markdown`, `markdown-to-docx`, `image-to-avif`, `video-to-mp3`, `video-to-wav`, `video-to-thumbnails` y `video-contact-sheet`
- **OCR base incorporado** — ahora también existen `pdf-ocr-to-txt`, `pdf-ocr-to-json`, `pdf-ocr-searchable-pdf`, `image-ocr-to-txt` e `image-ocr-to-json`
- **extracción limpia y preview corto** — HTML ya puede resolverse a `html-to-txt` y video ya suma `video-preview-mp4` y `video-preview-webm`
- **perfiles web de imagen** — ahora también existen `image-web-jpg-640`, `image-web-webp-640`, `image-web-avif-640`, `image-web-jpg-1600`, `image-web-webp-1600` e `image-web-avif-1600`
- **audio ampliado** — ahora también existen `audio-to-aac`, `audio-to-flac` y `audio-waveform-png`
- **video ampliado** — ahora también existen `video-to-aac`, `video-to-flac` y `video-waveform-png`
- **formatos de imagen modernos** — ahora también existen `image-heic-to-jpg`, `image-heic-to-png`, `image-heic-to-webp`, `image-svg-to-png`, `image-svg-to-webp` e `image-svg-to-pdf`
- **audio extra y extracción enriquecida** — ahora también existen `audio-to-m4a`, `audio-to-opus`, `video-to-m4a` y `video-to-opus`
- **presentaciones y hojas de cálculo** — ahora también existen `presentation-to-pdf`, `presentation-to-jpg`, `presentation-to-png`, `spreadsheet-to-pdf`, `spreadsheet-to-csv`, `spreadsheet-to-xlsx` y `spreadsheet-to-html`
- **detección y artefactos más honestos** — la detección ahora reconoce `svg`, `csv`, `heic/heif`, `pptx/odp`, `xlsx/ods`, `m4a` y `opus`, y los artefactos multiarchivo se persisten con la extensión real devuelta por el worker
- **artefactos visibles sin heurísticas locales** — `GET /api/jobs/{jobId}` ahora expone `artifactFileName`, `artifactMimeType` y `artifactSize`, y la UI usa esos datos para distinguir salidas ZIP y nombres finales reales
- **fixtures reales para rutas nuevas** — existe un corpus endurecido en `apps/api/tests/fixtures` con variantes válidas complejas y corruptas para HEIF, PPTX y XLSX usado por workers, ingestion y E2E backend
- **detección conservadora de Markdown por contenido** — el sistema no depende de la extensión y solo habilita Markdown cuando el contenido tiene señales suficientes
- **detección documental de HTML** — `text/html` entra al flujo documental real y habilita conversión backend a PDF
- **detección OOXML más robusta** — la ingestión ahora reconoce PPTX/XLSX complejos aunque el sniffing ZIP genérico no los clasifique bien a la primera
- **same-format permitido para `compress` y `preview`** — el resolver sigue bloqueando formato idéntico solo para `convert`
- **workers LibreOffice más honestos** — las rutas documentales ya validan que el archivo esperado exista realmente tras convertir, evitando falsos éxitos con outputs ausentes
- **frontend crítico cubierto** — `apps/web` ya tiene Vitest + Testing Library para los caminos de autenticación, éxito ZIP/multipágina, error de job y error de descarga en `ConversionCard`
- **smoke Docker reproducible** — existe `apps/api/scripts/docker-e2e-smoke.sh` y ya valida la stack real con `HEIF -> PNG`, `SVG -> PDF`, `PPTX -> JPG ZIP` y `XLSX -> CSV`
- **fallback no-cgo para Markdown documental** — `docx-to-markdown` conserva un camino puro Go para que la build de Docker con `CGO_ENABLED=0` no rompa el runtime
- **suite de tests ampliada** — el backend ya expone al menos 99 funciones `Test*` con cobertura específica en capabilities, ingestion y workers

## Realizado En Esta Pasada (Undecima)

### Imagen Y Formatos Modernos

- Se incorporan conversiones reales de `HEIC/HEIF -> JPG, PNG, WebP` usando tooling `libheif`
- Se incorporan conversiones reales de `SVG -> PNG, WebP, PDF`; PNG/WebP siguen rasterizando por `ffmpeg` y `SVG -> PDF` ahora exporta PDF vectorial mediante `librsvg`
- La detección por contenido reconoce `image/svg+xml` y familias `heic/heif` sin depender de extensión

### Audio, Video Y Office

- Audio suma `audio-to-m4a` y `audio-to-opus`
- Video suma `video-to-m4a` y `video-to-opus` reutilizando la misma ruta FFmpeg de extracción limpia sin video
- Presentaciones suman `presentation-to-pdf`, `presentation-to-jpg` y `presentation-to-png`
- Hojas de cálculo suman `spreadsheet-to-pdf`, `spreadsheet-to-csv`, `spreadsheet-to-xlsx` y `spreadsheet-to-html`
- El corpus real de `HEIF`, `PPTX` y `XLSX` ya incluye variantes complejas y corruptas para ejercer decode, detección y exportación documental sin depender de mocks

### Contrato De Artefactos Y UX

- El backend conserva el nombre real del archivo generado por el worker, por ejemplo `slides.zip`, en lugar de normalizar todo a `converted.ext`
- `GET /api/jobs/{jobId}` devuelve metadata real del artefacto y el frontend la usa para presentar ZIPs multiarchivo sin inferir el nombre desde el formato solicitado

### Runtime, Catalogo Y Contrato

- `apps/api/Dockerfile` y `apps/api/Dockerfile.worker` ahora incluyen `libheif-examples`, `libreoffice-calc`, `libreoffice-impress` y `librsvg2-bin`
- El catálogo, el resolver, `cmd/server`, `cmd/worker` y el handler de artefactos quedaron sincronizados para estas nuevas familias
- La detección añade un fallback OOXML por contenido ZIP para reconocer `pptx/xlsx/docx` complejos cuando la librería MIME devuelve un contenedor genérico
- Las conversiones documentales vía LibreOffice ahora comprueban que el artefacto esperado exista antes de declarar éxito, evitando falsos positivos con inputs corruptos
- `docx-to-markdown` mantiene un adaptador con fallback puro Go cuando el build corre con `CGO_ENABLED=0`, lo que desbloquea la imagen Docker sin romper la ruta local con cgo
- La implementación se apoyó en documentación actual consultada vía Context7 para rutas FFmpeg (`m4a`, Opus y render estático), libheif (`heif-dec` / `heif-convert`) y `librsvg` (`rsvg-convert --format=pdf`)

### Validacion Dirigida

- Pasaron validaciones dirigidas en `internal/ingestion`, `internal/workers`, `internal/workers/image`, `internal/workers/document`, `internal/api` y `apps/web`
- La cobertura nueva de `SVG -> PDF` vectorial y `HEIF` depende de disponer `rsvg-convert` y `heif-convert` en el runtime; cuando faltan localmente, los tests asociados se saltan de forma explícita
- La revalidación local con binarios reales quedó verde para HEIF y SVG vectorial
- El smoke de Docker Compose ya pasa sobre la imagen real con `HEIF -> PNG`, `SVG -> PDF`, `PPTX -> JPG ZIP` y `XLSX -> CSV`

## Realizado En Esta Pasada (Decima)

### Media Web, Audio Y Video

- Imagen suma perfiles web dedicados en `jpg`, `webp` y `avif` con topes de 640px y 1600px para entrega ligera sin cambiar la fuente de verdad del catálogo
- Audio suma `audio-to-aac`, `audio-to-flac` y `audio-waveform-png`
- Video suma `video-to-aac`, `video-to-flac` y `video-waveform-png`, reutilizando la misma ruta FFmpeg donde ya correspondía

### Runtime, Catalogo Y Contrato

- `catalog.go`, `cmd/server`, `cmd/worker` y el handler de artefactos quedaron sincronizados para que las nuevas capacidades sean visibles, ejecutables y persistidas con MIME/familia correctos
- Los perfiles web de imagen se modelan como operaciones `optimize`, lo que permite mantener formato o cambiarlo sin romper la regla de bloqueo same-format reservada para `convert`
- La implementación se apoyó en documentación actual de FFmpeg consultada vía Context7 para transcoding AAC/FLAC, `showwavespic` y filtros de escalado

### Validacion Dirigida

- Pasaron validaciones dirigidas en `internal/capabilities`, `internal/workers/audio` e `internal/workers/image`
- `cmd/server` y `cmd/worker` quedaron compilables en la suite dirigida aunque no agregan tests propios

## Realizado En Esta Pasada (Novena)

### OCR Base Para PDF E Imagen

- PDF suma `pdf-ocr-to-txt`, `pdf-ocr-to-json` y `pdf-ocr-searchable-pdf`
- Imagen suma `image-ocr-to-txt` e `image-ocr-to-json`
- La salida JSON queda estructurada por páginas, bloques, líneas y palabras a partir de TSV de Tesseract
- La salida searchable PDF reconstruye una capa buscable sobre páginas rasterizadas del PDF original

### Documento Y Preview

- HTML suma `html-to-txt` como extracción limpia de texto legible sin conservar markup, scripts ni estilos
- Video suma `video-preview-mp4` y `video-preview-webm` como clips ligeros del inicio del archivo para compartir o inspección rápida

### Runtime, Docker Y Fuente Externa

- Se incorporan los engines `tesseract`, `ocr-pdf` y `go-html` en la resolución operativa del catálogo
- `apps/api/Dockerfile` y `apps/api/Dockerfile.worker` ahora instalan `tesseract-ocr` y `tesseract-ocr-spa`
- La implementación se apoyó en documentación actual de Tesseract y FFmpeg consultada vía Context7 para las rutas OCR y de preview corto

### Validacion Dirigida

- Pasaron validaciones dirigidas en `internal/capabilities`, `internal/workers/document`, `internal/workers/image`, `internal/workers/ocrutil`, `internal/workers/pdf` e `internal/workers/video`
- `cmd/server` y `cmd/worker` quedaron compilables en la suite dirigida aunque no agregan tests propios

## Realizado En Esta Pasada (Octava)

### Cobertura No-OCR Cerrada

- PDF suma `pdf-to-html-preview` usando una ruta de preview HTML basada en `pdftohtml`
- Documentos y texto suman `html-to-pdf`, `docx-to-markdown` y `markdown-to-docx`
- Imágenes suman `image-to-avif`
- Video suma extracción de audio (`video-to-mp3`, `video-to-wav`) y previews (`video-to-thumbnails`, `video-contact-sheet`)
- OCR queda fuera de esta pasada y sigue pendiente como trabajo explícito de segunda iteración

### Runtime Y Motores

- Se incorporó `github.com/kreuzberg-dev/html-to-markdown/packages/go/v2` para la ruta DOCX -> HTML -> Markdown
- Se añadió el engine operativo `poppler-html` para distinguir previews HTML de PDF del resto del paquete Poppler
- El engine de audio vía ffmpeg ahora fuerza `-vn`, lo que permite reutilizarlo también para extraer audio desde video
- Los engines DOCX pasan a usar el filtro explícito `docx:Office Open XML Text`, corrigiendo una causa real de fallo en exportaciones DOCX ya declaradas en el catálogo

### Validacion Dirigida

- Pasaron validaciones dirigidas en `internal/capabilities`, `internal/ingestion`, `internal/workers/document`, `internal/workers/image`, `internal/workers/audio`, `internal/workers/pdf` e `internal/workers/video`
- `cmd/server` y `cmd/worker` quedaron compilables en la suite dirigida aunque no agregan tests propios

## Realizado En Esta Pasada (Septima)

### Cobertura Real Del Catalogo De Conversiones

- PDF suma compresión real vía `pdf-compress`
- Documentos suman `doc-to-html` y texto plano suma `txt-to-pdf`
- Markdown suma `markdown-to-html` y `markdown-to-pdf`
- Imágenes suman `image-to-webp`, `image-compress-jpg`, `image-compress-png`, `image-thumbnail-jpg` y `image-thumbnail-png`
- Server y worker registran estas capacidades como parte del runtime efectivo

### Resolucion Y Deteccion

- `CapabilityResolver` ahora solo rechaza formato origen=destino para operaciones `convert`
- `compress` y `preview` pueden producir artefactos del mismo formato sin quedar invisibles para el usuario
- `DetectFormat()` normaliza MIME con parámetros y aplica heurística conservadora para distinguir Markdown de texto plano

### Runtime Y Validacion

- Se añadió `ghostscript` como dependencia operativa para compresión de PDF
- Se añadió `goldmark` para renderizado de Markdown a HTML
- Pasaron validaciones dirigidas en `cmd/server`, `cmd/worker`, `internal/capabilities`, `internal/ingestion`, `internal/workers/document`, `internal/workers/image` e `internal/workers/pdf`

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

1. **Ampliar el corpus de fixtures** con muestras corruptas y variantes complejas para PDF, imágenes, audio y video, además de seguir extendiendo Office más allá del lote inicial `HEIF/PPTX/XLSX`.
2. **Tests frontend** para componentes críticos (`conversion-card`, `user-dashboard`, `admin-dashboard`) y contratos visibles de UI.

## Diagnóstico Final

> MVP funcional con autenticación, ownership, detección dinámica de engines, rate limiting per-IP, cancelación y retry de jobs, retención configurable por familia, feature flags operativas para capacidades, progreso granular, panel admin con auditoría y métricas operativas, dashboards enriquecidos, fixtures E2E reales para Office/HEIF y una UI que ya refleja el artefacto final real. El camino para producción sigue requiriendo validación E2E en Docker, ampliar el corpus a casos corruptos/complejos y sumar tests frontend para la UI crítica.