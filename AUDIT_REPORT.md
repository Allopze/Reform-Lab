# Auditoria tecnica y de producto - Reform Lab

Fecha: 2026-05-05

## Resumen ejecutivo

Reform Lab tiene una base tecnica bastante solida para un SaaS de conversion de archivos: separa frontend, API, dominio, storage, workers y observabilidad; detecta formatos por contenido real; centraliza capacidades en backend; usa jobs asincronos; y aplica controles relevantes como CSRF, CORS por allowlist, nombres internos UUID, limites por tipo de usuario y rutas protegidas por propietario o sesion anonima.

Los riesgos principales encontrados no son de estructura general, sino de consistencia entre politica declarada, tests y comportamiento real. Tras la primera pasada de fixes, quedaron corregidos tres puntos backend importantes: los reintentos automaticos de conversion ya no chocan con el modelo cerrado de estados, los parsers de PDF/audio/video ya clasifican corrupcion cuando fallan, y los E2E admin jobs usan el contrato API actual. El primer registro como admin se mantiene como feature de producto por decision explicita; la recomendacion restante es alinear documentacion, copy operativo y checklist de despliegue para que esa feature no sea una sorpresa en produccion.

## Alcance revisado

- Frontend: `apps/web` con Next.js 15, React 19, Tailwind CSS 4, Vitest y Playwright.
- Backend/API: `apps/api` con Go 1.25, Chi, SQLite, JWT, Asynq/Redis e in-process queue.
- Dominio: formatos detectados, capacidades, jobs, artefactos, cuotas y ownership.
- Seguridad: autenticacion, autorizacion, upload, storage, CORS, CSRF, webhooks, secrets y Docker.
- Calidad: tests unitarios/integracion, lint, vet, auditoria de dependencias npm.
- Documentacion: `README.md`, `AGENTS.md`, arquitectura, glosario, seguridad, testing y ADRs.

## Hallazgos priorizados

| Estado | Severidad | Area | Hallazgo | Recomendacion corta |
| --- | --- | --- | --- | --- |
| Completado | Media | Auth/Admin | El primer registro como admin es feature, pero README y checklist operativo debian decirlo sin ambiguedad. | README y env examples documentan que el primer registro reclama admin si no hay allowlist. |
| Completado | Alta | Jobs/Workers | Los reintentos automaticos de Asynq chocaban con el modelo cerrado de estados del dominio. | Conversiones encolan con `MaxRetries: 0`; el retry queda como accion explicita del orquestador. |
| Completado | Alta | Upload/Validacion | PDFs/audio/video corruptos podian pasar validacion si la extraccion de metadata fallaba silenciosamente. | `pdfinfo`/`ffprobe` ahora devuelven `ErrInvalidCorrupted` ante parser failure. |
| Completado | Media | Tests/API | Tests E2E de admin jobs estaban obsoletos y se saltaban cobertura critica. | Tests usan `/api/files/{fileId}/capabilities` y `/api/conversions`. |
| Completado | Media | Admin config | Escrituras de settings no atomicas podian dejar configuracion parcial tras un error. | `site_settings` tiene escritura batch transaccional; SMTP y upload policy la usan. |
| Completado | Media | Storage/Upload | El archivo temporal se escribia antes de comprobar presion de disco y cuota acumulada. | Upload ahora prechequea headroom de disco y corta staging al superar cuota restante. |
| Completado | Baja | Frontend security | Preview HTML de templates usaba iframe same-origin innecesariamente. | Preview usa `srcDoc`, `sandbox=""` y `referrerPolicy="no-referrer"`. |
| Completado | Baja | API/UX | Errores API devolvian solo strings sin codigo estable. | Respuestas mantienen `error` string y agregan `code`, `message`, `requestId`, `retryable`. |
| Completado | Baja | Producto/UX | Faltaba copy visible de privacidad, retencion y limites cerca de la subida. | Dropzone muestra limites/cupo reales y texto de deteccion real, no ejecucion y retencion. |
| Completado | Baja | Tooling | `next lint` estaba deprecado para Next.js 16. | `npm run lint` usa ESLint CLI con ignores explicitos para outputs generados. |

## Hallazgos detallados

### 1. Bootstrap del primer admin

**Estado:** Completado en la pasada 2  
**Severidad:** Media  
**Archivos:** `apps/api/internal/auth/service.go`, `apps/api/internal/auth/service_test.go`, `README.md`

El producto mantiene como feature que el primer registro pueda reclamar admin. Por decision explicita, no se modifico la logica de `auth.Service`.

Riesgo residual:

- Si una instancia de produccion queda expuesta antes de que el operador cree la primera cuenta, un tercero podria reclamar el admin inicial.
- La documentacion anterior podia interpretarse como que `APP_ENV=production` bloqueaba siempre el auto-admin.

Recomendacion:

- Implementado: `README.md` ya no afirma que `APP_ENV=production` bloquea el auto-admin del primer registro.
- Implementado: `.env.example` y `.env.production.example` explican que, si `BOOTSTRAP_ADMIN_EMAILS` queda vacio, la primera cuenta registrada se convierte en admin inicial.
- Implementado: `.env.production.example` recomienda crear ese admin antes de exponer la app publicamente o restringir la allowlist.

Decision de esta pasada: no tocar la logica de `auth.Service`; solo alinear documentacion y plantillas de entorno.

### 2. Reintentos automaticos de Asynq inconsistentes con estados cerrados

**Severidad:** Alta  
**Archivos:** `apps/api/internal/queue/asynq.go`, `apps/api/internal/workers/handler.go`, `apps/api/internal/orchestrator/service.go`, `apps/api/internal/domain/job.go`  
**Estado:** Completado en esta pasada.

El dominio define transiciones cerradas para jobs. Un job pasa de `queued` a `running`, y de `running` a `succeeded`, `failed` o `cancelled`. El retry documentado es `failed -> queued`, pero debe ocurrir como transicion explicita. Asynq, en cambio, reintenta automaticamente la misma tarea cuando el handler devuelve error.

Evidencia:

- `apps/api/internal/queue/asynq.go` encola jobs con `asynq.MaxRetry(opts.MaxRetries)`.
- `apps/api/internal/workers/handler.go` marca el job como `failed` y devuelve error ante fallo.
- En el siguiente intento automatico, el worker intentara `MarkRunning` sobre un job ya `failed`.
- `apps/api/internal/orchestrator/service.go` valida transiciones y rechazara `failed -> running`.

Impacto:

Los retries automaticos pueden consumir intentos sin poder ejecutar conversion real, producir logs ruidosos y romper la semantica de observabilidad. El usuario vera un job fallido aunque la cola siga reintentando una tarea que ya no puede transicionar.

Recomendacion:

- Implementado: las conversiones se encolan con `MaxRetries: 0` desde `orchestrator.Service`.
- Se preservan retries de email/webhook porque usan otros metodos de cola.
- Test agregado: `TestCreateAndEnqueueDisablesQueueAutoRetries`.

### 3. Archivos corruptos pueden pasar upload si metadata falla silenciosamente

**Severidad:** Alta  
**Archivos:** `apps/api/internal/ingestion/metadata.go`, `apps/api/internal/ingestion/validator.go`  
**Estado:** Completado en esta pasada.

La deteccion por magic bytes es una buena base, pero para PDF/audio/video no alcanza para validar integridad. Hoy algunos errores de metadata se convierten en metadata vacia sin error.

Evidencia:

- `apps/api/internal/ingestion/metadata.go`: los errores de `pdfinfo` que no sean timeout/cancelacion devuelven metadata vacia sin error.
- `apps/api/internal/ingestion/metadata.go`: los errores de `ffprobe` que no sean timeout/cancelacion devuelven metadata vacia sin error.
- `apps/api/internal/ingestion/validator.go`: si no hay paginas, duracion o dimensiones, muchas validaciones simplemente no se aplican.

Impacto:

Un archivo con cabecera compatible pero corrupto puede ser aceptado, almacenado y enviado a conversion, fallando mas tarde con un error menos claro. Esto contradice el objetivo documentado de validar archivos corruptos temprano y aumenta carga de workers/storage.

Recomendacion:

- Implementado: `pdfinfo` y `ffprobe` ahora devuelven `domain.ErrInvalidCorrupted` cuando el parser sale con error.
- Implementado: JSON invalido de `ffprobe` tambien se clasifica como corrupcion.
- Test agregado: `TestExtractMetadataRejectsCorruptedParserBackedFixtures` con fixtures PDF/WAV/MP4 truncados.

### 4. Tests E2E de admin jobs estan obsoletos y saltan cobertura

**Severidad:** Media  
**Archivos:** `apps/api/internal/api/api_e2e_test.go`, `apps/api/internal/api/router.go`, `apps/api/internal/api/handlers/upload.go`  
**Estado:** Completado en esta pasada.

Algunos tests E2E esperan un contrato antiguo: que la respuesta de upload incluya `capabilities` y que la conversion se cree en `/api/files/{fileId}/convert`. El router actual expone `/api/files/{fileId}/capabilities` y `/api/conversions`.

Evidencia:

- `api_e2e_test.go` lee `uploadResponse["capabilities"]` y hace `t.Skip` si esta vacio.
- El upload handler devuelve principalmente el `record`, no el listado de capacidades.
- El test usa una ruta de conversion que ya no existe.
- `go test ./internal/...` pasa, pero esa parte critica queda omitida.

Impacto:

La suite da una senal de confianza falsa sobre listado admin de jobs, detalle, retry/cancel y permisos asociados.

Recomendacion:

- Implementado: helper E2E `firstCapabilityForFile`.
- Implementado: `TestE2E_AdminJobsList`, `TestE2E_AdminJobsBatchActions` y `TestE2E_AdminSupportControls` usan `/api/files/{fileId}/capabilities` y `/api/conversions`.
- Verificado con test focalizado de esos E2E.

### 5. Escrituras de settings admin no atomicas

**Severidad:** Media  
**Archivos:** `apps/api/internal/api/handlers/smtp_settings.go`, `apps/api/internal/api/handlers/upload_policy.go`  
**Estado:** Completado en la pasada 2.

Algunas rutas admin escriben varios settings en pasos separados. Si un paso intermedio falla, la API puede devolver error despues de haber persistido cambios parciales.

Evidencia:

- En settings SMTP se escriben host, port y user antes de cifrar password. Si falta `SECRET_ENCRYPTION_KEY`, la ruta devuelve `503` pero ya cambio parte de la configuracion.
- En upload policy se guardan limites guest y registered con llamadas independientes.

Impacto:

Un admin puede recibir una respuesta de error y aun asi dejar el sistema en un estado parcialmente modificado, dificil de diagnosticar.

Recomendacion:

- Implementado para SMTP password: la ruta valida/cifra el password antes de persistir host, port o user.
- Test agregado: `TestSMTPSettingsUpdateValidatesSecretStorageBeforePersisting`.
- Implementado: `repository.SiteSettingRepository.UpsertValues` guarda grupos de settings dentro de una transaccion SQLite.
- Implementado: SMTP settings y upload policy usan `UpsertValues`.
- Test agregado: `TestSiteSettingRepoUpsertValuesPersistsBatch`.
- Verificado con E2E focalizados de SMTP y upload policy.

### 6. Staging temporal ocurre antes de controles de disco/cuota acumulada

**Severidad:** Media  
**Archivos:** `apps/api/internal/api/handlers/upload.go`, `apps/api/internal/storage/filesystem.go`  
**Estado:** Completado en la pasada 3.

La subida limita tamano por request, pero el archivo se escribe primero a un directorio temporal. La cuota acumulada y parte de la verificacion de espacio libre ocurren despues.

Evidencia:

- `upload.go` crea directorio temporal y copia el body multipart a disco.
- La cuota acumulada se verifica despues de copiar.
- La comprobacion de espacio libre esta en `SaveOriginal`, cuando el temporal ya existe.

Impacto:

Con cargas concurrentes o abuso distribuido se puede presionar el volumen temporal antes de que los controles de storage final se activen. El riesgo esta mitigado por limites por request y rate limiting, pero sigue siendo una brecha operativa.

Recomendacion:

- Implementado: `UploadHandler` consulta `DiskStats` antes de leer el multipart cuando el store lo soporta.
- Implementado: se exige headroom para staging temporal y copia final, preservando el umbral minimo de espacio libre de storage.
- Implementado: se calcula cuota restante antes de crear el temporal; si esta agotada, se rechaza antes de copiar.
- Implementado: el copy al temporal usa un limite basado en `min(limite por archivo, cuota restante)` y aborta en cuanto se excede.
- Test agregado: cobertura de cuota restante, limite de staging y headroom de disco en `upload_quota_test.go`.

### 7. Preview HTML de emails usa iframe same-origin

**Severidad:** Baja  
**Archivos:** `apps/web/src/components/email-templates.tsx`  
**Estado:** Completado en la pasada 3.

La UI admin escribe HTML de preview dentro de un iframe con `sandbox="allow-same-origin"`. No se permite `allow-scripts`, lo cual reduce el riesgo, pero `allow-same-origin` elimina aislamiento que no parece necesario para un preview.

Impacto:

Hoy el riesgo es bajo porque es una funcion admin y scripts no deberian ejecutarse. Pero si en el futuro se relajan flags o se previsualiza HTML externo, el patron aumenta superficie XSS.

Recomendacion:

- Implementado: se elimino `doc.write`.
- Implementado: el iframe usa `srcDoc={previewHtml ?? ""}`.
- Implementado: el sandbox queda sin `allow-same-origin` y con `referrerPolicy="no-referrer"`.
- Verificado con tests focalizados de `email-templates` y lint frontend.

### 8. Errores API sin codigo estable

**Severidad:** Baja  
**Archivos:** `apps/api/internal/api/handlers/health.go`, consumidores en `apps/web/src/lib/api.ts`
**Estado:** Completado en la pasada 4.

Las respuestas de error tienen forma simple `{"error": "mensaje"}`. Es legible, pero fragil para UX, internacionalizacion y manejo diferenciado de errores.

Impacto:

El frontend no puede distinguir de forma robusta entre cuota excedida, formato no soportado, archivo protegido, sesion expirada, job expirado o fallo transitorio sin parsear strings.

Recomendacion:

- Implementado: `respondError` mantiene `error` como string para compatibilidad.
- Implementado: cada error JSON incluye `code`, `message`, `requestId` cuando existe y `retryable`.
- Test agregado: `TestRespondErrorIncludesStableEnvelope`.

### 9. Copy visible de privacidad, retencion y limites

**Severidad:** Baja  
**Archivos:** `apps/web/src/components/conversion-card.tsx`, `apps/web/messages/es.json`  
**Estado:** Completado en la pasada 4.

La UI ya mostraba limites por archivo y cuota, pero la informacion de privacidad/retencion no estaba tan cerca del punto de decision de subida.

Recomendacion:

- Implementado: el detalle del dropzone combina limites/cupo reales del backend con un mensaje visible sobre deteccion real de tipo, no ejecucion del archivo y eliminacion por politica de retencion.
- Verificado con tests focalizados de `conversion-card`.

### 10. Lint frontend usa comando deprecado

**Severidad:** Baja  
**Archivo:** `apps/web/package.json`, `apps/web/eslint.config.mjs`  
**Estado:** Completado en la pasada 4.

`npm run lint` funciona, pero Next muestra advertencia de deprecacion: `next lint` sera removido en Next.js 16.

Recomendacion:

- Implementado: `npm run lint` ejecuta `eslint .`.
- Implementado: `eslint.config.mjs` ignora `.next`, `node_modules`, `test-results`, `playwright-report` y `next-env.d.ts`.
- Verificado: `npm run lint` pasa sin advertencia de deprecacion.

## Observaciones positivas

- La deteccion de tipo real usa lectura de contenido y no confia solo en extension.
- La resolucion de capacidades vive en backend y evita duplicar reglas finales en frontend.
- Los artefactos y originales se almacenan con nombres internos no derivados del input del usuario.
- Hay ownership consistente por usuario autenticado o guest session.
- La ruta de descarga de artefactos valida permisos antes de servir.
- CORS, CSRF y cookies `SameSite` estan considerados.
- Webhooks incluyen defensas SSRF: bloqueo de hosts privados/reservados, proxy deshabilitado y dialer restringido.
- Docker Compose aplica controles razonables: usuarios no root, `read_only`, `tmpfs`, `cap_drop` y `no-new-privileges`.
- Hay tests amplios en backend y frontend; las suites principales pasan.

## Producto y UX

- La UI ahora muestra limites/cupo y politica de privacidad/retencion cerca del upload. El dashboard puede seguir profundizando este copy con detalles operativos si el producto lo necesita.
- Si el roadmap incluye monetizacion, hoy no se ve un sistema completo de planes o billing; las cuotas son principalmente configuracion/env/admin policy. Esto puede ser suficiente para MVP, pero no para planes comerciales.
- Los usuarios anonimos pueden convertir, pero deberian entender claramente que el historial depende de la sesion/cookie y puede perderse.
- Falta una senal fuerte de confianza para archivos sensibles: tiempo de eliminacion, si se procesan aislados, si se escanean, y que no se usan para entrenamiento u otros fines.
- La documentacion de seguridad menciona antivirus/sandbox como controles complementarios. Para produccion publica, deberia priorizarse al menos escaneo asincrono o cuarentena por tipo de archivo de alto riesgo.

## Verificacion realizada

Comandos ejecutados:

```bash
cd apps/api && go test ./internal/... -count=1 -timeout=180s
cd apps/api && go vet ./...
cd apps/web && npm test
cd apps/web && npm run lint
npm audit --omit=dev --json
cd apps/web && npm audit --omit=dev --json
cd apps/api && go test ./internal/orchestrator ./internal/ingestion -count=1
cd apps/api && go test ./internal/api -run 'TestE2E_AdminJobsList|TestE2E_AdminJobsBatchActions|TestE2E_AdminSupportControls' -count=1 -timeout=300s
cd apps/api && go test ./internal/api/handlers ./internal/api -run 'TestSMTPSettingsUpdateValidatesSecretStorageBeforePersisting|TestE2E_AdminSMTPSettings' -count=1 -timeout=300s
cd apps/api && go test ./internal/repository ./internal/api/handlers ./internal/email -count=1
cd apps/api && go test ./internal/api -run 'TestE2E_AdminSMTPSettings|TestE2E_AdminSupportControls' -count=1 -timeout=300s
cd apps/api && go test ./internal/api -run 'TestE2E_AdminCanUpdateUploadPolicy|TestE2E_UploadPolicyAppliesDifferentLimitsForGuestsAndRegisteredUsers' -count=1 -timeout=300s
cd apps/api && go test ./internal/api/handlers ./internal/storage -count=1
cd apps/api && go test ./internal/api -run 'TestE2E_Upload|TestE2E_AdminCanUpdateUploadPolicy|TestE2E_UploadPolicyAppliesDifferentLimitsForGuestsAndRegisteredUsers|TestE2E_AnonymousUpload' -count=1 -timeout=300s
cd apps/web && npm test -- email-templates
cd apps/web && npm run lint
cd apps/api && go test ./internal/api/handlers -count=1
cd apps/web && npm test -- conversion-card
```

Resultados:

- Backend tests: pasan.
- `go vet`: pasa.
- Frontend tests: pasan, 90 tests en 18 archivos.
- Frontend lint: pasa. En la auditoria inicial pasaba con advertencia de `next lint`; tras la pasada 4 usa ESLint CLI sin esa advertencia.
- npm audit root y frontend: sin vulnerabilidades reportadas en dependencias de produccion.
- Tests focalizados de la pasada 1: pasan.
- Tras los fixes de la pasada 1, `go test ./internal/... -count=1 -timeout=180s` y `go vet ./...` volvieron a pasar.
- Tests focalizados de la pasada 2: pasan.
- Tras los fixes de la pasada 2, `go test ./internal/... -count=1 -timeout=180s` y `go vet ./...` volvieron a pasar.
- Tests focalizados de la pasada 3: pasan.
- Tras los fixes de la pasada 3, `go test ./internal/... -count=1 -timeout=180s` y `go vet ./...` volvieron a pasar.
- Tests focalizados frontend de la pasada 3 y lint: pasan.
- Tras tocar frontend en la pasada 3, `npm test` completo volvio a pasar.
- Tests focalizados de la pasada 4 para errores API, conversion-card y lint: pasan.
- Tras los fixes de la pasada 4, `go test ./internal/... -count=1 -timeout=180s`, `go vet ./...`, `npm test` y `npm run lint` volvieron a pasar.

Nota: tambien se ejecuto inicialmente `npm test -- --runInBand` en frontend; fallo porque `--runInBand` es una bandera de Jest, no de Vitest. Se corrigio ejecutando `npm test`.

## Plan de accion recomendado

No quedan hallazgos abiertos del alcance de esta auditoria inicial. Siguientes mejoras naturales:

1. Documentar formalmente el nuevo contrato de error en `docs/` para consumidores externos.
2. Evaluar antivirus/sandbox como control complementario para formatos de mayor riesgo.
3. Ejecutar Playwright E2E completo antes de release.

## Conclusion

El repositorio esta bien orientado: las fronteras de capas son claras, la seguridad de archivos esta presente en el diseno, y las pruebas basicas pasan. La primera pasada cerro los riesgos mas directos de jobs, corrupcion de archivos y E2E obsoletos. La segunda pasada dejo documentado el bootstrap del primer admin como feature y cerro la atomicidad de settings admin. La tercera pasada redujo el riesgo de staging temporal bajo presion de disco/cuota y endurecio el preview HTML admin. La cuarta pasada normalizo errores API, hizo mas visible privacidad/retencion/limites en upload y migro lint a ESLint CLI.
