# Auditoria Tecnica y de Seguridad

Fecha: 2026-04-09

## Alcance

Auditoria practica y proporcional de la web app de conversion de archivos, enfocada en dejarla en un estado razonablemente seguro y estable para exponerla a internet como proyecto personal.

Se revisaron:

- backend Go y API publica
- workers y ejecucion de motores de conversion
- almacenamiento local y persistencia en SQLite
- frontend Next.js y manejo de token
- Dockerfiles y `docker-compose.yml`
- variables de entorno documentadas en `.env`
- tests existentes, build del frontend y baseline de dependencias

Comprobaciones ejecutadas durante la revision:

- `go test ./...` en `apps/api`: pasa
- `npm audit --omit=dev --json` en `apps/web`: 0 vulnerabilidades de produccion reportadas
- `npm run build` en `apps/web`: pasa

Limitaciones de la auditoria:

- no se verifico un despliegue real detras de reverse proxy publico
- no se inspeccionaron logs reales de ejecucion en produccion
- `govulncheck` no estaba disponible, asi que no se validaron CVEs Go con esa herramienta
- no se probaron conversiones reales con corpus hostil extenso ni se confirmo el comportamiento de todos los binarios ante ficheros maliciosos

## Estado Tras La Pasada De Fixes

En esta pasada se implementaron cambios concretos sobre el codigo, excluyendo expresamente el tema del primer registro como admin.

Cambios aplicados:

- `JWT_SECRET` ahora es obligatorio, con longitud minima y sin permitir placeholders inseguros: [apps/api/config/config.go](apps/api/config/config.go), [apps/api/config/config_test.go](apps/api/config/config_test.go)
- la subida ya no bufferiza todo el archivo en memoria; ahora usa staging en temporal y valida antes de persistir el original: [apps/api/internal/api/handlers/upload.go](apps/api/internal/api/handlers/upload.go)
- se anadio un limite basico de megapixeles para frenar imagenes desproporcionadas: [apps/api/internal/ingestion/validator.go](apps/api/internal/ingestion/validator.go)
- la cola embebida ya no lanza goroutines sin limite; ahora tiene concurrencia acotada: [apps/api/internal/queue/inprocess.go](apps/api/internal/queue/inprocess.go), [apps/api/cmd/server/main.go](apps/api/cmd/server/main.go)
- se anadio limpieza periodica de originales y temporales locales: [apps/api/internal/storage/cleanup.go](apps/api/internal/storage/cleanup.go), [apps/api/internal/storage/cleanup_test.go](apps/api/internal/storage/cleanup_test.go)
- `/metrics` queda deshabilitado por defecto y la confianza en `X-Forwarded-For` pasa a ser opt-in: [apps/api/internal/api/router.go](apps/api/internal/api/router.go), [apps/api/internal/api/middleware/ratelimit.go](apps/api/internal/api/middleware/ratelimit.go)
- se endurecieron los limites de rate limiting globales y de endpoints sensibles: [apps/api/internal/api/router.go](apps/api/internal/api/router.go)
- los errores del motor ya no devuelven `stderr` bruto al usuario: [apps/api/internal/workers/handler.go](apps/api/internal/workers/handler.go)
- la cancelacion ya corta tambien la ejecucion del motor durante `running` mediante un contexto cancelable que observa el estado del job y tumba el proceso hijo cuando pasa a `cancelled`: [apps/api/internal/workers/handler.go](apps/api/internal/workers/handler.go), [apps/api/internal/workers/handler_cancel_test.go](apps/api/internal/workers/handler_cancel_test.go)
- el frontend ahora envia CSP y headers de seguridad desde Next.js: [apps/web/next.config.ts](apps/web/next.config.ts)
- el frontend deja de persistir la sesion en `localStorage` y toda la autenticacion del proyecto queda en cookie `HttpOnly` de sesion con `credentials: include`; `Bearer` deja de aceptarse en middleware y login/register ya no exponen `token` en JSON: [apps/api/internal/api/handlers/auth.go](apps/api/internal/api/handlers/auth.go), [apps/api/internal/api/middleware/auth.go](apps/api/internal/api/middleware/auth.go), [apps/api/internal/api/middleware/auth_test.go](apps/api/internal/api/middleware/auth_test.go), [apps/api/internal/api/middleware/cors.go](apps/api/internal/api/middleware/cors.go), [apps/web/src/lib/auth.ts](apps/web/src/lib/auth.ts), [apps/web/src/lib/auth-context.tsx](apps/web/src/lib/auth-context.tsx), [apps/web/src/lib/api.ts](apps/web/src/lib/api.ts)
- `/api/health` se reduce a un check publico minimo y el detalle operativo pasa a `/api/admin/health`: [apps/api/internal/api/handlers/health.go](apps/api/internal/api/handlers/health.go), [apps/api/internal/api/router.go](apps/api/internal/api/router.go), [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go)
- se cierra el uso anonimo en las rutas caras: upload, capacidades, conversiones, jobs y descarga de artefactos exigen autenticacion; los recursos legacy sin propietario dejan de ser accesibles salvo para admin y la UI ya no ofrece conversion sin cuenta: [apps/api/internal/api/router.go](apps/api/internal/api/router.go), [apps/api/internal/api/handlers/authz.go](apps/api/internal/api/handlers/authz.go), [apps/api/internal/api/handlers/authz_test.go](apps/api/internal/api/handlers/authz_test.go), [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go), [apps/web/src/components/conversion-card.tsx](apps/web/src/components/conversion-card.tsx)
- se añaden cuotas basicas por usuario autenticado para upload y conversion/retry, y un limite duro de jobs activos por usuario (`queued` + `running`) aplicado desde orquestacion para no trasladar el abuso al plano autenticado: [apps/api/internal/api/middleware/ratelimit.go](apps/api/internal/api/middleware/ratelimit.go), [apps/api/internal/api/middleware/ratelimit_test.go](apps/api/internal/api/middleware/ratelimit_test.go), [apps/api/internal/api/router.go](apps/api/internal/api/router.go), [apps/api/internal/orchestrator/service.go](apps/api/internal/orchestrator/service.go), [apps/api/internal/orchestrator/service_test.go](apps/api/internal/orchestrator/service_test.go), [apps/api/internal/repository/job_repo.go](apps/api/internal/repository/job_repo.go), [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go), [apps/api/config/config.go](apps/api/config/config.go), [apps/api/config/config_test.go](apps/api/config/config_test.go), [.env](.env), [.env.example](.env.example)
- el runtime de contenedores gana limites declarativos razonables de memoria, CPU, PIDs, `read_only`, `tmpfs` y `cap_drop`, y el worker standalone hace configurable su concurrencia: [apps/api/docker-compose.yml](apps/api/docker-compose.yml), [.env](.env), [.env.example](.env.example), [apps/api/config/config.go](apps/api/config/config.go), [apps/api/cmd/worker/main.go](apps/api/cmd/worker/main.go)
- el worker y Redis quedan en una red Compose interna sin egress publico ni puerto expuesto de Redis; la API solo entra a esa red para hablar con la cola: [apps/api/docker-compose.yml](apps/api/docker-compose.yml)
- se amplio la cobertura para validar sesion por cookie, retirada de `Bearer`, health detallada de admin, cuotas por usuario, limite de jobs activos y cancelacion durante `running`: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go), [apps/api/internal/api/middleware/auth_test.go](apps/api/internal/api/middleware/auth_test.go), [apps/api/internal/api/middleware/ratelimit_test.go](apps/api/internal/api/middleware/ratelimit_test.go), [apps/api/internal/workers/handler_cancel_test.go](apps/api/internal/workers/handler_cancel_test.go)

Validacion tras esta pasada:

- `go test ./...` en `apps/api`: pasa
- `npm run build` en `apps/web`: pasa
- `docker compose config` sobre `apps/api/docker-compose.yml`: pasa

No cambiado por peticion expresa:

- bootstrap del primer admin en el primer registro

Pendientes principales tras esta pasada:

- el primer registro sigue pudiendo convertirse en admin mientras no se cambie ese flujo
- las cuotas por usuario son locales al proceso y en memoria; si se despliega en varias instancias, tocara moverlas a un store compartido o a un gateway externo
- el worker ya no tiene salida publica de red en Compose, pero sigue sin un sandbox mas duro tipo gVisor/microVM ni una politica de egress mas fina fuera de ese despliegue
- sigue faltando corpus hostil y pruebas de integracion sobre conversiones reales y cleanup agresivo frente a fallos no simulados

### Nota Sobre El Punto 1: Modelo De Uso Anonimo

Se cerro en esta pasada por el camino conservador.

Decision tomada:

- exigir autenticacion para upload, consulta de capacidades, creacion de conversiones, estado/cancelacion/retry de jobs y descarga de artefactos
- retirar `Bearer` del middleware y dejar login/register sin `token` en JSON, manteniendo el JWT solo como contenido de la cookie `HttpOnly` de sesion
- bloquear acceso general a recursos legacy con `ownerId = nil`; solo admin puede inspeccionarlos
- aplicar cuotas por usuario autenticado y un limite de jobs activos para contener abuso economico basico

Motivo: es la opcion con menos superficie, menos coste operativo y menos riesgo de abuso economico para una app personal expuesta a internet. El camino mixto con anonimo + tokens efimeros + cuotas ya no queda abierto como decision pendiente; si algun dia se quisiera reabrir, tendria que entrar como feature deliberada y no como default.

## 1. Como Entiendo La Arquitectura

- El frontend es una app Next.js que consume una API Go y ahora usa una cookie `HttpOnly` de sesion para el navegador del proyecto.
- La API publica expone registro, login, logout y health minima; las rutas de archivos, capacidades, conversiones, jobs, descargas y dashboards requieren autenticacion, con area admin separada.
- El backend detecta el tipo real del archivo por contenido con `mimetype`, persiste el original en filesystem y registra metadatos en SQLite.
- La logica de capacidades esta centralizada y decide si un archivo puede convertirse y con que motor.
- Los jobs se orquestan desde la API y se ejecutan en workers usando motores externos como `ffmpeg`, `libreoffice`, `pdftoppm` y `pdftotext`.
- El sistema puede funcionar con Redis + worker separado o, si `REDIS_URL` esta vacio, en modo embebido dentro de la propia API.
- Los artefactos tienen TTL y existe un proceso de retencion para purgar artefactos expirados.

Referencias principales:

- [apps/api/internal/api/router.go](apps/api/internal/api/router.go)
- [apps/api/cmd/server/main.go](apps/api/cmd/server/main.go)
- [apps/api/cmd/worker/main.go](apps/api/cmd/worker/main.go)
- [apps/api/internal/ingestion/detector.go](apps/api/internal/ingestion/detector.go)
- [apps/api/internal/orchestrator/service.go](apps/api/internal/orchestrator/service.go)
- [apps/api/internal/workers/handler.go](apps/api/internal/workers/handler.go)
- [apps/api/internal/storage/filesystem.go](apps/api/internal/storage/filesystem.go)

## 2. Superficies De Ataque Y Puntos Debiles

Superficies principales:

- `POST /api/files`: upload autenticado de ficheros arbitrarios
- `GET /api/files/{fileId}/capabilities`: enumeracion autenticada de capacidades por archivo
- `POST /api/conversions`: creacion autenticada de jobs de conversion
- `GET /api/jobs/{jobId}` y `POST /api/jobs/{jobId}/cancel|retry`: gestion autenticada de jobs
- `GET /api/artifacts/{artifactId}/download`: descarga autenticada de artefactos
- `POST /api/auth/register` y `POST /api/auth/login`: alta y acceso publico
- `GET /metrics` y `GET /api/health`: exposicion de informacion operativa

Puntos debiles probables para este tipo de producto:

- abuso residual por cuentas o bots autenticados si las cuotas en memoria actuales resultan demasiado laxas o se despliega en varias instancias sin coordinacion
- consumo excesivo de RAM/CPU/disco durante ingesta o conversion
- binarios de terceros ejecutandose sobre archivos de internet
- conservacion innecesaria de originales y temporales
- bootstrap inseguro del primer administrador
- configuracion insegura por defaults de desarrollo
- falta de aislamiento suficiente entre API y trabajo pesado

## 3. Lo Que Ya Esta Razonablemente Bien

Estas piezas estan suficientemente bien para un proyecto personal y no requieren sobredisenar ahora mismo:

- Deteccion por contenido real, no por extension: [apps/api/internal/ingestion/detector.go](apps/api/internal/ingestion/detector.go)
- Sanitizacion del nombre original para display: [apps/api/internal/security/sanitize.go](apps/api/internal/security/sanitize.go)
- Uso de `exec.CommandContext` en lugar de shell: por ejemplo [apps/api/internal/workers/video/convert.go#L30](apps/api/internal/workers/video/convert.go#L30)
- Queries SQL parametrizadas y sin señales de SQL injection directa: [apps/api/internal/repository/file_repo.go](apps/api/internal/repository/file_repo.go), [apps/api/internal/repository/job_repo.go](apps/api/internal/repository/job_repo.go), [apps/api/internal/repository/artifact_repo.go](apps/api/internal/repository/artifact_repo.go)
- Ownership consistente para recursos autenticados: [apps/api/internal/api/handlers/authz.go#L23](apps/api/internal/api/handlers/authz.go#L23)
- Timeouts HTTP y logging estructurado: [apps/api/cmd/server/main.go](apps/api/cmd/server/main.go), [apps/api/internal/api/middleware/logging.go](apps/api/internal/api/middleware/logging.go)
- TTL de artefactos ya modelado: [apps/api/internal/orchestrator/retention.go](apps/api/internal/orchestrator/retention.go)

Nota de lectura: algunos hallazgos detallados mas abajo describen el estado original observado durante la auditoria. Los temas de `localStorage`, health publica demasiado verbosa, cancelacion parcial y limites runtime del worker quedaron mitigados total o parcialmente en la pasada actual descrita arriba.

## 4. Hallazgos Detallados

### JWT por defecto deja la autenticacion falsificable
- Severidad realista: Alta
- Probabilidad: Media
- Impacto: Alto
- Area: Seguridad
- Donde esta: [apps/api/config/config.go#L73](apps/api/config/config.go#L73), [apps/api/config/config.go#L75](apps/api/config/config.go#L75), [.env#L15](.env#L15), [apps/api/docker-compose.yml#L28](apps/api/docker-compose.yml#L28)
- Que ocurre exactamente: si `JWT_SECRET` no se define o se deja el valor por defecto, la API firma y valida tokens con un secreto conocido.
- Por que importa EN ESTE CONTEXTO: aunque no sea un producto enterprise, un error de despliegue dejaria la autenticacion practicamente rota.
- Escenario de fallo o abuso concreto: un atacante genera un JWT HS256 valido con rol `admin` usando `dev-secret-change-me` y accede a endpoints protegidos.
- Como reproducirlo o comprobarlo: arrancar con el `.env` actual, registrar un usuario y validar que un token firmado con el secreto por defecto es aceptado por `/api/auth/me` o rutas admin.
- Recomendacion minima viable: abortar el arranque si el secreto esta vacio, es corto o coincide con el valor de desarrollo.
- Recomendacion ideal: cargar el secreto solo desde entorno de despliegue y eliminar cualquier fallback funcional.
- Coste de implementacion: Bajo
- Prioridad sugerida: Ahora

### El primer registro publico se convierte en administrador
- Severidad realista: Alta
- Probabilidad: Alta
- Impacto: Alto
- Area: Seguridad / Operacion
- Donde esta: [apps/api/internal/auth/service.go#L62](apps/api/internal/auth/service.go#L62)
- Que ocurre exactamente: el primer usuario registrado en una base vacia recibe rol `admin` automaticamente.
- Por que importa EN ESTE CONTEXTO: en un despliegue personal con DB nueva, cualquiera puede adelantarse y apropiarse del panel de administracion.
- Escenario de fallo o abuso concreto: despliegas con base vacia y un bot se registra antes que tu.
- Como reproducirlo o comprobarlo: borrar la base, llamar a `POST /api/auth/register` y observar que el primer usuario sale con rol admin en `/api/auth/me`.
- Recomendacion minima viable: bootstrap explicito del primer admin mediante email permitido, token de instalacion o variable de entorno de setup.
- Recomendacion ideal: desactivar el registro abierto tras bootstrap o pasar a invitaciones.
- Coste de implementacion: Bajo
- Prioridad sugerida: Ahora

### La subida bufferiza archivos gigantes en memoria y los persiste antes de validar
- Severidad realista: Alta
- Probabilidad: Alta
- Impacto: Alto
- Area: Seguridad / Robustez / Rendimiento
- Donde esta: [apps/api/internal/api/handlers/upload.go#L46](apps/api/internal/api/handlers/upload.go#L46), [apps/api/internal/api/handlers/upload.go#L47](apps/api/internal/api/handlers/upload.go#L47), [apps/api/internal/api/handlers/upload.go#L70](apps/api/internal/api/handlers/upload.go#L70), [apps/api/internal/api/handlers/upload.go#L81](apps/api/internal/api/handlers/upload.go#L81), [apps/api/internal/workers/image/convert.go#L29](apps/api/internal/workers/image/convert.go#L29), [apps/api/internal/workers/image/to_pdf.go#L31](apps/api/internal/workers/image/to_pdf.go#L31)
- Que ocurre exactamente: el upload copia el archivo entero a memoria en un `bytes.Buffer` y solo despues detecta, guarda y valida. Algunos flujos de imagen decodifican el bitmap completo en RAM.
- Por que importa EN ESTE CONTEXTO: un atacante trivial puede usar pocos uploads grandes o imagenes desproporcionadas para agotar memoria en un servidor pequeno.
- Escenario de fallo o abuso concreto: varias subidas de 300-500 MB o imagenes comprimidas con dimensiones absurdas disparan el RSS del proceso y degradan toda la API.
- Como reproducirlo o comprobarlo: lanzar varias subidas concurrentes grandes y observar memoria del proceso, o usar una imagen con resolucion extrema.
- Recomendacion minima viable: hacer streaming a un temporal, detectar con un prefijo de bytes, validar antes de mover al storage definitivo y limitar tambien resolucion/paginas/duracion.
- Recomendacion ideal: staging asincrono con cuotas por IP/usuario y limites logicos por familia, no solo por bytes.
- Coste de implementacion: Medio
- Prioridad sugerida: Ahora

### Los originales y rechazos se quedan en disco demasiado tiempo
- Severidad realista: Alta
- Probabilidad: Alta
- Impacto: Alto
- Area: Privacidad / Operacion
- Donde esta: [apps/api/internal/api/handlers/upload.go#L70](apps/api/internal/api/handlers/upload.go#L70), [apps/api/internal/api/handlers/upload.go#L81](apps/api/internal/api/handlers/upload.go#L81), [apps/api/internal/storage/filesystem.go#L31](apps/api/internal/storage/filesystem.go#L31), [apps/api/internal/storage/filesystem.go#L86](apps/api/internal/storage/filesystem.go#L86), [apps/api/internal/storage/filesystem.go#L94](apps/api/internal/storage/filesystem.go#L94), [apps/api/internal/orchestrator/retention.go#L40](apps/api/internal/orchestrator/retention.go#L40)
- Que ocurre exactamente: solo hay retencion para artefactos. Los originales no tienen TTL ni purga periodica. Los ficheros rechazados despues de guardarse no se eliminan. Los temporales solo se limpian si el worker llega al `defer` de limpieza.
- Por que importa EN ESTE CONTEXTO: guardar originales indefinidamente no es una buena politica minima para una app publica de ficheros.
- Escenario de fallo o abuso concreto: subes un PDF cifrado o corrupto, el usuario recibe rechazo, pero el original sigue viviendo en `/data/originals`. Un crash del worker deja basura en `/data/temp`.
- Como reproducirlo o comprobarlo: subir un archivo protegido o matar el proceso durante una conversion y revisar el disco.
- Recomendacion minima viable: borrar el fichero guardado cuando `ValidateFile` falle y anadir limpieza periodica de originales y temporales por antiguedad.
- Recomendacion ideal: politicas separadas para originales, temporales y artefactos con trazabilidad de expiracion.
- Coste de implementacion: Medio
- Prioridad sugerida: Ahora

### El modo por defecto ejecuta conversiones pesadas dentro de la API y sin backpressure
- Severidad realista: Alta
- Probabilidad: Alta
- Impacto: Alto
- Area: Robustez / Rendimiento
- Donde esta: [apps/api/cmd/server/main.go#L84](apps/api/cmd/server/main.go#L84), [apps/api/cmd/server/main.go#L111](apps/api/cmd/server/main.go#L111), [apps/api/cmd/server/main.go#L114](apps/api/cmd/server/main.go#L114), [apps/api/internal/queue/inprocess.go#L37](apps/api/internal/queue/inprocess.go#L37)
- Que ocurre exactamente: si `REDIS_URL` esta vacio, la API arranca con worker embebido y cada job se lanza en una goroutine sin limite de concurrencia.
- Por que importa EN ESTE CONTEXTO: este es el modo mas probable de despliegue rapido y precisamente el mas facil de saturar.
- Escenario de fallo o abuso concreto: varios videos o PDFs grandes en paralelo se comen CPU y RAM y la API deja de responder con normalidad.
- Como reproducirlo o comprobarlo: arrancar con la `.env` actual y lanzar varias conversiones concurrentes; no hay semaforo, cola real ni backpressure.
- Recomendacion minima viable: no exponer este modo a internet o introducir un pool con concurrencia fija muy baja.
- Recomendacion ideal: API y worker separados con Redis o cola equivalente y observabilidad del backlog.
- Coste de implementacion: Medio
- Prioridad sugerida: Ahora

### El uso anonimo permitia abuso facil y acceso por UUID a recursos sin propietario
- Severidad realista: Alta
- Probabilidad: Alta
- Impacto: Medio-Alto
- Area: Seguridad / Operacion
- Donde esta: [apps/api/internal/api/router.go#L62](apps/api/internal/api/router.go#L62), [apps/api/internal/api/middleware/auth.go#L62](apps/api/internal/api/middleware/auth.go#L62), [apps/api/internal/api/handlers/authz.go#L23](apps/api/internal/api/handlers/authz.go#L23), [apps/api/internal/api/handlers/job.go#L36](apps/api/internal/api/handlers/job.go#L36), [apps/api/internal/api/handlers/artifact.go#L35](apps/api/internal/api/handlers/artifact.go#L35)
- Estado actual: mitigado en la pasada actual. Las rutas caras exigen autenticacion y los recursos legacy con `ownerId = nil` solo quedan visibles para admin.
- Que ocurria exactamente: upload, capacidades, conversiones, estado y descarga podian funcionar sin login. Si el recurso tenia `ownerId = nil`, cualquiera con el UUID podia usarlo.
- Por que importaba EN ESTE CONTEXTO: el riesgo mas realista no era un exploit sofisticado; era gente usando tu servicio gratis o accediendo a un recurso anonimo cuyo ID se filtraba.
- Escenario historico de fallo o abuso: un bot creaba conversiones anonimas en bucle o un tercero descargaba un artefacto anonimo si obtenia el `artifactId`.
- Comprobacion actual: [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go) ya valida que upload anonimo devuelve `401`, y [apps/api/internal/api/handlers/authz_test.go](apps/api/internal/api/handlers/authz_test.go) fija que un recurso legacy sin propietario no queda abierto a usuarios normales.
- Estado actual: mitigado tambien en el plano autenticado con cuotas basicas por usuario en upload/conversion/retry y un tope de jobs activos por usuario.
- Recomendacion ideal: si algun dia se reabre anonimo, hacerlo con tokens efimeros y quotas por recurso, no reutilizando el modelo antiguo.
- Coste de implementacion: Medio
- Prioridad sugerida: Ahora

### Los workers con binarios externos seguian cortos de aislamiento operativo minimo
- Severidad realista: Alta
- Probabilidad: Media
- Impacto: Alto
- Area: Seguridad / Operacion
- Donde esta: [apps/api/internal/workers/pdf/to_docx.go#L15](apps/api/internal/workers/pdf/to_docx.go#L15), [apps/api/internal/workers/video/convert.go#L30](apps/api/internal/workers/video/convert.go#L30), [apps/api/Dockerfile#L14](apps/api/Dockerfile#L14), [apps/api/Dockerfile.worker#L15](apps/api/Dockerfile.worker#L15), [apps/api/docker-compose.yml#L39](apps/api/docker-compose.yml#L39), [apps/api/cmd/worker/main.go#L128](apps/api/cmd/worker/main.go#L128)
- Estado actual: mitigado parcialmente. Ya hay `read_only`, `tmpfs`, `cap_drop`, `no-new-privileges`, limites de CPU/RAM/PIDs y red interna de Compose para worker/Redis sin puerto publico de Redis.
- Que ocurria exactamente: se ejecutaban parsers y convertidores complejos sobre archivos de internet con contencion insuficiente y sin restriccion de red visible.
- Por que importa EN ESTE CONTEXTO: no necesitas una microVM desde el dia 1, pero si un minimo de contencion porque `libreoffice` y `ffmpeg` tienen superficie real de ataque.
- Escenario residual de fallo o abuso: un bug serio en un parser seguiria ejecutandose dentro del contenedor del worker y tendria acceso al volumen compartido y a la red interna que necesite para la cola.
- Como comprobar el estado actual: revisar [apps/api/docker-compose.yml](apps/api/docker-compose.yml); el worker queda solo en `worker_internal`, red marcada `internal: true`, y Redis ya no expone puerto al host.
- Recomendacion minima viable que queda: endurecer mas el runtime real de produccion fuera de Compose o meter un sandbox mas duro si el nivel de riesgo cambia.
- Recomendacion ideal: sandbox mas fuerte para workers y politica deny-by-default de red tambien fuera del entorno Compose local.
- Coste de implementacion: Medio
- Prioridad sugerida: Ahora
- Posible hallazgo: no pude verificar si `libreoffice` o algun parser resuelve recursos remotos incrustados; si lo hace, la falta de restriccion de red lo convertiria en vector SSRF o relay.

### Cancelar un job en ejecucion no cancela realmente el trabajo
- Severidad realista: Media
- Probabilidad: Media
- Impacto: Medio
- Area: Robustez
- Donde esta: [apps/api/internal/api/handlers/job.go#L64](apps/api/internal/api/handlers/job.go#L64), [apps/api/internal/orchestrator/service.go#L125](apps/api/internal/orchestrator/service.go#L125), [apps/api/internal/workers/handler.go#L155](apps/api/internal/workers/handler.go#L155), [apps/api/internal/domain/job.go#L44](apps/api/internal/domain/job.go#L44), [apps/api/internal/api/api_e2e_test.go#L524](apps/api/internal/api/api_e2e_test.go#L524)
- Que ocurre exactamente: `CancelJob` cambia el estado del job en base de datos, pero el worker no consulta esa cancelacion durante la ejecucion ni mata el proceso externo.
- Por que importa EN ESTE CONTEXTO: te hace perder CPU justo cuando intentas frenar trabajo innecesario y abre la puerta a estados inconsistentes.
- Escenario de fallo o abuso concreto: cancelas una conversion larga de video, pero el proceso `ffmpeg` sigue corriendo y consume recursos igual.
- Como reproducirlo o comprobarlo: ejecutar una conversion lenta real y cancelarla durante `running`; observar que el proceso no se detiene.
- Recomendacion minima viable: introducir checkpoints de cancelacion antes de guardar artefacto y antes de marcar exito.
- Recomendacion ideal: cancelacion propagada desde la cola al contexto del proceso hijo y tests de integracion reales.
- Coste de implementacion: Medio
- Prioridad sugerida: Proxima tanda

### El rate limiting es facil de esquivar y demasiado laxo para endpoints caros
- Severidad realista: Media
- Probabilidad: Alta
- Impacto: Medio-Alto
- Area: Seguridad / Operacion
- Donde esta: [apps/api/internal/api/router.go#L45](apps/api/internal/api/router.go#L45), [apps/api/internal/api/middleware/ratelimit.go#L74](apps/api/internal/api/middleware/ratelimit.go#L74), [apps/api/internal/api/middleware/ratelimit.go#L26](apps/api/internal/api/middleware/ratelimit.go#L26)
- Que ocurre exactamente: el limite es global, en memoria, por IP, con `100 req/s` y burst `200`, y confia directamente en `X-Forwarded-For` y `X-Real-IP`.
- Por que importa EN ESTE CONTEXTO: para una app de conversion, upload y conversion necesitan un trato mucho mas duro que endpoints ligeros.
- Escenario de fallo o abuso concreto: un atacante rota `X-Forwarded-For` para esquivar buckets o satura con menos requests pero mas costosas.
- Como reproducirlo o comprobarlo: repetir requests cambiando `X-Forwarded-For`; el limiter asigna buckets distintos.
- Recomendacion minima viable: confiar en cabeceras de proxy solo detras de un proxy de confianza, bajar mucho los limites de auth/upload/conversion y anadir cuota por jobs activos.
- Recomendacion ideal: rate limit en proxy + app con presupuesto por IP/usuario.
- Coste de implementacion: Bajo-Medio
- Prioridad sugerida: Ahora

### La cobertura de casos hostiles y del flujo real de conversion es insuficiente
- Severidad realista: Media
- Probabilidad: Alta
- Impacto: Medio
- Area: Arquitectura / Robustez
- Donde esta: [docs/testing/test-strategy.md](docs/testing/test-strategy.md), [apps/api/internal/api/api_e2e_test.go](apps/api/internal/api/api_e2e_test.go), [apps/web/package.json](apps/web/package.json)
- Que ocurre exactamente: los tests basicos pasan, pero faltan pruebas y fixtures para archivos corruptos, protegidos, excesivos, limpieza tras fallos, cancelacion en `running` y frontend critico. En web no hay test runner definido.
- Por que importa EN ESTE CONTEXTO: en una app publica de ficheros, los fallos costosos suelen venir del error path, no del happy path.
- Escenario de fallo o abuso concreto: una regresion en cleanup, cancelacion o ownership de anonimos llega a produccion sin que ningun test la detecte.
- Como reproducirlo o comprobarlo: `go test ./...` pasa, pero varios paquetes criticos aparecen sin tests especificos de integracion real; el frontend compila pero no tiene pruebas funcionales.
- Recomendacion minima viable: anadir un mini corpus hostil y 4-5 pruebas de integracion de upload invalido, cancelacion real, cleanup y acceso cruzado.
- Recomendacion ideal: corpus real por formato y verificacion end-to-end del flujo Docker/worker.
- Coste de implementacion: Medio
- Prioridad sugerida: Proxima tanda

### Se exponen demasiados detalles operativos y del motor al exterior
- Severidad realista: Media
- Probabilidad: Media
- Impacto: Medio
- Area: Seguridad / Operacion
- Donde esta: [apps/api/internal/api/router.go#L49](apps/api/internal/api/router.go#L49), [apps/api/internal/api/router.go#L53](apps/api/internal/api/router.go#L53), [apps/api/internal/workers/handler.go#L192](apps/api/internal/workers/handler.go#L192)
- Que ocurre exactamente: `/metrics` y `/api/health` son publicos. Ademas, algunos errores visibles al usuario incluyen `stderr` de motores de conversion.
- Por que importa EN ESTE CONTEXTO: no te compromete por si solo, pero da informacion innecesaria y mejora la ergonomia del atacante o del abusador.
- Escenario de fallo o abuso concreto: un usuario obtiene mensajes detallados de `ffmpeg` o `libreoffice`; un tercero enumera salud, feature flags o metricas publicas.
- Como reproducirlo o comprobarlo: visitar `/metrics`, `/api/health` y forzar errores en conversiones.
- Recomendacion minima viable: quitar `/metrics` de internet, reducir informacion de health publica y limpiar errores del motor de cara al usuario.
- Recomendacion ideal: separar health publica de health interna y dejar metricas solo en red interna.
- Coste de implementacion: Bajo
- Prioridad sugerida: Proxima tanda

### El frontend mantiene el JWT en `localStorage` y no define CSP propia
- Severidad realista: Baja-Media
- Probabilidad: Media
- Impacto: Medio
- Area: Seguridad / Frontend
- Donde esta: [apps/web/src/lib/auth.ts#L56](apps/web/src/lib/auth.ts#L56), [apps/web/src/lib/auth.ts#L61](apps/web/src/lib/auth.ts#L61), [apps/web/src/lib/auth.ts#L65](apps/web/src/lib/auth.ts#L65), [apps/web/next.config.ts#L3](apps/web/next.config.ts#L3)
- Que ocurre exactamente: el token se guarda en `localStorage` y no hay cabeceras de seguridad definidas desde Next, especialmente CSP.
- Por que importa EN ESTE CONTEXTO: no es tu principal riesgo hoy porque no vi sinks claros de XSS, pero si apareciera un XSS en el futuro, el impacto sobre la sesion seria mayor.
- Escenario de fallo o abuso concreto: cualquier XSS futuro podria leer y exfiltrar el JWT.
- Como reproducirlo o comprobarlo: revisar el codigo de auth y la ausencia de `headers()` en `next.config.ts`.
- Recomendacion minima viable: anadir CSP estricta, evitar `dangerouslySetInnerHTML` y mantener auditado el frontend.
- Recomendacion ideal: pasar a sesion con cookie `HttpOnly` si mas adelante quieres endurecer auth.
- Coste de implementacion: Bajo-Medio
- Prioridad sugerida: Mas adelante

## 5. Priorizacion Obligatoria

### 1. Imprescindible antes de exponerla a internet

- Forzar `JWT_SECRET` real y bloquear el arranque con valores por defecto.
- Cerrar el bootstrap automatico del primer admin.
- Rehacer el upload para no bufferizar entero en memoria ni persistir antes de validar.
- Anadir limpieza y retencion real de originales, rechazos y temporales.
- No publicar el modo embebido sin limite de concurrencia; preferible worker separado.
- Endurecer el worker de forma minima: no root, limites de recursos y sin egress salvo necesidad.
- Bajar y endurecer rate limits en auth, upload y conversion.
- Revisar periodicamente si las cuotas por usuario y el tope de jobs activos siguen siendo suficientes para el uso real.

### 2. Muy recomendable en el corto plazo

- Corregir cancelacion real de jobs en ejecucion.
- Limpiar errores de motor visibles al usuario.
- Sacar `/metrics` de exposicion publica.
- Anadir tests de casos hostiles y de cancelacion real.

### 3. Mejoras utiles pero no urgentes

- CSP y cabeceras del frontend.
- Reducir informacion publica en `/api/health`.
- Cuotas por usuario y limites por familia mas finos.
- Resolver el warning de lockfiles multiples de Next para tener builds mas limpias.

### 4. Overkill para este proyecto

- WAF empresarial, SIEM o IDS avanzado desde el dia 1.
- Migracion inmediata a infraestructura compleja solo por postureo.
- Kubernetes, autoscaling complejo o segmentacion enterprise sin necesidad.
- Sandbox extremo tipo microVM antes de aplicar limites basicos y no root.
- Rehacer toda la autenticacion a refresh tokens y sesiones distribuidas antes de tapar los huecos fundamentales.

## A. Resumen Ejecutivo

- Estado general del proyecto: la base esta bastante ordenada y la arquitectura es razonable. El problema no es un diseno caotico, sino varios huecos practicos en el borde publico y en la ejecucion de conversiones.
- Nivel de riesgo actual: medio.
- Si la expondria a internet tal como esta: no.
- Bajo que condiciones minimas si lo haria: bootstrap de admin cerrado, cuotas por usuario/rate limiting mas duro en rutas caras, corpus hostil minimo, y despliegue detras de reverse proxy con TLS.

## B. Top 10 Problemas Reales

1. JWT por defecto deja la autenticacion falsificable.
2. El primer registro publico se convierte en administrador.
3. La subida bufferiza archivos gigantes en memoria y los persiste antes de validar.
4. Los originales y rechazos se quedan en disco demasiado tiempo.
5. El modo por defecto ejecuta conversiones pesadas dentro de la API y sin backpressure.
6. Las cuotas por usuario y el rate limiting siguen siendo locales al proceso y basicos para un despliegue multi-instancia.
7. Los workers siguen sin un sandbox mas duro que el contenedor endurecido actual.
8. La cobertura de casos hostiles y del flujo real de conversion es insuficiente.
9. Falta cerrar el bootstrap inseguro del primer admin antes de exponer el panel.
10. Sigue faltando validar con corpus real el comportamiento de binarios pesados bajo presion y errores no simulados.

## C. Plan De Accion De Menor Esfuerzo / Mayor Impacto

1. Forzar `JWT_SECRET` no default y cerrar bootstrap de admin.
2. Cerrar el bootstrap inseguro del primer admin.
3. Sustituir el upload bufferizado por streaming a temporal y borrar siempre el temporal/rechazo.
4. Publicar solo con worker separado o con un limite de concurrencia fijo y bajo.
5. Anadir limpieza periodica de originales y temporales.
6. Bajar limites y aplicar rate limiting especifico a auth, upload y conversion.
7. Endurecer o externalizar las cuotas por usuario si pasas a multi-instancia.
8. Evaluar si necesitas sandbox mas duro para el worker segun riesgo real del despliegue.
9. Anadir corpus hostil minimo y pruebas de integracion sobre rutas de error.
10. Cerrar el bootstrap del primer admin.

## D. Quick Wins

- Fallar al arrancar si `JWT_SECRET` es el de desarrollo.
- Eliminar la promocion automatica del primer usuario a admin.
- Sacar `/metrics` de internet.
- Reducir `100 req/s` y separar limites por endpoint.
- Limpiar el detalle de `stderr` que recibe el usuario.
- Activar el worker separado con Redis en despliegue publico.
- Meter un proceso simple de borrado para `/data/originals` y `/data/temp`.
- Cerrar bootstrap del primer admin.
- Ajustar cuotas por usuario autenticado si la telemetria real lo pide.

## E. Cambios Que No Merecen La Pena Ahora

- Montar un sistema complejo de antivirus antes de aislar bien el worker.
- Rehacer el storage a cloud object storage solo por imagen de arquitectura.
- Pasar ya mismo a cookies `HttpOnly` si antes no cierras los huecos de abuso y CSP.
- Desplegar un stack completo de tracing y alerting enterprise.
- Sandbox extremo antes de aplicar las defensas operativas basicas.

## F. Parches Concretos

### 1. Bloquear JWT insecure por defecto

Archivo objetivo: [apps/api/config/config.go](apps/api/config/config.go)

```go
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" || jwtSecret == "dev-secret-change-me" || len(jwtSecret) < 32 {
	return nil, fmt.Errorf("JWT_SECRET must be set to a non-default value with at least 32 chars")
}
```

### 2. Cerrar el bootstrap implicito del primer admin

Archivo objetivo: [apps/api/internal/auth/service.go](apps/api/internal/auth/service.go)

Ejemplo de aproximacion minima:

```go
role := domain.RoleUser

bootstrapAdminEmail := strings.TrimSpace(strings.ToLower(os.Getenv("BOOTSTRAP_ADMIN_EMAIL")))
if count == 0 {
	if bootstrapAdminEmail == "" || bootstrapAdminEmail != strings.ToLower(in.Email) {
		return nil, errors.New("first admin must be bootstrapped explicitly")
	}
	role = domain.RoleAdmin
}
```

### 3. Limitar concurrencia en modo embebido si no puedes quitarlo ya

Archivo objetivo: [apps/api/internal/queue/inprocess.go](apps/api/internal/queue/inprocess.go)

```go
type InProcessQueue struct {
	handler TaskHandler
	wg      sync.WaitGroup
	sem     chan struct{}
}

func NewInProcessQueue(handler TaskHandler, maxConcurrent int) *InProcessQueue {
	return &InProcessQueue{
		handler: handler,
		sem:     make(chan struct{}, maxConcurrent),
	}
}

func (q *InProcessQueue) Enqueue(ctx context.Context, taskType string, payload TaskPayload, opts TaskOptions) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	if q.handler == nil {
		return nil
	}

	q.wg.Add(1)
	q.sem <- struct{}{}
	go func() {
		defer q.wg.Done()
		defer func() { <-q.sem }()
		taskCtx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
		_ = q.handler(taskCtx, taskType, data)
	}()

	return nil
}
```

### 4. No exponer `stderr` bruto del motor al usuario

Archivo objetivo: [apps/api/internal/workers/handler.go](apps/api/internal/workers/handler.go)

```go
case containsAny(errText, "exit status"):
	return "El motor de conversion no pudo procesar este archivo."
```

### 5. Politica minima razonable de reverse proxy

Esto depende del despliegue exacto, pero una configuracion proporcional deberia incluir:

- TLS en el proxy
- limite de body mas bajo que el maximo del backend si no quieres aceptar archivos gigantes
- rate limiting en proxy para auth y uploads
- `metrics` no accesible desde internet publica
- cabeceras de seguridad comunes

Ejemplo con Caddy:

```caddy
example.com {
	encode gzip

	@api path /api/*
	reverse_proxy @api api:8080
	reverse_proxy web:5050

	request_body {
		max_size 120MB
	}

	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		Referrer-Policy "strict-origin-when-cross-origin"
		Content-Security-Policy "default-src 'self'; connect-src 'self' https://example.com; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'"
	}
}
```

### 6. Limites operativos razonables para un proyecto personal

Recomendacion minima viable:

- Uploads: 50-120 MB salvo que realmente necesites mas
- Jobs activos simultaneos por IP anonima: 1-2
- Jobs activos simultaneos por usuario autenticado: 2-4
- Timeout maximo de conversion: mantener 30-300 s salvo video, y acotar aun mas anonimo
- TTL de artefactos: 12-48 h segun familia esta bien
- TTL de originales: corto, por ejemplo 1-24 h tras fin del job
- TTL de temporales: minutos u horas, con limpieza agresiva

## Conclusiones Finales

La app no esta lejos de un nivel razonable para internet publica, pero todavia no la pondria online tal como esta porque combina tres riesgos demasiado evitables:

- autenticacion facil de romper por defaults inseguros
- abuso operativo sencillo por uploads y conversiones anonimas
- aislamiento insuficiente de workers y almacenamiento

La buena noticia es que no hace falta una re-arquitectura enterprise para arreglarlo. Con una tanda corta y enfocada de endurecimiento en auth, upload, retencion, limites y despliegue, el proyecto puede quedar en un estado sensato para un servicio personal expuesto a internet.