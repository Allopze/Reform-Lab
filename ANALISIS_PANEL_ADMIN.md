# Analisis del panel de administracion actual

## Alcance y criterio

- Analisis hecho directamente sobre el codigo del repositorio, no sobre supuestos de producto.
- Se revisaron frontend admin, backend/API admin, modelos, auth/roles, jobs/queues/workers, observabilidad y tests.
- Clasificacion usada en este documento:
  - Implementado y operativo
  - Implementado parcialmente
  - Esqueleto o placeholder
  - No implementado
- Fuera de alcance por pedido explicito: billing, pagos, suscripciones, planes y facturacion.

## Resumen ejecutivo

El panel admin actual existe y funciona, y ya dejo de ser solo una consola unica de resumen operativo y configuracion: ahora tambien tiene modulos dedicados para jobs y usuarios. Aun asi, no es un backoffice completo para operar toda la plataforma.

Lo mejor resuelto hoy esta en tres bloques:

1. Resumen operativo basico con metricas globales, jobs recientes y feed de auditoria.
2. Configuracion operativa puntual: mensaje de footer y limites de subida.
3. Comunicaciones salientes: SMTP, plantillas de email y webhooks.

Las brechas mas importantes ya no estan en jobs basicos, usuarios basicos o auditoria de cambios admin, sino en capas mas finas de soporte y gobernanza:

1. `/admin/system` ya muestra backlog actual, historico 5m/15m/1h y estado fino por worker con heartbeat, ultima tarea y fallas recientes, y ahora tambien permite controles activos de soporte (pausar/reanudar intake, drenar cola y limpiar workers obsoletos).
2. La vista de jobs ya permite cancelar/reintentar por fila, detectar atascos y ejecutar acciones masivas por seleccion o por filtro, pero aun falta trazabilidad tecnica mas profunda de ejecucion.
3. La gestion de usuarios ya cubre roles, suspension/reactivacion y revocacion de sesiones, pero siguen faltando invitaciones, borrado y herramientas de soporte mas avanzadas.
4. La salud del sistema ya cubre runtime, dependencias, workers e historicos operativos minimos; el siguiente salto real seria integrarla mas profundamente con observabilidad y diagnostico externo.

Mi conclusion es que el sistema ya tiene base suficiente para una `version minima razonable` de admin, pero esa version deberia priorizar operacion de jobs, usuarios basicos, visibilidad de salud y auditoria de cambios admin antes de sumar mas configuraciones.

## Inventario actual del panel admin

| Area | Estado | Lo que existe hoy | Limites o gaps | Evidencia |
| --- | --- | --- | --- | --- |
| Ruta y shell admin | Implementado y operativo | Existen `/admin`, `/admin/jobs`, `/admin/users`, `/admin/system` y `/admin/audit`. El header solo muestra el enlace admin para usuarios con rol admin y el dashboard enlaza a jobs, usuarios, sistema y auditoria. | Aun no hay submodulos dedicados para otros dominios (por ejemplo webhooks) fuera del dashboard principal. | [apps/web/src/app/admin/page.tsx](apps/web/src/app/admin/page.tsx), [apps/web/src/app/admin/jobs/page.tsx](apps/web/src/app/admin/jobs/page.tsx), [apps/web/src/app/admin/users/page.tsx](apps/web/src/app/admin/users/page.tsx), [apps/web/src/app/admin/system/page.tsx](apps/web/src/app/admin/system/page.tsx), [apps/web/src/app/admin/audit/page.tsx](apps/web/src/app/admin/audit/page.tsx), [apps/web/src/components/header.tsx](apps/web/src/components/header.tsx), [apps/web/src/components/admin-dashboard.tsx](apps/web/src/components/admin-dashboard.tsx) |
| Proteccion de acceso | Implementado y operativo | El API protege las rutas admin con autenticacion y `RequireAdmin`. El middleware de Next para `/admin` valida JWT y rol admin antes de servir la shell. | Sigue dependiendo de la cookie de sesion existente; no hay endurecimiento adicional tipo step-up auth o 2FA. | [apps/api/internal/api/middleware/auth.go](apps/api/internal/api/middleware/auth.go), [apps/api/internal/api/router.go](apps/api/internal/api/router.go), [apps/web/middleware.ts](apps/web/middleware.ts) |
| Resumen general admin | Implementado y operativo | El overview muestra total de usuarios, archivos, jobs, jobs por estado, success rate, duracion media, jobs recientes, auditoria reciente y disponibilidad agregada de engines. | Es un resumen agregado global. No hay cortes temporales, tendencias, storage, backlog real, alertas activas ni actividad por usuario/equipo. | [apps/api/internal/repository/dashboard_repo.go](apps/api/internal/repository/dashboard_repo.go), [apps/api/internal/api/handlers/dashboard.go](apps/api/internal/api/handlers/dashboard.go), [apps/web/src/components/admin-dashboard.tsx](apps/web/src/components/admin-dashboard.tsx) |
| Jobs recientes | Implementado parcialmente | El dashboard enseña una tabla de jobs recientes a nivel global con `capabilityId` y error visible, y enlaza al listado completo. | Sigue siendo una vista corta: no hay acciones inline ni detalle expandido desde el dashboard. | [apps/api/internal/repository/dashboard_repo.go](apps/api/internal/repository/dashboard_repo.go), [apps/web/src/components/admin-dashboard.tsx](apps/web/src/components/admin-dashboard.tsx) |
| Operacion de jobs por admin | Implementado parcialmente | Existe `GET /api/admin/jobs` con filtros, busqueda y paginacion, una UI dedicada en `/admin/jobs` con `cancel/retry`, deteccion de atascos y acciones masivas admin (`/api/admin/jobs/batch/cancel`, `/api/admin/jobs/batch/retry`) por seleccion o por filtro. | Aun faltan detalle tecnico profundo, trazas y diagnostico operativo mas fino por intento/worker. | [apps/api/internal/api/handlers/admin_jobs.go](apps/api/internal/api/handlers/admin_jobs.go), [apps/api/internal/repository/job_repo.go](apps/api/internal/repository/job_repo.go), [apps/web/src/app/admin/jobs/page.tsx](apps/web/src/app/admin/jobs/page.tsx), [apps/web/src/components/admin-jobs-table.tsx](apps/web/src/components/admin-jobs-table.tsx), [apps/web/src/lib/api.ts](apps/web/src/lib/api.ts) |
| Gestion de usuarios | Implementado parcialmente | Existe `GET /api/admin/users` con busqueda, filtro por rol y paginacion, `PATCH /api/admin/users/{userId}/role`, suspension/reactivacion (`PATCH /api/admin/users/{userId}/suspension`), revocacion de sesiones (`POST /api/admin/users/{userId}/revoke-sessions`) y UI dedicada en `/admin/users`. | Siguen faltando invitaciones, borrado y otras operaciones de soporte mas avanzadas. | [apps/api/internal/api/handlers/admin_users.go](apps/api/internal/api/handlers/admin_users.go), [apps/api/internal/repository/user_repo.go](apps/api/internal/repository/user_repo.go), [apps/api/internal/api/router.go](apps/api/internal/api/router.go), [apps/web/src/app/admin/users/page.tsx](apps/web/src/app/admin/users/page.tsx), [apps/web/src/components/admin-users-table.tsx](apps/web/src/components/admin-users-table.tsx) |
| Roles y permisos | Implementado parcialmente | Hay un rol binario `admin`/`user`, `RequireAdmin` en backend, promote/demote auditable y ya no existe la restriccion de admin unico. | No existe RBAC granular ni permisos por modulo. Sigue siendo un esquema de dos roles. | [apps/api/internal/domain/user.go](apps/api/internal/domain/user.go), [apps/api/internal/api/middleware/auth.go](apps/api/internal/api/middleware/auth.go), [apps/api/migrations/003_owner_roles.sql](apps/api/migrations/003_owner_roles.sql), [apps/api/migrations/011_multi_admin.sql](apps/api/migrations/011_multi_admin.sql), [apps/api/internal/api/handlers/admin_users.go](apps/api/internal/api/handlers/admin_users.go) |
| Health del sistema | Implementado parcialmente | Existe `GET /api/admin/health`, `GET /api/admin/engines` y un modulo dedicado `/admin/system` que muestra retencion, feature flags, estado de cola (modo, concurrencia, queued/running, stalled), historicos 5m/15m/1h, storage, DB, Redis, estado fino por worker (heartbeat, ultima tarea, job y fallas recientes) y alertas operativas activas. | Sigue faltando health mas amplio para otros subsistemas (por ejemplo SMTP/webhooks) y una integracion mas rica con observability externa. | [apps/api/internal/api/handlers/health.go](apps/api/internal/api/handlers/health.go), [apps/api/internal/api/handlers/dashboard.go](apps/api/internal/api/handlers/dashboard.go), [apps/api/internal/api/router.go](apps/api/internal/api/router.go), [apps/web/src/lib/api.ts](apps/web/src/lib/api.ts), [apps/web/src/components/admin-system-panel.tsx](apps/web/src/components/admin-system-panel.tsx), [apps/web/src/app/admin/system/page.tsx](apps/web/src/app/admin/system/page.tsx) |
| Engines y capacidades | Implementado parcialmente | Existe `GET /api/admin/engines` y `/admin/system` ahora muestra estado por engine y por capability (familia, operacion, formato, disponibilidad y motivo). | Sigue siendo solo lectura. No hay toggles seguros, no hay estado fino por worker ni capacidad de maintenance mode desde admin. | [apps/api/internal/api/handlers/dashboard.go](apps/api/internal/api/handlers/dashboard.go), [apps/api/internal/capabilities/flags.go](apps/api/internal/capabilities/flags.go), [apps/api/internal/api/router.go](apps/api/internal/api/router.go), [apps/web/src/components/admin-system-panel.tsx](apps/web/src/components/admin-system-panel.tsx) |
| Configuracion de limites de subida | Implementado y operativo, pero acotado | El admin puede editar limites de subida guest/registered y el backend los persiste en `site_settings`. | El backend tambien expone cuotas acumuladas, pero la UI admin no las muestra ni las edita. Las cuotas acumuladas siguen viniendo de config/env. | [apps/api/internal/api/handlers/upload_policy.go](apps/api/internal/api/handlers/upload_policy.go), [apps/web/src/components/admin-dashboard.tsx](apps/web/src/components/admin-dashboard.tsx), [apps/web/src/lib/api.ts](apps/web/src/lib/api.ts) |
| Configuracion de footer | Implementado y operativo | El admin puede editar el mensaje de footer. | Es una capacidad real, pero de poco peso para un panel admin comparada con operacion, usuarios o seguridad. | [apps/api/internal/api/handlers/footer.go](apps/api/internal/api/handlers/footer.go), [apps/web/src/components/admin-dashboard.tsx](apps/web/src/components/admin-dashboard.tsx) |
| SMTP | Implementado y operativo | El admin puede ver, editar y testear SMTP. El backend mascara password y usa secret keeper para secretos. | No hay historia de cambios, health SMTP continuo, ni auditoria de cambios admin. | [apps/api/internal/api/handlers/smtp_settings.go](apps/api/internal/api/handlers/smtp_settings.go), [apps/web/src/components/smtp-settings.tsx](apps/web/src/components/smtp-settings.tsx) |
| Plantillas de email | Implementado y operativo | CRUD completo, preview, edicion visual/codigo y plantillas persistidas en DB. | No hay versionado, diff, rollback, aprobacion ni metricas de uso por plantilla. | [apps/api/internal/api/handlers/email_template.go](apps/api/internal/api/handlers/email_template.go), [apps/api/internal/repository/email_template_repo.go](apps/api/internal/repository/email_template_repo.go), [apps/web/src/components/email-templates.tsx](apps/web/src/components/email-templates.tsx) |
| Webhooks | Implementado y operativo | CRUD de suscripciones, historial de entregas y runtime real de envio asincrono firmado. | Solo soporta `job.completed` y `job.failed`. No hay replay, pause global, filtros avanzados, health por destino ni backend E2E especifico de endpoints admin de webhook. | [apps/api/internal/api/handlers/webhook.go](apps/api/internal/api/handlers/webhook.go), [apps/api/internal/webhook/notifier.go](apps/api/internal/webhook/notifier.go), [apps/api/internal/workers/webhook_handler.go](apps/api/internal/workers/webhook_handler.go), [apps/web/src/components/webhook-settings.tsx](apps/web/src/components/webhook-settings.tsx) |
| Auditoria | Implementado parcialmente | El panel muestra auditoria reciente de eventos operativos y mutaciones admin, y ahora existe un modulo dedicado con `GET /api/admin/audit` + export CSV (`/api/admin/audit/export`) y UI `/admin/audit` con filtros/paginacion. Tambien se auditan accesos (`session_login`, `session_login_failed`, `session_logout`), descargas (`artifact_downloaded`) y controles operativos admin (`admin_queue_*`, `admin_workers_pruned`). | Falta enriquecer filtros/exports avanzados y `AuditDetection` sigue sin emitirse. | [apps/api/internal/domain/audit.go](apps/api/internal/domain/audit.go), [apps/api/internal/api/handlers/admin_audit.go](apps/api/internal/api/handlers/admin_audit.go), [apps/api/internal/api/handlers/auth.go](apps/api/internal/api/handlers/auth.go), [apps/api/internal/api/handlers/artifact.go](apps/api/internal/api/handlers/artifact.go), [apps/api/internal/api/handlers/admin_support.go](apps/api/internal/api/handlers/admin_support.go), [apps/web/src/components/admin-dashboard.tsx](apps/web/src/components/admin-dashboard.tsx), [apps/web/src/components/admin-audit-table.tsx](apps/web/src/components/admin-audit-table.tsx) |
| Observabilidad | Implementado parcialmente | El backend expone metricas Prometheus y reglas de alerta reales; ademas `/admin/system` ya muestra alertas activas minimas derivadas de estado operativo (dependencias, presion de storage, cola y jobs atascados). | Aun faltan series historicas, p95, errores por ventana y conexion directa con stack Prometheus/Alertmanager. | [apps/api/internal/observability/metrics.go](apps/api/internal/observability/metrics.go), [apps/api/alerts.yml](apps/api/alerts.yml), [apps/api/internal/api/handlers/health.go](apps/api/internal/api/handlers/health.go), [apps/web/src/components/admin-system-panel.tsx](apps/web/src/components/admin-system-panel.tsx) |
| Cola y workers | Implementado parcialmente | El runtime tiene workers reales, modo Redis/in-process y concurrencia configurable, `health` expone estado fino por worker persistido en DB y ahora el admin cuenta con controles activos de soporte: pausa/reanudacion de intake, drenado de cola en estado queued y limpieza de workers obsoletos. | La abstraccion `JobQueue` sigue sin exponer inspeccion/control nativo del backend Redis (pause per-queue de Asynq, replay/archivo por estado, etc.) y no hay replay avanzado. | [apps/api/internal/queue/queue.go](apps/api/internal/queue/queue.go), [apps/api/internal/repository/runtime_control_repo.go](apps/api/internal/repository/runtime_control_repo.go), [apps/api/internal/api/handlers/admin_support.go](apps/api/internal/api/handlers/admin_support.go), [apps/api/cmd/server/main.go](apps/api/cmd/server/main.go), [apps/api/cmd/worker/main.go](apps/api/cmd/worker/main.go), [apps/api/internal/api/handlers/health.go](apps/api/internal/api/handlers/health.go), [apps/api/internal/repository/worker_status_repo.go](apps/api/internal/repository/worker_status_repo.go) |
| Soporte y seguridad operativa | Implementado parcialmente | Existen rate limits, ownership checks, storage cleanup, retention, cifrado de secretos, suspension de usuarios, revocacion administrativa de sesiones y ahora trazabilidad de accesos/autenticacion y descargas de artefactos. | Siguen faltando impersonacion, 2FA y consola de incidentes. | [apps/api/internal/api/handlers/authz.go](apps/api/internal/api/handlers/authz.go), [apps/api/internal/api/handlers/admin_users.go](apps/api/internal/api/handlers/admin_users.go), [apps/api/internal/api/handlers/auth.go](apps/api/internal/api/handlers/auth.go), [apps/api/internal/api/handlers/artifact.go](apps/api/internal/api/handlers/artifact.go), [apps/api/internal/auth/service.go](apps/api/internal/auth/service.go), [apps/api/internal/api/middleware/auth.go](apps/api/internal/api/middleware/auth.go), [apps/api/internal/storage/cleanup.go](apps/api/internal/storage/cleanup.go), [apps/api/internal/orchestrator/retention.go](apps/api/internal/orchestrator/retention.go) |

## Matriz contra un panel admin ideal para esta webapp

| Dominio ideal | Estado actual | Brecha principal | Prioridad |
| --- | --- | --- | --- |
| A. Gobierno y acceso | Parcial | Hay `admin` y `user`, gestion basica de usuarios/roles y soporte de multi-admin, pero falta RBAC granular, permisos por modulo y controles de acceso avanzados. | Alta |
| B. Operacion de jobs y conversiones | Parcial | Hay overview y acciones backend puntuales, pero falta la consola real de jobs con filtros, detalle, bulk actions y diagnostico. | Muy alta |
| C. Motores, cola y salud del sistema | Parcial | Hay snapshots de health y engines con runtime/dependencias (cola, storage, DB, Redis), historicos operativos y controles activos basicos de soporte para intake/cola/workers. Falta inspeccion profunda del backend de cola. | Muy alta |
| D. Configuracion del producto | Parcial alto | Limites de subida y footer ya existen, pero el espacio de settings crecio sin una capa admin dedicada y sin cubrir cuotas acumuladas u otros limites. | Media |
| E. Comunicaciones salientes | Alto | SMTP, templates y webhooks ya estan bastante bien resueltos para una primera version. | Media |
| F. Observabilidad y auditoria | Parcial | Hay metricas, alertas y feed de auditoria operativa, mas auditoria administrativa exportable; ya se cubren accesos, descargas y controles de soporte. Aun falta mejor slicing y export avanzado. | Alta |
| G. Soporte y seguridad operativa | Parcial bajo | Ya existe revocacion/suspension y trazabilidad de accesos/descargas, pero faltan herramientas de soporte, respuesta ante incidentes e impersonacion controlada. | Media |

## Hallazgos clave

### 1. El admin actual es una pagina unica de resumen y configuracion, no un backoffice modular

La superficie admin real esta concentrada en una sola pagina y un solo componente principal. Eso simplifica hoy, pero ya mezcla overview, auditoria y multiples configuraciones en un mismo lugar y dificulta crecer de forma ordenada.

### 2. La autorizacion backend esta mejor resuelta que el gating del frontend

El backend corta bien con `RequireAdmin` y la app web ahora tambien valida JWT y rol admin en middleware antes de servir `/admin`. El hardening mejoro bastante, aunque no existe un segundo factor o step-up auth para acciones sensibles.

### 3. El hueco mas serio sigue siendo la operacion de jobs

Para una webapp de conversion, el admin deberia servir primero para entender y operar jobs. Eso ya mejoro con `/admin/jobs`, `GET /api/admin/jobs`, acciones `cancel/retry` por fila, acciones masivas por seleccion/filtro y deteccion de jobs atascados, pero sigue faltando sumar diagnostico mas profundo de ejecucion, fallas y backlog.

### 4. La gestion de usuarios ya existe, pero sigue siendo minima

Ya existe una capa minima de gestion cotidiana: listado admin de usuarios con busqueda/filtro/paginacion, promote/demote de rol, suspension/reactivacion y revocacion de sesiones. Lo que sigue faltando son operaciones de soporte mas amplias como invitaciones, borrado o trazabilidad de accesos.

### 5. El modelo ya escala a mas de un admin, pero no a permisos mas granulares

La nueva migracion elimina la restriccion de admin unico, asi que ya hay redundancia operativa minima. El siguiente limite real es la ausencia de RBAC o permisos por modulo.

### 6. Health, engines y feature flags ya mejoraron, pero aun no forman una consola operativa completa

Hoy la UI ya consume `health` y `engines`, existe `/admin/system` como modulo dedicado y el payload de health ya expone runtime/dependencias (cola, storage, DB y Redis), junto con alertas activas minimas por umbral, historicos 5m/15m/1h y estado fino por worker. Sigue faltando una frontera clara entre configuracion de despliegue y configuracion operativa editable, ademas de health mas amplio para otros subsistemas.

### 7. La auditoria ya cubre administracion basica, pero falta cerrar soporte operativo

Se auditan uploads y cambios de estado de jobs, tambien cambios de configuracion admin y de roles, y ahora hay una vista dedicada `/admin/audit` con export CSV. Ya quedaron cubiertos eventos de acceso/auth, descargas de artefactos y acciones operativas de soporte; lo pendiente principal es mejorar filtros y export avanzado para investigacion.

### 8. La cobertura de tests es razonable en overview/config, pero desigual en integraciones

Hay buenos tests de componentes y E2E backend para overview, upload policy, SMTP, templates, webhooks admin, jobs admin y usuarios admin. La cobertura sigue siendo mas fuerte en backend que en nuevas pantallas frontend dedicadas.

## Estado de testing del admin

### Cobertura clara hoy

- Overview admin y guardado de upload policy: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) (`TestE2E_AdminCanUpdateUploadPolicy`, `TestE2E_DashboardReturnsData`, `TestE2E_NonAdminCannotAccessAdminEndpoints`).
- SMTP admin: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) (`TestE2E_AdminSMTPSettings`).
- Plantillas de email admin: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) (`TestE2E_AdminEmailTemplates`).
- Webhooks admin: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) (`TestE2E_AdminWebhookCRUD`).
- Jobs admin: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) (`TestE2E_AdminJobsList`, incluyendo contrato de backlog atascado y filtro `stalled`; `TestE2E_AdminJobsBatchActions`, para acciones masivas admin).
- Usuarios admin: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) (`TestE2E_AdminUsersList`, incluyendo filtros y paginacion; `TestE2E_AdminUserSuspensionAndSessionRevocation`, para suspension y revocacion de sesiones).
- Health admin detallado: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) (`TestE2E_AdminDetailedHealth`, incluyendo contrato de `alerts` y snapshot de backlog atascado; `TestE2E_AdminDetailedHealthIncludesWorkersAndHistory`, para workers e historicos).
- Engines admin detallado: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) (contrato de `/api/admin/engines`, incluyendo lista de capabilities y motivos de disponibilidad).
- Alertas operativas por umbral (unit): [apps/api/internal/api/handlers/health_test.go](apps/api/internal/api/handlers/health_test.go) (`TestBuildHealthAlerts_*`).
- Auditoria admin y export: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) (`TestE2E_AdminAuditListAndExport`).
- Dashboard admin en frontend: [apps/web/src/components/admin-dashboard.test.tsx](apps/web/src/components/admin-dashboard.test.tsx).
- System panel frontend: [apps/web/src/components/admin-system-panel.test.tsx](apps/web/src/components/admin-system-panel.test.tsx) (controles operativos de intake/cola/workers y estados de validacion).
- Widgets frontend: [apps/web/src/components/smtp-settings.test.tsx](apps/web/src/components/smtp-settings.test.tsx), [apps/web/src/components/email-templates.test.tsx](apps/web/src/components/email-templates.test.tsx), [apps/web/src/components/webhook-settings.test.tsx](apps/web/src/components/webhook-settings.test.tsx).

### Cobertura incompleta o indirecta

- Webhooks admin en browser: [apps/web/e2e/batch-webhooks.spec.ts](apps/web/e2e/batch-webhooks.spec.ts) cubre el flujo UI, pero mockea `/api/admin/webhooks`.
- Las nuevas pantallas [apps/web/src/app/admin/jobs/page.tsx](apps/web/src/app/admin/jobs/page.tsx), [apps/web/src/app/admin/users/page.tsx](apps/web/src/app/admin/users/page.tsx) y [apps/web/src/app/admin/audit/page.tsx](apps/web/src/app/admin/audit/page.tsx) no tienen aun tests de componente o browser dedicados; `/admin/system` ya tiene cobertura de componente sobre [apps/web/src/components/admin-system-panel.tsx](apps/web/src/components/admin-system-panel.tsx).

### Higiene de suite (ultima pasada)

- Se limpio una rotura de compilacion en [apps/api/internal/security/secret_keeper_test.go](apps/api/internal/security/secret_keeper_test.go) (declaracion de `package` duplicada).
- Se completo el stub de repositorio en [apps/api/internal/email/notifier_test.go](apps/api/internal/email/notifier_test.go) para cumplir el contrato actual de `UserRepository`.
- Se detecto y corrigio un bug real de runtime en [apps/web/src/components/admin-system-panel.tsx](apps/web/src/components/admin-system-panel.tsx) (`formatDate` no definido).
- Se actualizo el mock de API en [apps/web/src/components/admin-dashboard.test.tsx](apps/web/src/components/admin-dashboard.test.tsx) para cubrir `getHealthInfo` y mantener la suite estable.
- Resultado: suite backend `go test ./... -count=1`, suite frontend `npm run test` y lint frontend en verde; sin errores activos en diagnosticos del workspace.

## Recomendacion de arquitectura

Mi recomendacion es no seguir creciendo el admin actual como una pagina gigante con llamadas directas dispersas, sino moverlo a una frontera de backoffice mas explicita.

### Recomendaciones concretas

1. Crear una capa admin explicita en backend.
   - Un `internal/admin` o un conjunto de handlers/query services orientados a backoffice.
   - Separar claramente endpoints de lectura administrativa de endpoints operativos de usuario.

2. Introducir read models admin dedicados.
   - `AdminJobList`, `AdminUserList`, `AdminSystemHealth`, `AdminAuditFeed`.
   - No reutilizar solo el dashboard agregado para cubrir necesidades de exploracion y soporte.

3. Separar inspeccion de cola de la abstraccion de encolado.
   - `JobQueue` hoy no permite ver backlog ni workers.
   - Hace falta una interfaz aparte de inspeccion, por ejemplo `QueueInspector`, para Redis/InProcess sin contaminar el flujo normal de jobs.

4. Auditar toda mutacion admin.
   - Cambios de SMTP, footer, upload policy, templates, webhooks, promociones de rol y futuras acciones de soporte.
   - Esa auditoria debe ser de primer orden y visible desde el panel.

5. Mantener capabilities y engines con una sola fuente de verdad.
   - El admin debe leer del mismo registro de capabilities/engines.
   - Evitar duplicar reglas de disponibilidad entre frontend, admin y workers.

6. No convertir `site_settings` en un cajon de sastre.
   - Sirve para settings chicos y puntuales.
   - Para flags de operacion mas sensibles conviene un modelo admin/versionado/auditado o seguir en config de despliegue si el riesgo es alto.

## Plan priorizado y accionable

### Fase 0: cerrar huecos con lo que ya existe ✅ COMPLETADA

Objetivo: sacar valor rapido sin redisenar todo.

1. ✅ Integrar `health` y `engines` al panel actual. — Seccion "System Health" en sidebar operativo con retention policy y feature flags.
2. ✅ Mostrar errores y mas detalle en la tabla de jobs recientes. — Columna `capabilityId` y display de error bajo el badge de status.
3. ✅ Agregar auditoria a todas las mutaciones admin existentes. — 11 tipos de evento audit (`admin_*`) en dominio, emitidos por footer, upload policy, SMTP, templates, webhooks y role changes.
4. ✅ Agregar E2E backend para CRUD admin de webhooks. — `TestE2E_AdminWebhookCRUD` cubre auth, create, list, update, delete.
5. ✅ Endurecer el acceso a `/admin` del lado web para evitar que un no-admin vea la shell. — Middleware async JWT role verification con jose v6, 5 tests unitarios.

### Fase 1: version minima razonable del admin ✅ COMPLETADA

Objetivo: cubrir operacion diaria real de una webapp de conversion.

1. ✅ Crear una vista `/admin/jobs` con filtros por estado, capability, usuario, fecha y texto libre. — `GET /api/admin/jobs` con `AdminJobFilter` (status, capability, search, limit, offset), handler, E2E test, pagina frontend con filtros y paginacion.
2. ✅ Permitir ver detalle de job, error, tiempos y artefacto asociado. — Jobs table muestra error, capabilityId, outputFormat, timestamps.
3. ✅ Exponer acciones admin de cancelar y reintentar desde esa vista. — `/admin/jobs` ahora permite `cancel` para queued/running y `retry` para failed usando endpoints existentes.
4. ✅ Crear un listado admin basico de usuarios con busqueda y visibilidad de rol/alta. — `GET /api/admin/users`, `PATCH /api/admin/users/{id}/role`, pagina `/admin/users` con tabla, promote/demote, E2E test.
5. ✅ Permitir al menos promover un segundo admin de forma controlada y auditable. — Migration 011 elimina `idx_users_single_admin`, promote/demote con auditoria `admin_role_changed`.

### Fase 2: operacion del sistema

Objetivo: que el admin sirva para entender salud y capacidad del sistema.

1. ✅ Completado: `/admin/system` ahora consume un `health` extendido con cola (modo/concurrencia/queued/running), storage (path/disponible), DB y Redis en runtime.
2. ✅ Completado (version inicial): `GET /api/admin/jobs` y `/admin/jobs` ya permiten inspeccion de jobs atascados (resumen global, filtro `stalled`, `stalledReason` y `backlogAgeSec` por fila).
3. ✅ Completado: `/admin/system` ya muestra historicos 5m/15m/1h para cola/fallas/latencia y estado fino por worker con heartbeat, ultima tarea y fallas recientes.
4. 🟨 Avanzado: ya se muestran alertas activas minimas derivadas de umbrales operativos en `/admin/system` (incluyendo jobs atascados); faltan alertas conectadas a stack observability y series historicas mas profundas.
5. ✅ Completado: ya se muestra estado por engine y por capability en `/admin/system`, incluyendo motivo de indisponibilidad.

### Fase 3: soporte y gobernanza

Objetivo: cerrar huecos de seguridad y soporte sin sobreconstruir.

1. ✅ Completado: revocacion de sesiones y suspension/reactivacion de usuarios.
2. ✅ Completado: historial de cambios admin y export simple de auditoria (`/api/admin/audit` + `/api/admin/audit/export`, UI `/admin/audit`).
3. ✅ Completado parcialmente: controles operativos de soporte (pausar/reanudar intake, drenar cola, limpiar workers obsoletos) y auditoria de esas acciones.
4. Versionado minimo de plantillas de email.

## Backlog inicial sugerido

### Alta prioridad

1. ✅ Crear `GET /api/admin/jobs` con filtros, orden y paginacion.
2. ✅ Crear `/admin/jobs` reutilizando las acciones backend de cancel/retry ya existentes.
3. ✅ Crear `GET /api/admin/users` con busqueda simple y metadatos basicos.
4. ✅ Remover la restriccion de admin unico y reemplazarla por una regla de negocio mas segura y auditable.
5. ✅ Auditar `PUT /admin/footer-message`, `PUT /admin/upload-policy`, `PUT /admin/smtp-settings`, `POST /admin/smtp-test`, CRUD de templates, CRUD de webhooks y cambios de rol.
6. ✅ Agregar E2E backend de webhooks admin.

### Prioridad media

7. ✅ Integrar `GET /api/admin/health` y `GET /api/admin/engines` en la UI.
8. ✅ Extender health con informacion real de cola, workers (signal), disco y dependencias.
9. 🟨 Exponer alertas y metricas operativas minimas en admin. — Alertas activas minimas e inspeccion inicial de atascos completadas tambien en `/admin/system`; pendiente metricas historicas y slicing temporal.
10. Agregar replay manual de webhook fallido.

### Prioridad baja, pero util

11. Versionado y rollback simple de plantillas de email.
12. ✅ Descargar o exportar feed de auditoria.
13. ✅ Mostrar trazabilidad de descargas de artefactos si el contexto operativo lo requiere.

## Version minima razonable

Si tuviera que definir una `version minima razonable` del panel admin para esta webapp, seria esta:

1. `Resumen`: metricas globales, jobs fallidos recientes, engines no disponibles, alertas basicas, historicos minimos y auditoria reciente.
2. `Jobs`: listado administrativo real con filtros, detalle, cancel/retry y acciones masivas.
3. `Usuarios`: listado simple con busqueda, rol, suspension y revocacion de sesiones, con capacidad de mantener al menos dos admins.
4. `Configuracion`: upload policy, footer y SMTP.
5. `Comunicaciones`: plantillas de email y webhooks.

Y le pondria tres condiciones no negociables:

1. Todas las mutaciones admin auditadas.
2. Acceso admin validado antes de renderizar la shell protegida.
3. Cobertura automatizada real para los endpoints admin criticos, incluido webhook CRUD.

Con eso, el panel seguiria siendo pequeno y mantenible, pero ya seria util para operar una plataforma de conversion de archivos de verdad. Con los cambios de esta pasada, el estado actual ya esta cerca de esa linea minima, aunque todavia falta cerrar observabilidad operativa y soporte avanzado.