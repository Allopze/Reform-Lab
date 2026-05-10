# Runbooks operativos

## Propósito

Este documento ofrece procedimientos base para incidentes comunes del sistema.

Debe evolucionar con el producto real.
No es un sustituto de observabilidad.
Es una guía para actuar sin improvisar.

---

## Señales básicas a observar siempre

- tasa de error por capacidad
- latencia de jobs
- tamaño de cola
- workers activos
- tasa de retry
- crecimiento de temporales
- fallos de validación de salida
- almacenamiento disponible

---

## Smoke de runtime

- existe un smoke reproducible en `apps/api/scripts/docker-e2e-smoke.sh`
- existe un smoke local contra una API ya levantada en `apps/api/scripts/conversion-smoke.sh`
- el smoke Docker valida familias críticas contra la stack real de Docker Compose:
  - `PDF -> TXT`
  - `PNG -> WebP`
  - `WAV -> MP3`
  - `MP4 -> GIF`
  - `DOCX -> PDF`
  - `HEIF -> PNG`
  - `SVG -> PDF`
  - `PPTX -> JPG ZIP`
  - `XLSX -> CSV`
- el script espera `/api/health`, crea usuarios aislados por escenario y evita confundir cuotas anti-abuso con fallos funcionales
- los fixtures simples de PDF, PNG, WAV, MP4 y DOCX se generan en runtime para no guardar binarios pesados o frágiles en el repo
- usar un `JWT_SECRET` válido y suficientemente largo al ejecutarlo fuera de un entorno ya configurado

Uso local contra una API ya disponible:

```bash
BASE_URL=http://127.0.0.1:4040 apps/api/scripts/conversion-smoke.sh
```

El smoke local:

- no levanta ni destruye Docker
- genera fixtures pequeños en un directorio temporal
- usa fixtures versionados para HEIF, PPTX y XLSX cuando existen
- marca `SKIP` si el runtime no acepta el upload o no ofrece una capability
- falla si una conversion ofrecida termina en estado terminal distinto de `succeeded`

## Despliegue: Docker Compose en servidor

### Stack esperado

- `docker-compose.yml` en la raíz del repo es la referencia de despliegue full stack
- expone `web` en `5050` y `api` en `8080` por defecto
- usa Redis y worker standalone en modo `production`

### Persistencia en host

- SQLite, originales, temporales y artefactos viven en `./runtime/data` relativo al directorio donde se ejecuta `docker compose`
- Redis persiste en `./runtime/redis` y debe fallar con presión de memoria antes de expulsar claves de cola
- `data-permissions` corrige la ownership de `./runtime/data` antes de que arranquen `api` y `worker`; después, los procesos runtime escriben como usuario no-root sin capacidades añadidas

### Procedimiento base

1. copiar `.env.production.example` a `.env`
2. ajustar URLs públicas, `JWT_SECRET` y cualquier secreto adicional
3. levantar con:
   ```bash
   docker compose up -d --build
   ```
4. verificar salud:
   ```bash
   curl http://127.0.0.1:8080/api/health
   ```

### Notas

- cambios en `NEXT_PUBLIC_API_URL` requieren rebuild del servicio `web`
- no borrar manualmente `./runtime/data` sin entender el impacto en SQLite y artefactos
- si el servidor queda detrás de proxy, mantener `TRUST_PROXY_HEADERS=true` solo si ese proxy sanea `X-Forwarded-*`
- si activas `EXPOSE_METRICS=true` en producción, configura `METRICS_TOKEN`; el backend rechaza esa combinación insegura
- `REQUIRE_VERIFIED_EMAIL_FOR_SENSITIVE_ACTIONS=true` exige email verificado para mutaciones sensibles; actívalo solo después de comprobar que SMTP y el flujo de verificación funcionan para los administradores

### Contrato de engines API/worker

- La API resuelve `Capability` consultando la disponibilidad local de engines en su propio proceso.
- En modo in-process, la API reporta engines probados porque el worker vive en el mismo proceso.
- En despliegues con worker standalone y Redis, la API puede usar engines declarados para no depender de binarios pesados en el contenedor API; el worker reporta su disponibilidad efectiva en `/api/admin/health`.
- Revisar `runtime.workers.apiEngineMode`: `probed` significa que la API verifico binarios locales; `declared` significa que la API esta declarando capacidades y la ejecucion real depende del worker.
- Revisar `runtime.workers.workers[].engines` para confirmar el set efectivo de engines visto por cada worker.
- Si falta un engine en la API, la UI puede ocultar una capacidad aunque el worker pudiera ejecutarla.
- Si falta un engine en el worker, la API puede mostrar una capacidad que luego fallará en ejecución.
- Tras cada despliegue, revisar `/api/admin/engines` y al menos un smoke de conversión real para las familias críticas del entorno.
- Si se decide separar runtimes livianos/pesados, cambiar antes el modelo de disponibilidad para que provenga de configuración explícita o health reportado por workers.

### Binarios de ingestion

La API no solo necesita binarios para conversiones. Algunas familias dependen de herramientas de inspeccion durante la subida:

- PDF: `pdfinfo` para paginas y proteccion.
- Audio/video: `ffprobe` para duracion.
- Imagenes no cubiertas por decoders Go, como HEIC/HEIF: `ffprobe` como fallback de dimensiones.
- SVG: lectura segura del root `<svg>` sin ejecutar el archivo; no requiere binario para metadata basica.
- OOXML/ODF: inspeccion ZIP/XML para detectar proteccion; no requiere LibreOffice en ingestion.

Si el contenedor API se deja "liviano", validar que esos binarios de ingestion sigan presentes para las familias que se quieran aceptar.

### Advertencia conocida: Sentry/OpenTelemetry en build web

`npm run build` puede emitir una advertencia no bloqueante de Sentry/OpenTelemetry por dependencia dinámica en instrumentación Prisma.
La política operativa actual es aceptarla si el build termina OK y siguen pasando tests, `npm audit --omit=dev` y `govulncheck`.
Revisar esta nota cuando se actualicen Sentry, OpenTelemetry o la instrumentación de Next.js.

---

## Incidente: la cola crece sin bajar

### Síntomas
- muchos jobs en `queued`
- poca o nula ejecución real
- latencia total creciente

### Verificaciones
1. revisar salud del sistema de colas
2. revisar disponibilidad de workers
3. revisar consumo de CPU y memoria
4. revisar si hay un tipo de job bloqueando el throughput
5. revisar despliegues recientes

### Posibles mitigaciones
- escalar workers
- pausar una capacidad problemática
- reencolar solo jobs seguros
- desactivar temporalmente una feature flag
- aplicar rate limiting adicional

---

## Incidente: muchos jobs fallan para un mismo formato

### Síntomas
- aumento de `failed`
- errores similares por una capacidad concreta

### Verificaciones
1. revisar último cambio en motor o adaptador
2. revisar muestras reales fallidas
3. revisar límites de tamaño y timeout
4. revisar validación de salida
5. revisar si el fallo es de input, motor o infraestructura

### Posibles mitigaciones
- desactivar la capacidad afectada
- revertir despliegue
- aumentar límites temporalmente si está justificado y es seguro
- aislar jobs problemáticos

---

## Incidente: temporales creciendo sin control

### Síntomas
- uso de disco en aumento
- cleanup incompleto
- errores por falta de espacio

### Verificaciones
1. revisar jobs que no limpian al finalizar
2. revisar procesos abortados
3. revisar TTL configurado
4. revisar fallos del proceso de limpieza

### Posibles mitigaciones
- ejecutar cleanup controlado
- reforzar cleanup on-failure
- bajar TTL si el producto lo permite
- aislar workers que dejan residuos

---

## Incidente: artefactos corruptos o inválidos

### Síntomas
- usuarios descargan resultados inútiles
- validación de salida insuficiente
- éxito falso del worker

### Verificaciones
1. revisar validador de salida
2. revisar muestras problemáticas
3. revisar cambios en motor o librería
4. revisar tamaño anormal de outputs
5. revisar logs de warnings omitidos

### Posibles mitigaciones
- endurecer validación de salida
- marcar como fallo donde antes se marcaba éxito
- retirar temporalmente capacidad
- reintentar solo si el error es transitorio

---

## Incidente: subida de archivos falla masivamente

### Verificaciones
1. revisar storage de entrada
2. revisar límites configurados
3. revisar autenticación y sesiones
4. revisar errores de red o balanceador
5. revisar cambios recientes en validación

### Posibles mitigaciones
- rollback del cambio reciente
- degradación controlada del flujo
- mensajes claros al usuario
- restricción temporal de tamaños máximos si el problema es de presión

---

## Reglas operativas

- no reintentar indefinidamente errores permanentes
- no marcar éxito sin validación de salida
- no borrar evidencia útil antes de clasificar el incidente
- no exponer detalles internos al usuario final
- documentar postmortem después de incidentes relevantes

---

## Procedimiento: backup de SQLite

### Cuándo
- antes de cualquier despliegue que incluya migraciones
- como parte de backup periódico (cron recomendado)

### Pasos
1. usar la API de backup online de SQLite (`.backup` o `VACUUM INTO`):
   ```bash
   sqlite3 /data/reform.db "VACUUM INTO '/backups/reform-$(date +%Y%m%d-%H%M%S).db';"
   ```
2. verificar integridad del backup:
   ```bash
   sqlite3 /backups/reform-*.db "PRAGMA integrity_check;"
   ```
3. copiar el backup a almacenamiento externo (S3, volumen remoto, etc.)
4. mantener al menos 7 días de backups; rotar los más antiguos

### Notas
- no usar `cp` directamente sobre la base de datos en modo WAL; puede generar copias inconsistentes
- no interrumpir el servidor durante `VACUUM INTO`; es seguro ejecutar en caliente

---

## Procedimiento: rotación de JWT secret

### Cuándo
- compromiso sospechado de credenciales
- rotación periódica planificada

### Pasos
1. generar un nuevo secret (mínimo 32 bytes):
   ```bash
   openssl rand -base64 48
   ```
2. configurar el nuevo valor en la variable de entorno `JWT_SECRET`
3. reiniciar el servidor de API
4. todas las sesiones existentes se invalidarán; los usuarios deberán autenticarse de nuevo
5. verificar que el endpoint `/api/health` responde y que login funciona

### Notas
- no soporta doble-secret (old + new) simultáneo; el corte es instantáneo
- coordinar con ventana de mantenimiento si el impacto de sesión es relevante

---

## Procedimiento: deshabilitar una capacidad

### Cuándo
- una capacidad produce artefactos corruptos
- un motor tiene vulnerabilidad conocida
- se requiere mantenimiento del motor subyacente

### Pasos
1. identificar el `capabilityId` afectado en `capabilities/catalog.go`
2. añadir flag de desactivación en `capabilities/flags.go` o desactivar vía feature flag
3. verificar que el resolver ya no la devuelve para archivos nuevos:
   ```bash
   curl -s http://localhost:8080/api/files/<file-id>/capabilities | jq
   ```
4. los jobs ya en cola continuarán; monitorear hasta que se completen o fallen
5. comunicar al usuario que la capacidad no está disponible temporalmente

---

## Procedimiento: limpieza de artefactos expirados

### Cuándo
- disco por encima del 80% de uso
- ejecución manual del proceso de retención

### Pasos
1. verificar la configuración actual de TTL:
   - `ARTIFACT_TTL_HOURS` (por defecto)
   - `ARTIFACT_TTL_BY_FAMILY` (override por familia)
2. el servicio de retención (`orchestrator/retention.go`) limpia automáticamente en cada ciclo
3. para forzar manualmente:
   ```bash
   # Listar artefactos expirados antes de borrar
   sqlite3 /data/reform.db "SELECT id, file_name, created_at FROM artifacts WHERE expires_at < datetime('now');"
   ```
4. ejecutar el proceso de retención manualmente si el ciclo automático no corre
5. verificar que el espacio se recuperó:
   ```bash
   du -sh /data/artifacts/
   ```

### Notas
- no borrar archivos del filesystem sin actualizar la base de datos
- los originales tienen su propia política de retención

---

## Incidente: Redis no disponible

### Síntomas
- jobs no se encolan
- API responde pero conversiones no inician
- errores de conexión en logs del servidor y workers

### Verificaciones
1. revisar estado de Redis:
   ```bash
   redis-cli ping
   ```
2. revisar conectividad de red entre API y Redis
3. revisar memoria y uso de disco de Redis
4. revisar si el problema es transitorio (reinicio) o persistente

### Posibles mitigaciones
- reiniciar Redis si es un problema de estado
- verificar y corregir la dirección de Redis en configuración (`REDIS_ADDR`)
- si Redis es irrecuperable a corto plazo, activar el queue in-process como degradación temporal (ver `queue/inprocess.go`)
- los jobs que estaban en vuelo podrían quedar en estado "running" sin avanzar; marcarlos como failed manualmente si no se recuperan tras restaurar Redis

---

## Postmortem mínimo recomendado

- resumen del incidente
- impacto
- línea temporal
- causa raíz
- mitigación inmediata
- acciones preventivas
- owner y fecha objetivo
