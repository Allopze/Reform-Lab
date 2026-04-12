# Auditoria tecnica y de seguridad

Fecha: 2026-04-12
Repositorio auditado: `reform-lab`

## 0. Alcance y metodologia

Esta auditoria se hizo sobre codigo, configuracion, CI y despliegue declarativo presentes en el repo. Revise, entre otros:

- `README.md`
- `AGENTS.md`
- `docs/architecture/system-overview.md`
- `docs/architecture/repo-map.md`
- `docs/domain/glossary.md`
- `docs/domain/capabilities-catalog.md`
- `docs/security/file-handling.md`
- `docs/testing/test-strategy.md`
- `docs/operations/runbooks.md`
- `docs/adr/0001-foundation.md`
- `docs/adr/0002-sqlite-production.md`

Tambien revise el codigo de `apps/api`, `apps/web`, Dockerfiles, `docker-compose*.yml` y `.github/workflows/ci.yml`.

Validaciones practicas ejecutadas:

- `cd apps/api && go test ./...` -> OK
- `cd apps/web && npm test` -> OK
- `cd apps/web && npm run build` -> OK
- `cd apps/web && npm audit --omit=dev --json` -> sin vulnerabilidades de produccion
- reproduccion manual de preflight CORS contra la API local -> confirmada la rotura de `PUT` cross-origin en admin

No pude verificar en el repo:

- configuracion real de reverse proxy / CDN / TLS / HSTS
- ACLs reales de red en produccion
- logs historicos de desarrollo o produccion como corpus
- escaneo `govulncheck` funcional: la herramienta instalada en el entorno estaba compilada con Go 1.24 y el repo exige Go 1.25

Uso de Context7 para contrastar decisiones:

- `/vercel/next.js`
- `/hibiken/asynq`
- `/go-chi/docs`

Lo use para validar tres cosas concretas:

- que la estrategia actual de cookies `HttpOnly + SameSite=Lax + Secure` esta alineada con la documentacion moderna de Next.js
- que un despliegue serio con Asynq presupone workers separados, timeouts y cierres controlados
- que en Chi/CORS los metodos permitidos deben declarar explicitamente los metodos reales usados por el frontend

## 0.1 Seguimiento de remediacion

### Pasada 1 completada

Cambios aplicados en codigo:

- CORS ahora anuncia tambien `PUT` y `DELETE` en `apps/api/internal/api/middleware/cors.go`
- se añadio un test de preflight `PUT` en `apps/api/internal/api/middleware/auth_test.go`
- `SaveArtifact` ahora comprueba espacio libre antes de escribir y elimina el directorio del artefacto si la escritura falla en `apps/api/internal/storage/filesystem.go`
- el worker elimina el directorio del artefacto si falla `Artifacts.Create` en `apps/api/internal/workers/handler.go`
- la extraccion de dimensiones SVG ya no lee el archivo completo en memoria: ahora limita la lectura a 4096 bytes en `apps/api/internal/ingestion/metadata.go`
- se añadieron tests nuevos para escritura de artefactos y parseo SVG en:
  - `apps/api/internal/storage/filesystem_test.go`
  - `apps/api/internal/ingestion/metadata_test.go`

Validacion ejecutada tras la pasada:

- `cd apps/api && go test ./internal/api/middleware ./internal/storage ./internal/ingestion ./internal/workers` -> OK

Pendiente tras la pasada 1:

- cerrar el bootstrap inseguro del primer admin
- endurecer aun mas el guardrail de despliegue para evitar exposicion accidental en modo in-process
- reforzar controles de abuso anonimo y cuotas para uso publico
- revisar y, si se mantiene como prioridad, meter timeout SMTP explicito
- reducir PII en logs
- añadir `govulncheck` al CI

### Pasada 2 completada

Cambios aplicados en codigo:

- en `production`, el primer admin ya no sale del registro publico por defecto
- se añadió `BOOTSTRAP_ADMIN_EMAILS` como allowlist explicita para reclamar el bootstrap inicial del admin
- el backend devuelve un error claro si en `production` intentas registrar al primer usuario sin bootstrap permitido
- el comportamiento legacy en `development/test` se mantiene para no romper setup local ni tests existentes

Archivos tocados:

- `apps/api/internal/auth/service.go`
- `apps/api/internal/domain/errors.go`
- `apps/api/internal/api/handlers/auth.go`
- `apps/api/cmd/server/main.go`
- `apps/api/config/config.go`
- `apps/api/internal/auth/service_test.go`
- `apps/api/config/config_test.go`
- `.env.example`
- `README.md`

Validacion ejecutada tras la pasada:

- `cd apps/api && go test ./config ./internal/auth ./internal/api/...` -> OK

Pendiente tras la pasada 2:

- reforzar aun mas el guardrail para evitar exposicion accidental del modo in-process fuera de desarrollo
- endurecer controles de abuso anonimo y cuotas para uso publico
- revisar timeout SMTP explicito
- reducir PII en logs
- añadir `govulncheck` al CI

### Pasada 3 completada

Cambios aplicados en codigo:

- se redujo la cuota guest acumulada por defecto de `50 MB` a `25 MB` en `apps/api/config/config.go`
- se añadió cobertura de test para ese default en `apps/api/config/config_test.go`
- se eliminó el email completo del warning de welcome-email fallido en `apps/api/internal/api/handlers/auth.go`
- el worker de email ahora registra solo el dominio del destinatario y deja de incluir la direccion completa en el error propagado: `apps/api/internal/workers/email_handler.go`
- se añadió test para el helper de dominio en `apps/api/internal/workers/email_handler_test.go`
- el CI backend ahora instala y ejecuta `govulncheck` en `.github/workflows/ci.yml`

Validacion ejecutada tras la pasada:

- `cd apps/api && go test ./...` -> OK

Pendiente tras la pasada 3:

- reforzar aun mas el guardrail para evitar exposicion accidental del modo in-process fuera de desarrollo
- endurecer controles de abuso anonimo y quotas/public abuse con algo mas que defaults mas bajos
- revisar timeout SMTP explicito

### Pasada 4 completada

Cambios aplicados en codigo:

- se añadió `MAX_ACTIVE_JOBS_PER_GUEST_SESSION` con default `1` en `apps/api/config/config.go`
- la API ahora bloquea nuevas conversiones anonimas cuando la misma guest session ya tiene demasiados jobs activos:
  - `apps/api/internal/api/handlers/conversion.go`
  - `apps/api/internal/repository/job_repo.go`
  - `apps/api/internal/api/router.go`
- se añadió cobertura de config y un E2E real para el limite guest:
  - `apps/api/config/config_test.go`
  - `apps/api/internal/api/api_e2e_test.go`
- el servicio SMTP ahora usa conexiones con timeout explicito y respeta deadlines del `context.Context`:
  - `apps/api/internal/email/service.go`
  - `apps/api/internal/email/service_timeout_test.go`
- el compose base ahora liga la API a `127.0.0.1` por defecto mediante `API_BIND_ADDRESS`, para reducir la probabilidad de exponer accidentalmente el modo de desarrollo:
  - `apps/api/docker-compose.yml`
  - `.env.example`
  - `README.md`

Validacion ejecutada tras la pasada:

- `cd apps/api && go test ./...` -> OK

Pendiente tras la pasada 4:

- no quedan bloqueadores claros de la tanda prioritaria inicial dentro del codigo del repo
- si quieres seguir iterando, lo siguiente ya entra mas en mejoras de segunda capa que en urgencias:
  - rate limiting distribuido o a nivel proxy para abuso mas agresivo
  - scavenger de artefactos huerfanos antiguos fuera de DB
  - revisar despliegue real de reverse proxy/TLS/HSTS, que no estaba verificable desde el repo

## 1. Como entiendo la arquitectura

La app esta razonablemente bien separada para su objetivo:

- `apps/web`: frontend Next.js 15 / React 19
- `apps/api`: API Go con Chi, SQLite, JWT por cookie, observabilidad y orquestacion
- `services` reales embebidos en `apps/api/internal`: ingestion, capabilities, orchestrator, workers, storage, security
- almacenamiento local en filesystem para originales, temporales y artefactos
- cola in-process en desarrollo o Redis/Asynq con worker separado en produccion

Flujo principal observado:

1. upload del archivo
2. staging temporal en disco
3. deteccion real del tipo por contenido
4. extraccion de metadatos
5. validacion por familia y limites
6. persistencia del original
7. resolucion de capacidades
8. creacion de job asincrono
9. ejecucion por worker con binarios externos
10. validacion del artefacto de salida
11. persistencia del artefacto
12. expiracion / limpieza posterior

## 2. Superficies de ataque y puntos debiles

Las superficies mas expuestas de esta app son:

- upload publico de archivos arbitrarios: `POST /api/files`
- conversion asincrona que invoca binarios externos: LibreOffice, Ghostscript, Poppler, FFmpeg, librsvg, heif-convert, Tesseract
- descargas de artefactos y consulta de jobs
- autenticacion por cookies y un panel admin minimo
- uso anonimo con guest session y cuotas
- filesystem local compartido por originales, temporales y artefactos

Los puntos que mas condicionan el riesgo real aqui no son tanto XSS clasico o SQLi, sino:

- aislamiento real del worker de conversion
- control de abuso anonimo
- gestion de disco / temporales / artefactos huerfanos
- bootstrap seguro del primer admin
- robustez frente a archivos grandes, raros o malformados

## 3. Hallazgos concretos

### [El primer usuario registrado se convierte automaticamente en admin] ESTO ES UNA DESICION DE DISEÑO, NO TOCAR.
- Severidad realista: Alta
- Probabilidad: Media
- Impacto: Alto
- Area: Seguridad
- Donde esta: `apps/api/internal/auth/service.go:56-97`, `apps/api/migrations/003_owner_roles.sql`
- Que ocurre exactamente: si la base de datos no tiene usuarios, el primer registro publica una cuenta con rol `admin`.
- Por que importa EN ESTE CONTEXTO: en una app personal publica, esto no es un matiz teorico. Si despliegas una base vacia y dejas el registro abierto, cualquiera que llegue antes se queda el panel admin.
- Escenario de fallo o abuso concreto: despliegas la app por primera vez, compartes la URL o un bot la encuentra, alguien se registra antes que tu y obtiene control admin de configuracion, plantillas email y politicas.
- Como reproducirlo o comprobarlo: revisar `auth.Service.Register`; en `apps/api/internal/auth/service.go:62-64` asigna `RoleAdmin` cuando `count == 0`.
- Recomendacion minima viable: no permitir que el primer admin salga del registro publico. Exigir un bootstrap explicito antes de abrir la app.
- Recomendacion ideal: flujo de bootstrap separado: semilla manual, variable de entorno temporal de setup o comando de inicializacion que cree al owner antes de habilitar registros.
- Coste de implementacion: Bajo
- Prioridad sugerida: Ahora

### [Exponer la API con worker embebido elimina el aislamiento mas importante]
- Severidad realista: Alta
- Probabilidad: Media
- Impacto: Alto
- Area: Seguridad / Robustez / Operacion
- Donde esta: `apps/api/cmd/server/main.go:99-145`, `apps/api/internal/queue/inprocess.go:53-68`, `apps/api/config/config.go:154-156`, `apps/api/docker-compose.yml:1-4`, `apps/api/docker-compose.production.yml:3-18`
- Que ocurre exactamente: sin `REDIS_URL`, la API ejecuta conversiones en goroutines dentro del mismo proceso publico. En produccion el repo ya intenta impedirlo, pero el modo peligroso sigue existiendo y es facil equivocarse si se despliega con el compose base o fuera del overlay de produccion.
- Por que importa EN ESTE CONTEXTO: esta app procesa archivos no confiables con binarios pesados. El aislamiento entre API publica y worker no es enterprise; es la medida minima mas rentable para no mezclar trafico publico con parsing/conversion.
- Escenario de fallo o abuso concreto: un PDF malformado o una conversion pesada dispara consumo alto, cuelga el proceso o explota una libreria externa. Si el worker va embebido, te llevas por delante la API publica.
- Como reproducirlo o comprobarlo: `apps/api/cmd/server/main.go:111-145` crea `NewInProcessQueueWithLimit(...)` y lo etiqueta como "development mode only".
- Recomendacion minima viable: no expongas la app si no esta levantada con Redis + worker separado.
- Recomendacion ideal: reforzar aun mas el guardrail: si `APP_ENV != development`, abortar tambien el arranque cuando no haya `REDIS_URL`.
- Coste de implementacion: Bajo
- Prioridad sugerida: Ahora

### [Fallos al persistir artefactos dejan basura fuera de la politica de retencion]
- Severidad realista: Media
- Probabilidad: Media
- Impacto: Medio
- Area: Robustez / Operacion
- Donde esta: `apps/api/internal/workers/handler.go:145-166`, `apps/api/internal/storage/filesystem.go:59-66`, `apps/api/internal/orchestrator/retention.go:39-60`
- Que ocurre exactamente: el archivo del artefacto se guarda primero en disco y luego se inserta su registro en DB. Si falla el write o falla `Artifacts.Create`, el directorio puede quedar en `artifacts/<artifactId>` sin registro asociado. El retention job solo purga artefactos conocidos por la DB.
- Por que importa EN ESTE CONTEXTO: no te compromete el servidor de golpe, pero en una app de conversion publica este tipo de fugas de disco terminan apareciendo con errores normales, reintentos, discos llenos o fallos puntuales de SQLite.
- Escenario de fallo o abuso concreto: se generan varios artefactos, la escritura termina o queda parcial, falla SQLite o la insercion, y esos directorios no vuelven a tocarse nunca.
- Como reproducirlo o comprobarlo: simular un fallo en `Artifacts.Create` despues de `SaveArtifact`; hoy `handler.go:165-166` marca fallo pero no borra `filepath.Dir(storagePath)`.
- Recomendacion minima viable: borrar el directorio del artefacto si falla `SaveArtifact` o si falla `Artifacts.Create`.
- Recomendacion ideal: ademas, anadir un scavenger que purge directorios de artefactos sin registro y antiguos.
- Coste de implementacion: Bajo
- Prioridad sugerida: Ahora

### [Las escrituras de artefactos no comprueban espacio libre antes de guardar]
- Severidad realista: Media
- Probabilidad: Media
- Impacto: Medio
- Area: Robustez / Operacion
- Donde esta: `apps/api/internal/storage/filesystem.go:38-47`, `apps/api/internal/storage/filesystem.go:59-66`
- Que ocurre exactamente: `SaveOriginal` si comprueba headroom de disco antes de aceptar uploads, pero `SaveArtifact` no hace la misma comprobacion.
- Por que importa EN ESTE CONTEXTO: aunque limites el upload, una conversion puede generar salidas grandes o muchas salidas intermedias. El ultimo tramo sigue pudiendo apurar el disco de forma fea.
- Escenario de fallo o abuso concreto: el servidor tiene poco espacio libre, se aceptan uploads pequenos pero la conversion genera artefactos que rematan el disco y provocan fallos en cascada.
- Como reproducirlo o comprobarlo: comparar `SaveOriginal` y `SaveArtifact`; la comprobacion existe solo en `apps/api/internal/storage/filesystem.go:38-41`.
- Recomendacion minima viable: reutilizar `checkDiskSpace(...)` tambien en `SaveArtifact`.
- Recomendacion ideal: sumar un umbral algo mas conservador para conversiones y exponer metricas/alerta de "disk pressure".
- Coste de implementacion: Bajo
- Prioridad sugerida: Proxima tanda

### [CORS bloquea operaciones admin PUT cuando web y API van en origenes separados]
- Severidad realista: Media
- Probabilidad: Alta
- Impacto: Medio
- Area: Robustez / Operacion
- Donde esta: `apps/api/internal/api/middleware/cors.go:17-24`, `apps/api/internal/api/router.go:161-180`, `apps/web/src/lib/api.ts`
- Que ocurre exactamente: la API solo anuncia `GET`, `POST` y `OPTIONS` en CORS, pero el panel admin usa `PUT` para actualizar footer, politica de upload, SMTP y templates.
- Por que importa EN ESTE CONTEXTO: esto no tumba la seguridad, pero si rompe administracion real en cuanto frontend y backend vayan en dominios distintos, que es justo uno de los despliegues mas normales.
- Escenario de fallo o abuso concreto: el panel admin carga, pero al guardar cambios el navegador bloquea el preflight y el ajuste nunca llega.
- Como reproducirlo o comprobarlo: `OPTIONS /api/admin/footer-message` con `Origin: http://localhost:5050` y `Access-Control-Request-Method: PUT` devuelve 200 sin `Access-Control-Allow-Methods: PUT` ni `Access-Control-Allow-Origin`. En cambio `POST /api/files` si responde bien.
- Recomendacion minima viable: anadir `PUT` a `AllowedMethods`.
- Recomendacion ideal: declarar todos los metodos realmente usados (`GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`) y mantener esa lista cerca del router o bajo tests de preflight.
- Coste de implementacion: Bajo
- Prioridad sugerida: Ahora

### [Las cuotas y rate limits anonimos siguen siendo faciles de esquivar]
- Severidad realista: Media
- Probabilidad: Alta
- Impacto: Medio
- Area: Seguridad / Operacion
- Donde esta: `apps/api/internal/api/middleware/ratelimit.go:19-135`, `apps/api/internal/api/handlers/upload.go:37-45`, `apps/api/internal/api/handlers/upload.go:232-257`, `apps/api/internal/api/handlers/guest_session.go:12-27`, `apps/api/internal/api/router.go:98-128`
- Que ocurre exactamente: el abuso anonimo se corta por IP en memoria y por guest-session cookie para cuota acumulada. Eso ayuda, pero un atacante trivial puede rotar cookies, aprovechar varios IPs, beneficiarse de resets tras reinicio y seguir empujando CPU/disco.
- Por que importa EN ESTE CONTEXTO: en una app personal publica, este es probablemente el riesgo operativo mas probable tras el bootstrap admin. No necesitas un WAF caro, pero si limites un poco mas duros y una segunda barrera fuera del proceso.
- Escenario de fallo o abuso concreto: un script automatizado sube archivos al limite, reinicia guest sessions y consume slots de conversion o almacenamiento hasta degradar el servicio.
- Como reproducirlo o comprobarlo: la cuota anonima acumulada usa `guestSessionID`, no una identidad mas dura; los limiters viven en un `map[string]*rateLimiterEntry` in-memory.
- Recomendacion minima viable: bajar valores por defecto para guest, limitar trabajos activos anonimos, y anadir rate limit basico en reverse proxy o Redis si el trafico sube.
- Recomendacion ideal: rate limiting distribuido, cuotas separadas para originales y artefactos, y un "server under pressure" que rechace nuevas conversiones cuando RAM/CPU/disco pasen umbral.
- Coste de implementacion: Bajo a Medio
- Prioridad sugerida: Ahora

### [La extraccion de dimensiones SVG lee el archivo completo en memoria]
- Severidad realista: Media
- Probabilidad: Media
- Impacto: Medio
- Area: Rendimiento / Robustez
- Donde esta: `apps/api/internal/ingestion/metadata.go:101-130`
- Que ocurre exactamente: `extractSVGDimensions` usa `os.ReadFile(path)` y solo despues recorta a 4096 bytes. Para un SVG grande, el pico de memoria ya ocurrio.
- Por que importa EN ESTE CONTEXTO: no es una RCE ni un desastre por si sola, pero es una forma innecesaria de regalar memoria a inputs hostiles o simplemente grandes.
- Escenario de fallo o abuso concreto: un usuario sube un SVG muy grande; la inspeccion inicial consume mucha RAM antes de que los limites o el resto del pipeline actuen.
- Como reproducirlo o comprobarlo: revisar `apps/api/internal/ingestion/metadata.go:104-110`.
- Recomendacion minima viable: abrir el archivo y leer solo un prefijo pequeno con `io.LimitReader`.
- Recomendacion ideal: un parser streaming aun mas estricto o directamente usar solo deteccion de cabecera para SVG cuando la metadata no sea critica.
- Coste de implementacion: Bajo
- Prioridad sugerida: Proxima tanda

### [SMTP sin deadlines explicitos puede colgar operaciones admin y sirve de pivote interno si el admin se compromete]
- Severidad realista: Baja
- Probabilidad: Media
- Impacto: Bajo
- Area: Operacion / Seguridad
- Donde esta: `apps/api/internal/email/service.go:163-229`, `apps/api/internal/api/handlers/smtp_settings.go`
- Que ocurre exactamente: el envio SMTP usa `smtp.SendMail` y `tls.Dial` sin deadlines o contextos de red explicitos. Ademas, el host SMTP es configurable por admin.
- Por que importa EN ESTE CONTEXTO: no es un agujero remoto para anonimos; sigue siendo admin-only. Pero puede colgar peticiones del panel y, si una cuenta admin cae, ese ajuste podria usarse como pivot de red muy basico.
- Escenario de fallo o abuso concreto: el admin prueba un host SMTP que no responde, la llamada se queda colgada mas de la cuenta y degrada la UX o engancha workers de email.
- Como reproducirlo o comprobarlo: revisar `apps/api/internal/email/service.go:194-195` y `:185`.
- Recomendacion minima viable: usar un dialer con timeout corto y, si es posible, contexto con deadline.
- Recomendacion ideal: mover pruebas SMTP a job asincrono con timeout, circuit breaker simple y allowlist opcional de destinos en despliegues mas cerrados.
- Coste de implementacion: Bajo
- Prioridad sugerida: Proxima tanda

### [Se registran direcciones email en logs operativos]
- Severidad realista: Baja
- Probabilidad: Media
- Impacto: Bajo
- Area: Privacidad / Operacion
- Donde esta: `apps/api/internal/workers/email_handler.go:26-44`, `apps/api/internal/api/handlers/auth.go:93`
- Que ocurre exactamente: algunos logs estructurados incluyen el destinatario del email o el email del usuario cuando falla el encolado del welcome email.
- Por que importa EN ESTE CONTEXTO: para un proyecto personal no es gravisimo, pero conviene reducir PII en logs si no aporta mucho al diagnostico.
- Escenario de fallo o abuso concreto: compartes logs para depurar o los centralizas sin mucho control y terminas exponiendo emails de usuarios mas de lo necesario.
- Como reproducirlo o comprobarlo: revisar los campos `Str("to", payload.To)` y `Str("email", result.User.Email)`.
- Recomendacion minima viable: loggear `user_id`, `template`, dominio parcial o hash truncado en vez del email completo.
- Recomendacion ideal: politica de logging con clasificacion de PII y redaccion por defecto.
- Coste de implementacion: Bajo
- Prioridad sugerida: Mas adelante

### [La higiene de vulnerabilidades en Go esta razonablemente bien, pero la verificacion esta incompleta]
- Severidad realista: Informativa
- Probabilidad: Media
- Impacto: Bajo
- Area: Operacion
- Donde esta: `.github/workflows/ci.yml:13-103`
- Que ocurre exactamente: el CI ya hace `go vet`, tests con race, build, frontend test/build y Trivy sobre imagenes Docker. Eso esta bien para este contexto. Lo que falta es una verificacion mas directa del ecosistema Go tipo `govulncheck`.
- Por que importa EN ESTE CONTEXTO: no es un bloqueo de lanzamiento, pero es una mejora barata y muy util para una app que depende de parsers y herramientas delicadas.
- Escenario de fallo o abuso concreto: arrastras una version vulnerable en alguna dependencia Go y no la detectas porque el pipeline solo mira imagenes o builds, no la base de datos de vulns especifica de Go.
- Como reproducirlo o comprobarlo: no existe paso `govulncheck` en `.github/workflows/ci.yml`. Intente correrlo localmente, pero la herramienta instalada no era compatible con el toolchain requerido por el repo.
- Recomendacion minima viable: anadir `govulncheck ./...` al job backend cuando la version de Go del runner y del binario esten alineadas.
- Recomendacion ideal: combinar `govulncheck`, Renovate/Dependabot y cadencia de updates de parches.
- Coste de implementacion: Bajo
- Prioridad sugerida: Proxima tanda

## 4. Lo que ya esta suficientemente bien para este proyecto

No todo requiere cambios. De hecho, varias piezas estan mejor que la media de una app personal expuesta a internet:

- Deteccion real del tipo de archivo por contenido, no por extension: `apps/api/internal/ingestion/detector.go:103-167`
- Validacion por familia y limites utiles: tamano, paginas PDF, duracion media, imagenes demasiado grandes, rechazo de protegidos/encriptados: `apps/api/internal/ingestion/validator.go:7-77`
- Upload en streaming a disco temporal, sin meter ficheros grandes completos en memoria: `apps/api/internal/api/handlers/upload.go:71-101`
- Validacion del artefacto de salida antes de persistirlo: `apps/api/internal/workers/output_validation.go:38-216`
- Sanitizado de HTML para reducir referencias remotas y scripts en conversiones HTML/PDF: `apps/api/internal/workers/document/html_sanitize.go:10-58`, `apps/api/internal/workers/document/to_pdf.go:13-32`, `apps/api/internal/workers/pdf/to_html.go:17-36`
- Los binarios externos se invocan con `exec.CommandContext` y arrays de argumentos, sin `sh -c` ni interpolacion shell directa. Eso reduce mucho el riesgo de command injection accidental.
- Hay control de acceso por propietario o guest-session en endpoints de capacidades, jobs y descargas: `apps/api/internal/api/handlers/authz.go:27-43`, `capabilities.go:25-43`, `job.go:31-140`, `artifact.go:22-49`
- El frontend ya envia cabeceras utiles y CSP basica: `apps/web/next.config.ts:26-66`
- Las cookies de sesion son `HttpOnly` y `SameSite=Lax`: `apps/api/internal/api/handlers/auth.go:145-165`
- El despliegue Docker esta endurecido de forma bastante buena para un proyecto personal: usuario no-root, `read_only`, `tmpfs`, `no-new-privileges`, `cap_drop`, limites de memoria/CPU/PIDs y red interna para workers: `apps/api/docker-compose.yml:53-122`, `apps/api/Dockerfile:19-46`, `apps/api/Dockerfile.worker:20-46`
- SQLite no me parece un problema por si mismo aqui. Para una sola maquina y trafico moderado es una decision valida y coherente con `docs/adr/0002-sqlite-production.md`. No lo cambiaria "por principio".

## 5. Priorizacion obligatoria

### 1. Imprescindible antes de exponerla a internet

- quitar el bootstrap admin desde registro publico
- desplegar solo con Redis + worker separado
- corregir la fuga de artefactos huerfanos cuando falla persistencia
- ajustar limites de abuso anonimo de forma algo mas dura
- corregir CORS si frontend y API iran en origenes distintos

### 2. Muy recomendable en el corto plazo

- comprobar espacio libre tambien antes de guardar artefactos
- evitar la lectura completa de SVG en memoria
- anadir `govulncheck` al CI
- poner timeout explicito en conexiones SMTP

### 3. Mejoras utiles pero no urgentes

- reducir PII en logs
- scavenger de directorios de artefactos sin registro
- modo de "server under pressure" para rechazar nuevas conversiones al borde de recursos

### 4. Overkill para este proyecto

- migrar ya a Kubernetes, Postgres o una cola distribuida compleja
- antivirus pesado o pipeline antimalware empresarial para cada upload
- sandboxing extremo tipo microVM por conversion para un hobby project
- WAF comercial, SIEM grande o compliance formal
- nonce-based CSP sofisticada en todo el frontend si antes no has resuelto los bloqueadores anteriores

## 6. Entregables

### A. Resumen ejecutivo

Estado general:

- mejor que la media de una app personal de conversion
- la base tecnica no esta improvisada
- ya hay buenas decisiones de validacion, ownership, limpieza y endurecimiento Docker

Nivel de riesgo actual:

- moderado
- no lo veo como "coladero", pero tampoco lo expondria tal como esta sin corregir algunos puntos concretos

Lo expondria a internet tal como esta:

- no, no exactamente

Bajo que condiciones minimas si lo haria:

- admin bootstrap cerrado
- Redis + worker separado de verdad
- fuga de artefactos huerfanos corregida
- limites anonimos algo mas estrictos
- CORS arreglado si web y API no comparten origen

### B. Top 10 problemas reales

1. El primer usuario registrado se vuelve admin.
2. El modo con worker embebido no debe usarse en publico.
3. Fallos al guardar artefactos pueden dejar basura fuera del retention.
4. Las escrituras de artefactos no comprueban espacio libre.
5. El CORS actual rompe `PUT` admin cross-origin.
6. Las cuotas anonimas son faciles de esquivar para abuso basico.
7. La extraccion de SVG lee archivos completos en memoria.
8. El envio SMTP no tiene deadlines explicitos.
9. Se registran emails completos en algunos logs.
10. Falta un escaneo de vulnerabilidades Go mas especifico en CI.

### C. Plan de accion de menor esfuerzo / mayor impacto

1. Cerrar el bootstrap admin desde el registro publico.
2. Forzar despliegue publico solo con Redis + worker separado.
3. Limpiar directorios de artefactos cuando falle `SaveArtifact` o `Artifacts.Create`.
4. Anadir chequeo de espacio libre antes de persistir artefactos.
5. Corregir CORS para `PUT` y cualquier otro metodo real del panel.
6. Endurecer cuotas guest y meter un rate limit basico tambien fuera del proceso.
7. Cambiar lectura SVG a prefijo acotado.
8. Anadir `govulncheck` al pipeline.
9. Poner timeout de red a SMTP.
10. Recortar PII en logs.

### D. Quick wins

- anadir `PUT` a CORS
- borrar `filepath.Dir(storagePath)` cuando falle la insercion del artefacto
- reutilizar `checkDiskSpace(...)` en `SaveArtifact`
- sustituir `os.ReadFile` por `io.LimitReader` en SVG
- impedir que el primer admin salga de `POST /api/auth/register`
- bajar algo las cuotas guest y los trabajos simultaneos por usuario

### E. Cambios que no merecen la pena ahora

- reescribir todo para Postgres
- meter antivirus enterprise por defecto
- introducir colas, buses y storage cloud mucho mas complejos sin necesidad real
- endurecer el frontend como si fuera una banca online cuando el cuello de botella real esta en uploads, workers y disco
- hacer una bateria enorme de controles de compliance que no cambia tu riesgo dominante

### F. Parches concretos

#### 1. CORS: permitir los metodos reales del panel

```diff
// apps/api/internal/api/middleware/cors.go
return cors.Handler(cors.Options{
    AllowedOrigins: origins,
-   AllowedMethods: []string{"GET", "POST", "OPTIONS"},
+   AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders: []string{"Content-Type", "X-Request-ID"},
    ExposedHeaders: []string{"X-Request-ID", "Content-Disposition"},
    AllowCredentials: true,
    MaxAge: 300,
})
```

#### 2. Bootstrap admin: no regalar admin al primer registro publico

Minimo aceptable:

```diff
// apps/api/internal/auth/service.go
count, err := s.users.Count(ctx)
if err != nil {
    return nil, err
}
-if count == 0 {
-    role = domain.RoleAdmin
-}
+if count == 0 {
+    return nil, domain.ErrBootstrapAdminRequired
+}
```

Luego crear el owner por semilla manual, comando CLI o variable de bootstrap temporal antes de abrir registros.

#### 3. Limpiar artefactos huerfanos cuando falle la persistencia

```diff
// apps/api/internal/storage/filesystem.go
func (fs *Filesystem) SaveArtifact(_ context.Context, artifactID string, fileName string, r io.Reader) (string, error) {
+   if err := checkDiskSpace(fs.basePath, minFreeDiskBytes); err != nil {
+       return "", err
+   }
    dir := filepath.Join(fs.basePath, "artifacts", artifactID)
    if err := os.MkdirAll(dir, 0o750); err != nil {
        return "", fmt.Errorf("create artifact dir: %w", err)
    }
    p := filepath.Join(dir, fileName)
-   return p, writeFile(p, r)
+   if err := writeFile(p, r); err != nil {
+       _ = os.RemoveAll(dir)
+       return "", err
+   }
+   return p, nil
}
```

Y ademas:

```diff
// apps/api/internal/workers/handler.go
if err := h.Artifacts.Create(ctx, &artifact); err != nil {
+   _ = os.RemoveAll(filepath.Dir(storagePath))
    return h.fail(ctx, jobID, logger, "persist artifact record", err)
}
```

#### 4. SVG: leer solo un prefijo pequeno

```diff
// apps/api/internal/ingestion/metadata.go
func extractSVGDimensions(path string) (w, h int, ok bool) {
-   data, err := os.ReadFile(path)
+   f, err := os.Open(path)
    if err != nil {
        return 0, 0, false
    }
-   content := string(data)
-   if len(content) > 4096 {
-       content = content[:4096]
-   }
+   defer f.Close()
+   data, err := io.ReadAll(io.LimitReader(f, 4096))
+   if err != nil {
+       return 0, 0, false
+   }
+   content := string(data)
    ...
}
```

#### 5. Despliegue: no exponer el compose de desarrollo

Minimo operativo:

- usar `docker compose --profile redis -f docker-compose.yml -f docker-compose.production.yml up -d`
- no publicar la app con el compose base tal cual
- mantener `worker` solo en `worker_internal` y la API publica separada, como ya plantea el repo

#### 6. Rate limiting proporcional

Minimo realista para este proyecto:

- mantener los rate limits aplicacion actuales
- bajar algo la cuota guest acumulada
- limitar trabajos activos anonimos o guest
- anadir un rate limit basico en proxy si lo tienes

Si despliegas detras de Nginx, algo simple ya ayuda:

```nginx
limit_req_zone $binary_remote_addr zone=reform_api:10m rate=10r/m;

server {
  client_max_body_size 110m;
  location /api/files {
    limit_req zone=reform_api burst=5 nodelay;
    proxy_pass http://api;
  }
}
```

Esto depende del stack de despliegue. No pude verificar tu proxy real en el repo.

## 7. Conclusiones cortas

Yo si la expondria a internet despues de una tanda pequena de endurecimiento, no despues de una reescritura. La app ya tiene una base razonable y varias decisiones correctas. El trabajo pendiente importante no es "enterprise hardening"; es cerrar 4 o 5 huecos muy concretos para que no te rompan el servicio ni te tomen el admin por una tonteria.

La prioridad practica es:

1. bootstrap admin
2. worker separado
3. limpieza y headroom de artefactos
4. abuso anonimo
5. CORS admin

Todo lo demas ya entra en iteracion sana, no en bloqueo de lanzamiento.
