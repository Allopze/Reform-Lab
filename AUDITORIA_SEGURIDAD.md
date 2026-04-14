# Auditoria tecnica y de seguridad

Fecha: 2026-04-14

## Alcance y supuestos

Esta auditoria parte de dos condiciones operativas que cambian la priorizacion:

- En produccion habra un reverse proxy bien configurado delante de la API y la web.
- Que el primer registro obtenga rol administrador es una politica deliberada, no un bug.

Con esos supuestos, no trato como hallazgo:

- la promocion del primer usuario a admin en un despliegue vacio
- el uso de cabeceras `X-Forwarded-For` y `X-Real-IP` siempre que el proxy las sobrescriba y no exponga la API directamente

## Resumen ejecutivo

La base tecnica es buena para una app personal publica. Hay varias decisiones correctas que reducen riesgo real:

- deteccion de formato por contenido y no por extension
- validacion de salidas de conversion antes de marcar exito
- CSP razonable en la web
- cookies HttpOnly con `SameSite=Lax`
- contenedores ya bastante endurecidos para el nivel del proyecto

No he visto una SQL injection evidente, ni una shell injection directa, ni un sink XSS obvio en frontend.

Los problemas mas relevantes no son de arquitectura global sino huecos concretos de implementacion y operacion.

## Como entiendo la arquitectura

La aplicacion tiene una web Next.js que consume una API en Go. La API recibe archivos, detecta el tipo real, extrae metadatos, decide capacidades permitidas y crea jobs asincronos. Los workers ejecutan conversiones con herramientas externas como LibreOffice, FFmpeg, Poppler, Ghostscript, Tesseract o `pdf2docx`. Los originales y artefactos se guardan en filesystem local y el estado vive en SQLite. Redis se usa para cola de jobs.

Flujo principal:

1. subida de archivo
2. validacion inicial y deteccion real de formato
3. extraccion de metadatos
4. resolucion de capacidades disponibles
5. creacion de job
6. ejecucion asincrona por worker
7. validacion del artefacto de salida
8. persistencia y descarga

## Superficies de ataque

- autenticacion y cookies de sesion
- subida multipart de archivos no confiables
- creacion de jobs, cuotas y rate limiting
- ejecucion de conversores externos en workers
- descargas de artefactos
- panel admin, SMTP y webhooks
- despliegue, TLS, proxy y exposicion de puertos

## Hallazgos

## Remediacion aplicada

### Pasada 1

- corregido el motor de migraciones para descubrir y aplicar automaticamente todos los archivos `.sql` presentes en el directorio de migraciones
- añadido test para evitar que vuelvan a quedarse migraciones fuera por mantener una lista manual incompleta
- el hallazgo de deriva entre migraciones existentes y migraciones aplicadas queda mitigado

### Pasada 2

- movido el limite de jobs activos para invitados desde un pre-check en handler a una comprobacion atomica en repositorio/orquestador
- eliminada la ventana de carrera mas obvia en creacion individual y batch para sesiones guest
- añadido test de repositorio para asegurar que el segundo job guest se rechaza al superar el limite

### Pasada 3

- añadida validacion compartida para webhooks que bloquea `localhost`, credenciales embebidas, IPs privadas, loopback, link-local, multicast, CGNAT y rangos de benchmark
- reforzada la entrega real del worker con validacion de resolucion DNS y cliente HTTP restringido para evitar conexiones salientes a destinos internos
- añadidos tests para rechazar destinos no permitidos y aceptar destinos publicos

### Pasada 4

- endurecidos los defaults del `docker-compose.yml` root para publicar `web` y `api` en loopback por defecto
- actualizada `.env.production.example` para asumir reverse proxy con TLS y URLs publicas `https://...`
- actualizado el README para documentar el modelo de produccion soportado y cuándo activar `TRUST_PROXY_HEADERS`

### Pasada 5

- sustituido `FormFile` por `MultipartReader` en la ruta de subida para evitar el parseo multipart con temporales implícitos antes del staging controlado
- reducida la presión innecesaria sobre `/tmp` y el doble manejo de temporales en el camino feliz de upload
- mantenido el staging explícito para detección, metadatos y validación antes de persistir el original

### Pasada 6

- añadida una capa de cifrado AES-GCM para secretos persistidos por panel admin
- el password SMTP ya no se guarda en claro cuando se actualiza desde admin
- los secretos de webhook ya no se guardan en claro en SQLite y se descifran en lectura para runtime
- la lectura sigue siendo compatible con valores legacy en claro, pero los nuevos persistidos exigen `SECRET_ENCRYPTION_KEY`

### Pasada 7

- endurecido el pipeline de SVG para sanear el contenido antes de entregarlo a `ffmpeg` o `rsvg-convert`
- reutilizada la lógica de neutralización de recursos remotos y eliminación de `foreignObject` ya usada en HTML/SVG sanitizado
- añadido test para comprobar que se eliminan recursos remotos peligrosos sin romper enlaces navegacionales legítimos

### Verificacion

- tests focalizados de secretos: OK (`internal/security`, `internal/email`, `internal/repository`, `internal/api`)
- tests focalizados de SVG: OK (`internal/workers/image`)
- suite completa backend: OK (`go test ./...` en `apps/api`)

### Imprescindible antes de exponerla a internet

#### 1. El ejemplo de despliegue sigue orientado a HTTP en claro

Estado actual:

- mitigado en esta sesion a nivel de plantilla y documentacion; el stack root y la plantilla de producción ya orientan a loopback + reverse proxy TLS

Aunque has indicado que en produccion usaras reverse proxy, el material de despliegue del repo sigue apuntando a publicar web y API por HTTP y escuchando en `0.0.0.0`.

Referencias:

- `docker-compose.yml` publica web y API en host: `0.0.0.0` por defecto
- `.env.production.example` usa `http://YOUR_SERVER_OR_DOMAIN` para `CORS_ORIGIN`, `NEXT_PUBLIC_API_URL` y `APP_URL`

Impacto:

- alto si alguien despliega siguiendo el ejemplo tal cual
- medio si el despliegue real ya va detras de TLS y proxy bien cerrados

Riesgo concreto:

- cookies no seguras en accesos no terminados en HTTPS
- credenciales y ficheros en claro si se expone sin proxy/TLS
- falsa sensacion de “production ready” en un operador que copie el ejemplo

Codigo relevante:

- `docker-compose.yml`
- `.env.production.example`
- `apps/api/internal/api/handlers/auth.go`
- `apps/api/internal/api/handlers/guest_session.go`

Recomendacion:

- cambiar los ejemplos de produccion para asumir proxy TLS delante
- documentar explicitamente que API no debe exponerse directa a internet
- dejar una variante de ejemplo con bind local para API cuando vaya detras de proxy

### Muy recomendable en el corto plazo

#### 2. Las migraciones de webhooks existen pero no se aplican en instalaciones nuevas

Estado actual:

- corregido en codigo en esta sesion; el motor ya no depende de una lista manual cerrada

El motor de migracion mantiene una lista manual que llega solo hasta `008_file_expired_at.sql`, pero en el repositorio existen `009_webhooks.sql` y `010_webhook_deliveries.sql`.

Referencias:

- `apps/api/internal/database/sqlite.go`
- `apps/api/migrations/009_webhooks.sql`
- `apps/api/migrations/010_webhook_deliveries.sql`

Impacto:

- alto en fiabilidad operativa
- medio en seguridad, porque deja una funcionalidad administrativa en estado inconsistente o rota en instalaciones limpias

Riesgo concreto:

- fallos al usar webhooks en nuevos entornos
- diferencias de comportamiento entre entornos viejos y nuevos
- deuda operativa dificil de detectar hasta que alguien intente usar la feature

Recomendacion:

- eliminar la lista manual o validarla contra el contenido real del directorio de migraciones
- añadir test que falle si existe una migracion no registrada

#### 3. El limite de jobs activos para invitados no es atomico

Estado actual:

- corregido en codigo en esta sesion; la limitacion guest ya no depende de contar y luego insertar desde el handler

Para invitados, el handler consulta el numero de jobs activos y luego crea el job. Ese control no queda protegido por una operacion transaccional equivalente a la que si existe para usuarios autenticados.

Referencias:

- `apps/api/internal/api/handlers/conversion.go`
- `apps/api/internal/repository/job_repo.go`

Impacto:

- medio-alto

Riesgo concreto:

- un invitado con peticiones concurrentes puede superar el limite configurado
- aumento evitable de carga en cola, workers y disco temporal
- degradacion de servicio sin necesidad de comprometer cuentas

Recomendacion:

- mover el limite de invitado a una comprobacion atomica en repositorio
- testear concurrencia real del caso guest

#### 4. Los webhooks permiten SSRF desde contexto admin

Estado actual:

- mitigado en codigo en esta sesion con validacion al configurar el webhook y defensa adicional durante la entrega del worker

La validacion de URL para webhooks acepta cualquier `http` o `https` con host no vacio. Despues el worker realiza la peticion saliente con un cliente HTTP normal.

Referencias:

- `apps/api/internal/api/handlers/webhook.go`
- `apps/api/internal/workers/webhook_handler.go`

Impacto:

- medio

Notas:

- no es una SSRF anonima; requiere control del panel admin o abuso interno
- aun asi merece correccion porque los webhooks son una primitiva de salida de red muy util para pivotar

Riesgo concreto:

- acceso a servicios internos si la red del contenedor lo permite
- sondeo de hosts internos o metadata endpoints
- exfiltracion indirecta de eventos o tokens de firma

Recomendacion:

- bloquear `localhost`, rangos privados, loopback, link-local y metadata IPs
- resolver DNS y revalidar destino antes de conectar
- si no necesitas flexibilidad total, usar allowlist explicita

### Mejoras utiles pero no urgentes

#### 5. La ruta de subida puede tensionar `/tmp` antes de que entren todas tus cuotas

Estado actual:

- parcialmente mitigado en esta sesion; ya no depende de `FormFile` ni del staging temporal implícito de multipart, aunque la cuota acumulada sigue comprobándose tras copiar al staging propio

La subida usa `FormFile`, lo que implica parseo multipart con temporales, y luego vuelve a copiar a un staging propio. En el compose principal el `tmpfs` de la API es de 256 MB.

Referencias:

- `apps/api/internal/api/handlers/upload.go`
- `docker-compose.yml`

Impacto:

- medio en robustez

Riesgo concreto:

- errores de subida o degradacion bajo concurrencia
- agotamiento de temporal antes de que las validaciones de negocio hagan su trabajo

Recomendacion:

- evitar depender de `FormFile` para ficheros grandes y hacer streaming mas directo al staging controlado
- revisar tamano de `tmpfs` frente a limites efectivos de upload y concurrencia esperada

#### 6. Los secretos de SMTP y webhooks se persisten en claro en SQLite

Estado actual:

- mitigado en esta sesion para nuevas escrituras persistidas desde el panel admin

El password SMTP y los secretos de webhook se guardan sin cifrado aplicativo.

Referencias:

- `apps/api/internal/api/handlers/smtp_settings.go`
- `apps/api/internal/repository/site_setting_repo.go`
- `apps/api/internal/repository/webhook_repo.go`

Impacto:

- medio-bajo para una app personal bien contenida
- subiria si hay copias de base de datos, backups poco controlados o varios operadores

Recomendacion:

- si quieres mantener simplicidad, moverlos a entorno y dejar en DB solo flags o configuracion no sensible
- si necesitas edicion desde UI, cifrar en reposo con clave fuera de la base

#### 7. SVG merece una politica mas explicita

Estado actual:

- mitigado en esta sesion para el pipeline de conversion SVG; el worker ya sanea el SVG antes de renderizarlo

El repo sanea HTML remoto antes de ciertas conversiones, pero los SVG se envian a motores de render externo sin una fase equivalente visible de saneado previo.

Referencias:

- `apps/api/internal/workers/document/html_sanitize.go`
- `apps/api/internal/workers/image/svg_convert.go`
- `apps/api/cmd/server/main.go`

Estado:

- hallazgo probable, no explotacion confirmada

Riesgo concreto:

- fetches externos no deseados segun comportamiento de libreria
- cargas patologicas o consumo de recursos en parseo/render

Recomendacion:

- decidir si SVG se acepta solo tras saneado
- o bien desactivar temporalmente las capacidades SVG mas expuestas hasta validarlo con tests de seguridad

### Overkill para este proyecto

No considero bloqueantes ahora mismo:

- WAF pesado
- antivirus por cada fichero
- sandbox por VM por conversion
- gestor de secretos externo complejo
- controles enterprise de IAM o microsegmentacion fina

La app ya tiene una base mejor que la media para su escala. El objetivo sensato aqui es cerrar huecos concretos, no meter complejidad teatral.

## Lo que esta bien resuelto

- deteccion real de formato, no basada solo en extension
- validacion posterior del artefacto generado
- CSP y ausencia de sinks XSS obvios en frontend
- metricas HTTP con route pattern, no con paths de alta cardinalidad
- separacion razonable entre API, orquestacion, workers y storage
- endurecimiento de contenedores por encima de lo habitual en proyectos personales

## Plan de accion proporcional

### Fase 1

- corregir el sistema de migraciones para que no pueda omitirse ninguna migration existente
- documentar el despliegue soportado con reverse proxy y TLS como via canonica
- dejar claro en ejemplos que la API no debe exponerse directamente a internet

### Fase 2

- hacer atomico el limite de jobs activos para invitados
- restringir destinos de webhooks para evitar SSRF administrativa
- añadir tests de concurrencia para rate limits y limites de guest

### Fase 3

- reducir dependencia de temporales multipart en upload
- sacar secretos de DB o cifrarlos
- revisar y endurecer la politica de SVG

## Conclusión

Con tus dos supuestos operativos, no veo una aplicacion “coladero”. Veo una base razonablemente seria para una app personal publica, con unos pocos puntos concretos que si conviene cerrar antes o poco despues de exponerla.

La prioridad real, bajo ese marco, es:

1. despliegue canonico seguro y bien documentado
2. migraciones correctas y sin deriva manual
3. limites atomicos para invitados
4. control de SSRF en webhooks
5. mejora progresiva de uploads y gestion de secretos