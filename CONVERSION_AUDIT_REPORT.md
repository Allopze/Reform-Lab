# Auditoria de logica de conversion de archivos

Fecha: 2026-05-10  
Repositorio: `/home/allopze/dev/reform-lab`  
Alcance: `apps/api`, `apps/web`, documentacion de arquitectura/dominio/seguridad/testing y tests cercanos.  
Metodo: auditoria estatica profunda + ejecucion focalizada de tests + smoke real local + smoke Docker full-stack.

## Resumen ejecutivo

El sistema esta bien encaminado: la fuente de verdad funcional vive en el backend (`capabilities.Catalog` + resolver), la UI no inventa conversiones despues de subir archivos, los jobs son asincronos, los outputs se validan antes de persistirse y existen controles razonables de cuota, ownership, limpieza, cancelacion, retencion y auditoria.

La respuesta corta a la pregunta principal es:

> Las conversiones estan modeladas de forma consistente y segura, pero hay riesgos reales de falso fallo en conversiones validas y una inconsistencia fuerte entre el contrato de retries declarado y el comportamiento efectivo.

Hallazgos principales:

1. **Importante:** PDFs de salida validos y grandes pueden fallar por una validacion de EOF que solo mira los primeros 128 KiB.
2. **Importante:** previews/optimizaciones legitimas pueden fallar si el artifact pesa menos del 1% del input.
3. **Importante:** `MaxRetries` se declara y se expone, pero las conversiones se encolan con retry automatico deshabilitado y los retries manuales no tienen contador/limite.
4. **Importante:** HTML exportado por LibreOffice puede perder assets companion, generando descargas HTML incompletas.
5. **Medio:** en Redis/worker standalone, la API declara engines disponibles sin probar el runtime real del worker; si los contenedores divergen, se muestran capacidades que luego fallan.

Estado tras fixes del 2026-05-10:

- Hallazgos 1, 2, 3, 4 y 5: corregidos o mitigados con cambios de codigo y tests.
- Hallazgo 6: documentado como contrato operativo de ingestion; no se cambio comportamiento porque la dependencia es parte de la validacion segura actual.
- Riesgo residual principal: integrar el smoke real en CI/release gate y ampliar corpus negativo/edge, no la ausencia de smoke manual.

## Verificacion ejecutada

Comando ejecutado:

```bash
cd apps/api
go test ./internal/capabilities ./internal/ingestion ./internal/workers ./internal/api/handlers -count=1 -timeout=180s
```

Resultado: OK.

Esta suite cubre resolucion de capacidades, deteccion/validacion de ingestion, validacion de outputs y handlers cercanos. Por si sola no sustituye un smoke real con Poppler, LibreOffice, FFmpeg, Tesseract, Ghostscript, libheif y librsvg; por eso se agregaron y ejecutaron smokes local y Docker en las pasadas 6 y 7.

Verificacion final tras fixes:

```bash
cd apps/api
go test ./internal/capabilities ./internal/ingestion ./internal/workers ./internal/api/handlers -count=1 -timeout=180s
go test ./internal/... -count=1 -timeout=180s
go vet ./...

cd apps/web
npm run test -- admin-system-panel.test.tsx
npm run lint

cd apps/api
BASE_URL=http://127.0.0.1:4040 scripts/conversion-smoke.sh
scripts/docker-e2e-smoke.sh
```

Resultado: OK.

## Registro de fixes

### Pasada 1 - validacion de outputs

Fecha: 2026-05-10  
Estado: completada

Cambios realizados:

- Corregido `apps/api/internal/workers/output_validation.go` para validar PDFs leyendo el header al inicio y buscando `%%EOF` en una ventana final del archivo, en vez de exigirlo dentro de los primeros 128 KiB.
- Agregado `readOutputValidationTail` con limite de lectura de 64 KiB para mantener la validacion acotada y evitar cargar PDFs grandes completos en memoria.
- Ajustada la regla global de "output menor al 1% del input" para que no aplique a operaciones `preview` y `optimize`, ademas de las excepciones ya existentes para `extract` y `compress`.
- Mantenidos los minimos absolutos por formato, de modo que previews/optimizaciones pequenas siguen teniendo que parecer artifacts validos del formato esperado.
- Agregados tests en `apps/api/internal/workers/output_validation_test.go` para:
  - aceptar PDF valido con `%%EOF` fuera del sample inicial de 128 KiB;
  - aceptar previews pequenos cuando el input es grande;
  - aceptar optimizaciones pequenas cuando el input es grande;
  - seguir rechazando conversiones `convert` sospechosamente pequenas.

Verificacion de la pasada:

```bash
cd apps/api
go test ./internal/workers -count=1 -timeout=180s
```

Resultado: OK.

Faltante tras esta pasada:

- Resolver el contrato real de retries: `MaxRetries` sigue expuesto/declarado pero no limita retries manuales ni gobierna la cola de conversiones.
- Corregir o documentar el export HTML de LibreOffice con assets companion.
- Agregar visibilidad operativa para divergencia API/worker en engines declarados vs ejecutables.
- Documentar binarios de ingestion requeridos por familia.
- Ejecutar una suite mas amplia despues de la siguiente pasada.

Siguientes pasos naturales:

1. Implementar limite real de retry manual usando metadata persistida por job raiz o, si se decide no hacerlo todavia, dejar de exponer `MaxRetries` como contrato operativo.
2. Revisar el export HTML de documentos y decidir entre ZIP con recursos companion o HTML autocontenido.
3. Cerrar con `go test ./internal/capabilities ./internal/ingestion ./internal/workers ./internal/api/handlers -count=1 -timeout=180s`.

### Pasada 2 - contrato de retries manuales

Fecha: 2026-05-10  
Estado: completada

Decision aplicada:

- Se mantienen los retries automaticos de cola deshabilitados para conversiones (`TaskOptions.MaxRetries = 0`), preservando el comportamiento existente y evitando reejecuciones automaticas con artifacts parciales.
- `ExecutionLimits.MaxRetries` pasa a gobernar los retries manuales, que son los retries visibles y auditables del producto.

Cambios realizados:

- Agregada migracion `apps/api/migrations/017_job_retry_attempts.sql` con:
  - `jobs.source_job_id`
  - `jobs.attempt_number`
  - indice `idx_jobs_source_job_id`
- Extendida la entidad `domain.Job` con `SourceJobID` y `AttemptNumber`.
- Agregado error sentinel `domain.ErrRetryLimitExceeded`.
- Actualizado `repository.JobRepository` SQLite para persistir y leer lineage/attempt en creacion y consulta de jobs.
- Actualizado `orchestrator.RetryFailedJob` y `RetryFailedJobForGuest` para:
  - rechazar retries si el job fuente no esta en `failed`;
  - rechazar retries cuando `sourceJob.AttemptNumber >= cap.ExecutionLimits.MaxRetries`;
  - crear el nuevo job con `attempt_number = source.attempt_number + 1`;
  - conservar el job raiz en `source_job_id` incluso al reintentar un retry fallido;
  - auditar `sourceJobId`, `rootSourceJobId`, `attemptNumber`, `maxRetries` y `capabilityId`.
- Actualizado `POST /api/jobs/{jobId}/retry`, retry batch de usuario y retry batch admin para devolver conflicto claro cuando se alcanza el limite.
- Corregido el retry batch de usuario para usar el flujo de retry del orquestador por job, en vez de crear jobs nuevos con `CreateAndEnqueueBatch` sin lineage de retry.
- Agregados tests en `apps/api/internal/orchestrator/service_test.go` para:
  - persistir `source_job_id` y `attempt_number`;
  - rechazar un retry cuando ya se alcanzo `MaxRetries`.

Verificacion de la pasada:

```bash
cd apps/api
go test ./internal/orchestrator ./internal/repository ./internal/database ./internal/api/handlers -count=1 -timeout=180s
```

Resultado: OK.

Faltante tras esta pasada:

- Revisar el export HTML de LibreOffice con assets companion.
- Agregar visibilidad operativa para divergencia API/worker en engines declarados vs ejecutables.
- Documentar binarios de ingestion requeridos por familia.
- Ejecutar suite focalizada amplia y, si el entorno tiene motores, smoke real de conversiones.

Siguientes pasos naturales:

1. Resolver `doc-to-html`/`spreadsheet-to-html` para que preserve recursos companion en un ZIP o documentar explicitamente que el output es HTML sanitizado sin assets externos.
2. Agregar metadata operativa de engines ejecutables por worker al health/admin.
3. Actualizar documentacion de ingestion vs conversion engines.

### Pasada 3 - HTML de LibreOffice con assets companion

Fecha: 2026-05-10  
Estado: completada

Decision aplicada:

- `ToHTMLEngine` sigue produciendo HTML plano por defecto para usos internos, como `docx-to-markdown`, donde el siguiente paso necesita leer HTML UTF-8 directo.
- Las capacidades finales `doc-to-html` y `spreadsheet-to-html` registran `ToHTMLEngine{PackageCompanions: true}` para empaquetar recursos companion cuando LibreOffice los emite.
- Si no hay recursos companion, el artifact sigue siendo `.html`; si los hay, el artifact real pasa a `.zip` y el handler ya usa la extension real para formato, MIME, nombre y validacion.

Cambios realizados:

- Actualizado `apps/api/internal/workers/document/to_html.go`:
  - agregado flag `PackageCompanions`;
  - agregado empaquetado ZIP de HTML sanitizado + archivos companion bajo el temp dir de conversion;
  - validacion de rutas relativas antes de agregarlas al ZIP;
  - compresion `Deflate` y preservacion de nombres relativos.
- Actualizado `apps/api/internal/workers/registry.go` para activar `PackageCompanions` solo en:
  - `doc-to-html`
  - `spreadsheet-to-html`
- Actualizado `apps/api/internal/workers/output_validation.go` para aceptar ZIPs cuyo primer entry valido sea `text/html`, ademas de previews JPG/PNG.
- Agregados tests en:
  - `apps/api/internal/workers/document/html_sanitize_test.go` para empaquetado con y sin assets companion;
  - `apps/api/internal/workers/output_validation_test.go` para ZIP HTML valido.

Verificacion de la pasada:

```bash
cd apps/api
go test ./internal/workers ./internal/workers/document -count=1 -timeout=180s
```

Resultado: OK.

Faltante tras esta pasada:

- Agregar visibilidad operativa para divergencia API/worker en engines declarados vs ejecutables.
- Documentar binarios de ingestion requeridos por familia.
- Ejecutar suite focalizada amplia.
- Ejecutar smoke real de motores si el entorno lo permite.

Siguientes pasos naturales:

1. Revisar health/runtime status existente para exponer mejor engines ejecutables por worker.
2. Actualizar documentacion de seguridad/operacion con binarios necesarios en ingestion.
3. Correr suite amplia y corregir cualquier regresion transversal.

### Pasada 4 - visibilidad API/worker e ingestion binaries

Fecha: 2026-05-10  
Estado: completada

Cambios realizados:

- Agregada migracion `apps/api/migrations/018_worker_engine_status.sql` con `worker_status.engines_json`.
- Extendida `repository.WorkerStatusSnapshot` con `Engines map[string]bool`.
- Actualizado heartbeat de workers para persistir disponibilidad efectiva de engines probados por el proceso worker.
- Actualizado worker standalone para ejecutar `capabilities.DefaultProber.Probe()` y reportar su snapshot de engines.
- Actualizado server in-process para reportar el mismo snapshot del prober local cuando el worker embebido esta activo.
- Actualizado `/api/admin/health` para exponer:
  - `runtime.workers.apiEngineMode`: `probed` o `declared`;
  - `runtime.workers.apiEngineAvailability`;
  - `runtime.workers.workers[].engines`.
- Agregado test `TestWorkerStatusHeartbeatPersistsEngineAvailability`.
- Actualizado `docs/operations/runbooks.md` con:
  - lectura operativa de `apiEngineMode`;
  - revision de `workers[].engines`;
  - contrato de binarios de ingestion.
- Actualizado `docs/security/file-handling.md` para separar dependencias de inspeccion en ingestion de engines de conversion.

Verificacion de la pasada:

```bash
cd apps/api
go test ./internal/repository ./internal/database ./internal/api/handlers ./internal/workers ./cmd/server ./cmd/worker -count=1 -timeout=180s
```

Resultado: OK.

Faltante tras esta pasada:

- Ejecutar suite focalizada amplia.
- Ejecutar smoke real con motores externos/corpus representativo si el entorno lo permite.
- Revisar si la UI/admin deberia destacar visualmente divergencias de engines, no solo exponerlas en health JSON.

Siguientes pasos naturales:

1. Correr `go test ./internal/capabilities ./internal/ingestion ./internal/workers ./internal/api/handlers -count=1 -timeout=180s`.
2. Correr `go test ./internal/... -count=1 -timeout=180s` si la focalizada pasa.
3. Revisar diff completo y cerrar el informe con estado final.

Estado final de verificacion:

- Suite focalizada amplia: OK.
- Suite backend interna completa: OK.
- No se ejecuto smoke real con todos los motores externos/corpus representativo en esta pasada.

### Pasada 5 - UI admin para divergencia de engines

Fecha: 2026-05-10  
Estado: completada

Cambios realizados:

- Actualizados tipos frontend en `apps/web/src/lib/api.ts` para consumir:
  - `runtime.workers.apiEngineMode`
  - `runtime.workers.apiEngineAvailability`
  - `runtime.workers.workers[].engines`
- Actualizado `apps/web/src/components/admin-system-panel.tsx` para mostrar:
  - modo de engines de la API (`probed`, `declared` o desconocido);
  - estado de paridad API/workers;
  - divergencias por worker, separando engines que la API declara disponibles pero el worker reporta ausentes, y engines disponibles solo en worker;
  - chips compactos con engines reportados por cada worker.
- Actualizados mensajes en `apps/web/messages/es.json`.
- Actualizado `apps/web/src/components/admin-system-panel.test.tsx` con cobertura para:
  - paridad sin divergencias;
  - divergencia API/worker visible (`libreoffice` faltante en worker, `tesseract` solo worker).

Verificacion de la pasada:

```bash
cd apps/web
npm run test -- admin-system-panel.test.tsx
```

Resultado: OK.

Faltante tras esta pasada:

- Crear smoke real automatizado para conversiones por familia critica.
- Ejecutar el smoke en el entorno local y registrar binarios/casos omitidos.
- Ejecutar lint/test frontend mas amplio si el tiempo lo permite.

Siguientes pasos naturales:

1. Crear script de smoke con fixtures generados en runtime y skips claros por binario faltante.
2. Ejecutar el smoke y documentar resultado en este informe.
3. Correr `npm run lint` o tests frontend focalizados adicionales si la UI queda estable.

### Pasada 6 - smoke local automatizado de conversiones

Fecha: 2026-05-10  
Estado: completada

Cambios realizados:

- Agregado `apps/api/scripts/conversion-smoke.sh`, smoke local contra una API ya disponible.
- El script no levanta ni destruye Docker; usa `BASE_URL` y espera `/api/health`.
- Genera en runtime fixtures pequenos para:
  - PDF
  - PNG
  - WAV
  - DOCX
  - SVG
  - MP4 si existe `ffmpeg` local
- Reutiliza fixtures versionados cuando existen:
  - HEIF
  - PPTX
  - XLSX
- Escenarios cubiertos:
  - `PDF -> TXT`
  - `PNG -> WebP`
  - `WAV -> MP3`
  - `MP4 -> GIF`
  - `DOCX -> PDF`
  - `HEIF -> PNG`
  - `SVG -> PDF`
  - `PPTX -> JPG ZIP`
  - `XLSX -> CSV`
- El script marca `SKIP` cuando el runtime no acepta el upload o no ofrece la capability, y marca `FAIL` si una conversion ofrecida no llega a `succeeded` o el artifact no valida.
- Actualizado `docs/operations/runbooks.md` con uso del smoke local y diferencias frente al smoke Docker.

Verificacion de la pasada:

```bash
cd apps/api
bash -n scripts/conversion-smoke.sh

# API local aislada en 4040 con SQLite/storage temporales y worker embebido
BASE_URL=http://127.0.0.1:4040 scripts/conversion-smoke.sh
```

Resultado del smoke local:

```text
PASS pdf-to-txt
PASS png-to-webp
PASS wav-to-mp3
PASS mp4-to-gif
PASS docx-to-pdf
PASS heif-to-png
PASS svg-to-pdf
PASS pptx-to-jpg-zip
PASS xlsx-to-csv
Summary: pass=9 skip=0 fail=0
```

Faltante tras esta pasada:

- Ejecutar el smoke Docker full-stack para validar Redis + worker standalone + Compose desde cero.
- Correr lint frontend y suite backend/frontend final.

Siguientes pasos naturales:

1. Ejecutar `npm run lint` en `apps/web`.
2. Reejecutar suites backend relevantes tras la incorporacion del smoke.
3. Ejecutar `apps/api/scripts/docker-e2e-smoke.sh` en entorno Docker dedicado.

Estado de verificacion tras Pasada 6:

```bash
cd apps/web
npm run lint
npm run test -- admin-system-panel.test.tsx

cd apps/api
go test ./internal/... -count=1 -timeout=180s
go vet ./...
```

Resultado: OK.

### Pasada 7 - smoke Docker full-stack

Fecha: 2026-05-10  
Estado: completada

Cambios realizados:

- Ejecutado `apps/api/scripts/docker-e2e-smoke.sh`, que construye la imagen, levanta Docker Compose, espera `/api/health`, corre conversiones reales y limpia el stack al terminar.
- Corregido `apps/api/scripts/docker-e2e-smoke.sh` para que las llamadas internas a `docker compose exec` y `docker compose cp` durante la generacion del fixture MP4 reciban `JWT_SECRET`, evitando errores de interpolacion de Compose.
- Ajustada la generacion del fixture MP4 para preferir `ffmpeg` local cuando esta disponible, manteniendo fallback al contenedor API. Esto evita depender de copiar un archivo temporal desde el contenedor cuando el host ya puede crear el fixture de forma determinista.

Verificacion de la pasada:

```bash
cd apps/api
bash -n scripts/docker-e2e-smoke.sh scripts/conversion-smoke.sh
scripts/docker-e2e-smoke.sh
```

Resultado del smoke Docker:

```text
PDF -> TXT
PNG -> WebP
WAV -> MP3
MP4 -> GIF
DOCX -> PDF
HEIF -> PNG
SVG -> PDF
PPTX -> JPG ZIP
XLSX -> CSV
Docker E2E smoke passed
```

Faltante tras esta pasada:

- Integrar `apps/api/scripts/conversion-smoke.sh` o `apps/api/scripts/docker-e2e-smoke.sh` en CI/release gate segun el perfil operativo deseado.
- Ampliar corpus negativo/edge para archivos corruptos, protegidos, muy grandes y formatos con recursos externos.
- Optimizar cache del build Docker si el smoke full-stack se vuelve parte frecuente de CI; la capa de dependencias nativas es pesada.

Siguientes pasos naturales:

1. Decidir si CI debe usar smoke local contra API ephemeral, smoke Docker full-stack o ambos en etapas distintas.
2. Si se usa Docker full-stack en CI, aislar volumenes, puertos y secrets por job.
3. Si se usa smoke local, arrancar API con `ENV_FILE` aislado para evitar heredar `.env` de desarrollo.

## Flujo auditado

El flujo real observado coincide con el modelo del repo:

1. `POST /api/files` recibe upload, aplica limites, guarda staging temporal, detecta tipo real y extrae metadatos.
2. `GET /api/files/{fileId}/capabilities` y `POST /api/files/capabilities/batch` resuelven capacidades desde backend.
3. `POST /api/conversions` o `/api/conversions/batch` valida elegibilidad y crea jobs.
4. La cola in-process o Asynq ejecuta el worker.
5. El worker crea temp dir, ejecuta engine, valida output, persiste artifact y marca job final.
6. La UI hace polling de jobs y habilita descarga del artifact.

Evidencia clave:

- Upload/deteccion/validacion: `apps/api/internal/api/handlers/upload.go:41`, `apps/api/internal/ingestion/detector.go:109`, `apps/api/internal/ingestion/validator.go:23`.
- Resolver de capacidades: `apps/api/internal/capabilities/resolver.go:12`, `apps/api/internal/capabilities/resolver.go:52`.
- Jobs y transiciones: `apps/api/internal/domain/job.go:10`, `apps/api/internal/orchestrator/service.go:160`.
- Worker y validacion de artifact: `apps/api/internal/workers/handler.go:153`, `apps/api/internal/workers/output_validation.go:42`.
- UI consume capabilities reales post-upload: `apps/web/src/components/hooks/use-upload.ts`, `apps/web/src/components/hooks/use-conversion.ts`.

## Matriz de formatos soportados

Fuente canonica: `apps/api/internal/capabilities/catalog.go`. La UI hidrata hints desde `GET /api/catalog`, pero la disponibilidad por archivo depende de `Resolve`/`IsEligible`.

### PDF

| Entrada detectada | Capacidades backend | Salida |
|---|---|---|
| `application/pdf` | `pdf-to-docx` | DOCX |
| `application/pdf` | `pdf-to-jpg`, `pdf-to-png` | JPG/PNG o ZIP multipagina |
| `application/pdf` | `pdf-to-txt` | TXT |
| `application/pdf` | `pdf-compress`, `pdf-ocr-searchable-pdf` | PDF |
| `application/pdf` | `pdf-to-html-preview` | HTML |
| `application/pdf` | `pdf-ocr-to-txt`, `pdf-ocr-to-json` | TXT/JSON |

### Imagenes

| Entrada detectada | Capacidades backend | Salida |
|---|---|---|
| JPG/PNG/WEBP/GIF/BMP/TIFF | conversion raster, PDF, OCR, thumbnails, variantes web | JPG/PNG/WEBP/AVIF/PDF/TXT/JSON |
| HEIC/HEIF | `image-heic-to-jpg/png/webp` | JPG/PNG/WEBP |
| SVG | `image-svg-to-png/webp/pdf` | PNG/WEBP/PDF |

Notas: mismas salidas target aparecen varias veces con operaciones distintas (`convert`, `compress`, `preview`, `optimize`), y la UI usa `capabilityId`, no solo formato destino. Esto evita colisiones importantes.

### Documentos, presentaciones y hojas

| Entrada detectada | Capacidades backend | Salida |
|---|---|---|
| DOC/DOCX/ODT/RTF | `doc-to-pdf`, `doc-to-txt`, `doc-to-docx`, `doc-to-html`, `docx-to-markdown` segun MIME | PDF/TXT/DOCX/HTML/MD |
| TXT | `txt-to-pdf` | PDF |
| HTML | `html-to-pdf`, `html-to-txt` | PDF/TXT |
| Markdown | `markdown-to-html/pdf/docx` | HTML/PDF/DOCX |
| PPTX/ODP | `presentation-to-pdf/jpg/png` | PDF o ZIP de imagenes |
| XLSX/ODS/CSV | `spreadsheet-to-pdf/csv/xlsx/html` segun MIME | PDF/CSV/XLSX/HTML |

### Audio y video

| Entrada detectada | Capacidades backend | Salida |
|---|---|---|
| MP3/WAV/OGG/OPUS/FLAC/AAC/M4A | conversion audio y waveform | MP3/WAV/OGG/OPUS/FLAC/AAC/M4A/PNG |
| MP4/MOV/WEBM/AVI | conversion video, GIF, extraccion audio, previews, thumbnails, waveform | MP4/WEBM/GIF/MP3/WAV/FLAC/AAC/M4A/OPUS/ZIP/JPG/PNG |

## Frontend vs backend

No encontre una inconsistencia critica donde la UI permita lanzar una conversion que el backend no soporte: el flujo real usa capabilities del backend.

Si hay diferencias de expectativa:

- `apps/web/src/config/categories.ts` contiene `acceptedMimeTypes`, `acceptedFormats` y `targetFormats` como hints pre-upload. Esto puede sugerir opciones generales que luego no aparecen para un archivo concreto.
- La categoria `documents` muestra `jpg`/`png` como target general, pero solo presentaciones (`pptx`/`odp`) tienen conversion a imagen.
- La categoria `video` muestra MP4/WebM como target; el backend permite re-encode same-format para video, asi que MP4 -> MP4 y WebM -> WebM son intencionales.
- La categoria `auto` reduce esta friccion porque permite `*/*` y deja que el backend decida.

Recomendacion de producto: cambiar copy de hints a "opciones habituales segun archivo" para no prometer una matriz universal por categoria.

## Hallazgos

### 1. PDFs validos grandes pueden fallar validacion de salida

Severidad: Importante  
Estado tras fixes: corregido en Pasada 1  
Ubicacion: `apps/api/internal/workers/output_validation.go:249`

`validatePDFOutput` lee solo `outputValidationSampleLimit` de 128 KiB y exige que ese sample contenga `%%EOF`:

- `readOutputValidationSample` limita lectura a 128 KiB (`output_validation.go:20`, `output_validation.go:290`).
- `validatePDFOutput` busca `%%EOF` dentro de ese sample (`output_validation.go:260`).

En PDFs reales medianos/grandes, el marcador EOF suele estar al final del archivo, no necesariamente en los primeros 128 KiB. Resultado: un PDF valido generado por LibreOffice, Ghostscript, OCR o Markdown puede marcarse como `failed` con "La conversion no produjo un resultado valido".

Impacto:

- DOCX/TXT/HTML/Markdown/PPTX/XLSX -> PDF pueden fallar falsamente.
- `pdf-compress` y `pdf-ocr-searchable-pdf` tambien.
- La suite actual solo prueba PDF truncado, no PDF valido con EOF fuera de 128 KiB.

Recomendacion:

- Validar header al inicio y buscar `%%EOF` en una ventana final del archivo, por ejemplo los ultimos 64 KiB.
- Agregar test con PDF valido >128 KiB donde EOF este al final.

### 2. La regla "output < 1% del input" puede romper previews y optimizaciones validas

Severidad: Importante  
Estado tras fixes: corregido en Pasada 1  
Ubicacion: `apps/api/internal/workers/output_validation.go:349`, llamada desde `apps/api/internal/workers/handler.go:157`

`validateMinimumOutputSize` rechaza outputs menores al 1% del input salvo operaciones `extract` y `compress`:

- La excepcion solo cubre `OpExtract` y `OpCompress` (`output_validation.go:359`).
- `OpOptimize`, `OpPreview` y conversiones con perdida quedan sujetas a la regla (`output_validation.go:366`).

Eso es peligroso para salidas legitimas:

- video de 500 MB -> preview MP4/WebM de 8 segundos puede ser mucho menor al 1%.
- video -> contact sheet JPG o waveform PNG casi siempre sera menor al 1%.
- imagen grande -> AVIF/WebP optimizado puede bajar por debajo del 1% en casos reales.
- presentacion grande -> ZIP de thumbnails podria ser menor al 1%.

Impacto:

- Conversiones tecnicamente exitosas se reportan como fallidas.
- La UX mostrara error de conversion aun cuando el artifact exista y sea valido.

Recomendacion:

- Aplicar la regla por capability/familia, no global.
- Eximir `OpPreview` y `OpOptimize`, o reemplazar por validadores semanticos por formato: dimensiones, duracion, cantidad de frames/entradas ZIP, MIME y legibilidad.
- Agregar tests para preview de video e imagen optimizada con input grande y output chico.

### 3. `MaxRetries` esta declarado, expuesto y documentado, pero no gobierna las conversiones

Severidad: Importante  
Estado tras fixes: corregido en Pasada 2  
Ubicacion: `apps/api/internal/domain/capability.go:19`, `apps/api/internal/orchestrator/service.go:170`, `apps/api/internal/orchestrator/service.go:268`

Cada capability declara `ExecutionLimits.MaxRetries`, y `GET /api/catalog` lo expone (`apps/api/internal/api/handlers/capabilities.go:206`). Sin embargo:

- `CreateAndEnqueue` usa `queue.TaskOptions{MaxRetries: 0}`.
- `CreateAndEnqueueBatch` tambien usa `MaxRetries: 0`.
- El test `TestCreateAndEnqueueDisablesQueueAutoRetries` espera explicitamente ese comportamiento.
- El retry manual crea un job nuevo (`service.go:347`) y no hay `retry_count`, `source_job_id` persistido como limite operativo ni enforcement del maximo por capability.

Esto contradice el modelo indicado por `AGENTS.md`: "Solo jobs en estado failed pueden ser reintentados" y "los jobs tienen un limite de reintentos configurado por capacidad".

Impacto:

- El catalogo promete un limite que no se aplica.
- Un usuario/admin puede encadenar retries manuales sin limite de intentos salvo cuotas activas/rate limits.
- Observabilidad de tasa de retry por job raiz queda incompleta.

Recomendacion:

- Decidir producto: retries automaticos de cola o retries manuales limitados.
- Si se mantienen manuales, persistir `source_job_id`/`attempt_number` y rechazar retries cuando `attempt_number > cap.ExecutionLimits.MaxRetries`.
- Si se usan retries automaticos, pasar `cap.ExecutionLimits.MaxRetries` a `TaskOptions` y revisar idempotencia de persistencia de artifacts.
- No exponer `MaxRetries` en catalogo si no es contrato real.

### 4. Export HTML de LibreOffice puede perder assets companion

Severidad: Importante  
Estado tras fixes: corregido en Pasada 3 para capacidades finales `doc-to-html` y `spreadsheet-to-html`  
Ubicacion: `apps/api/internal/workers/document/to_html.go:24`, `apps/api/internal/workers/document/to_html.go:28`

`ToHTMLEngine` llama `libreoffice --convert-to html`, espera `base.html`, sanitiza ese archivo y devuelve solo esa ruta. LibreOffice suele generar recursos companion para documentos ricos, por ejemplo imagenes o carpetas auxiliares.

Impacto:

- DOCX/XLSX/ODS con imagenes o formato rico pueden descargar HTML con referencias rotas.
- La validacion `html` solo revisa texto/UTF-8 y tamano minimo, no que los recursos referenciados existan.

Recomendacion:

- Si hay assets companion, empaquetar HTML + recursos en ZIP.
- Alternativamente, inlinear imagenes como data URI despues de sanitizar y bloquear referencias remotas.
- Agregar fixture DOCX/XLSX con imagen y test que confirme que la descarga conserva recursos o documenta explicitamente que se descartan.

### 5. Disponibilidad de engines puede divergir entre API y worker

Severidad: Medio  
Estado tras fixes: mitigado en Pasada 4 con health efectivo por worker y modo de engines API  
Ubicacion: `apps/api/cmd/server/main.go:41`, `apps/api/internal/capabilities/engines.go:37`, `apps/api/internal/capabilities/resolver.go:38`

En modo Redis, el server usa `NewDeclaredEngineProber`, que marca engines conocidos como disponibles sin probar binarios locales. Esto tiene sentido si el worker standalone es quien convierte, pero crea un contrato operativo fuerte: API y worker deben tener capacidades efectivas compatibles.

Impacto:

- API puede mostrar `ffmpeg`, `libreoffice` o `tesseract` aunque el worker desplegado no los tenga.
- El fallo se desplaza de resolucion de capacidades a ejecucion del job.

Recomendacion:

- Mantener el runbook actual, pero agregar health efectivo por worker: engines instalados, versiones y capabilities ejecutables.
- En `GET /api/catalog` o admin health, diferenciar "declarado" vs "ejecutable por worker".

### 6. Ingestion depende de binarios externos antes de que la capacidad sea elegible

Severidad: Medio  
Estado tras fixes: documentado en Pasada 4; comportamiento mantenido por seguridad  
Ubicacion: `apps/api/internal/ingestion/metadata.go:48`, `apps/api/internal/ingestion/metadata.go:92`, `apps/api/internal/ingestion/metadata.go:125`

La subida de PDF usa `pdfinfo`; audio/video e imagenes no soportadas por `image.DecodeConfig` usan `ffprobe`. Si esos binarios faltan, un upload puede fallar antes de llegar a resolver capacidades.

Impacto:

- Un entorno que quiera aceptar solo capacidades pure-Go igual puede rechazar ciertos uploads por metadata.
- En modo Redis con API "liviana", no basta con que el worker tenga motores; la API tambien necesita algunos binarios de inspeccion.

Recomendacion:

- Documentar explicitamente "binarios de ingestion requeridos por familia".
- Separar engine availability de ingestion availability.
- Considerar degradacion segura para metadata opcional solo cuando la policy lo permita.

## Seguridad y validacion

Fortalezas observadas:

- No se confia en extension para decisiones funcionales (`DetectFormat` usa contenido).
- Upload se stream-ea a temp file y no carga archivos grandes en memoria.
- Hay limites por familia: PDF/imagen/documento 100 MB, audio 250 MB, video 500 MB.
- Hay limite de pixeles, paginas y duracion.
- ZIP input tiene controles de entradas, tamano descomprimido y ratio.
- `OriginalFile.InternalName` usa UUID; `OriginalName` se sanitiza.
- Descarga valida ownership por archivo original antes de abrir artifact.
- Artifact path usa `artifactId` + `fileName` validado, no rutas arbitrarias del cliente.
- HTML/SVG tienen sanitizacion contra scripts y referencias remotas.

Riesgos residuales:

- No observe sandbox duro para motores externos: se usan `exec.CommandContext` con timeout, pero no limites OS de memoria/CPU/red/filesystem por proceso.
- No observe escaneo malware.
- Algunos mensajes al usuario siguen siendo genericos en worker failure; internamente hay logs, pero producto podria clasificar mas fino.

## Estados y UX

Estados backend: `queued`, `running`, `succeeded`, `failed`, `cancelled`, `expired`.  
Estados UI por item: `uploading`, `selected`, `converting`, `done`, `error`.

La UI maneja todos los terminales backend:

- `succeeded` -> `done`
- `failed` -> `error`
- `cancelled` -> vuelve a `selected`
- `expired` -> `error`

Puntos a mejorar:

- Mostrar reintento al usuario final en el flujo principal cuando un job falla, no solo en dashboard.
- Mostrar causa diferenciada para timeout, protegido, output invalido y motor no disponible cuando el backend ya la clasifica.
- Evitar copy pre-upload que parezca promesa cerrada de formatos.

## Cobertura de tests

Cobertura fuerte observada:

- Resolucion de capabilities por familia, engine, feature flags, protected/oversized/same-format.
- Detector con Markdown, SVG, OOXML complejo, DOC legacy, zip bomb y corruptos.
- Validacion de output para JSON, CSV, text, MIME binario, ZIP traversal, OOXML, PDF truncado.
- Worker cancelation.
- E2E API para upload/capabilities/conversion/job/cancel/retry/ownership/download.
- Tests reales de varios engines condicionados por disponibilidad.

Brechas recomendadas:

1. Completado: PDF valido >128 KiB con `%%EOF` al final debe pasar validacion.
2. Completado: preview/optimize con output <1% del input debe pasar si el formato es valido.
3. Completado: retry manual debe respetar `MaxRetries` o el catalogo debe dejar de exponerlo.
4. Completado: LibreOffice HTML con assets debe preservar recursos o convertirlos en ZIP.
5. Completado manualmente: smoke Docker por familia critica en local/Compose para PDF->TXT, DOCX->PDF, XLSX->CSV, PPTX->JPG ZIP, PNG->WEBP, HEIF->PNG, SVG->PDF, WAV->MP3 y MP4->GIF. Falta llevarlo a CI/release gate.
6. Completado: test de API/worker divergence y visibilidad administrativa de catalogo declarado en API vs engines reales por worker.
7. Pendiente: E2E frontend para artifact expirado y retry visible al usuario.

## Mejoras de producto y nuevas funcionalidades

Prioridad alta:

- Historial con retry desde la pantalla principal, no solo dashboard.
- Explicacion de "por que no hay conversiones disponibles" por archivo: engine faltante, limite, protegido, formato no soportado.
- Preflight de runtime: admin ve engines por worker, version de binarios y fallos recientes por capability.
- Smoke de conversion real como release gate.

Prioridad media:

- PDF split/extract pages.
- Remover metadata/EXIF en imagenes.
- PDF optimize for web.
- CSV/XLSX preview antes de convertir.
- Transcripcion audio/video a TXT/SRT/VTT si el producto acepta dependencia STT.

Prioridad baja:

- Estimacion de tiempo por capability basada en historial.
- Webhooks por job terminal para usuarios finales.
- Comparacion de tamano antes/despues en UI.

## Checklist de cierre

- La logica de capacidades esta centralizada en backend.
- No encontre decision funcional basada solo en extension.
- Hay validacion de input, output, ownership y retencion.
- Hay inconsistencias importantes en retries y validacion de outputs.
- Tests focalizados pasan, pero faltan pruebas para los falsos negativos detectados.
- La prioridad tecnica mas rentable es corregir `validatePDFOutput` y la regla global del 1% antes de ampliar el catalogo.
