# AGENTS.md

## Propósito

Este repositorio contiene un sistema de **conversión de archivos inteligente**.

La función principal del sistema es:

1. recibir un archivo subido por un usuario
2. detectar su tipo real
3. extraer metadatos relevantes
4. resolver qué capacidades son compatibles
5. ejecutar una o más operaciones permitidas mediante jobs
6. entregar artefactos derivados
7. registrar el proceso de forma observable y segura

Este archivo define cómo debe trabajar cualquier agente de IA dentro del repositorio.

---

## Reglas generales para cualquier agente

### 1. Cambiar lo mínimo necesario

Toda modificación debe ser:

- pequeña
- local
- reversible
- explicable
- consistente con el diseño existente

No reestructurar el repositorio completo para resolver una tarea pequeña.

### 2. No inventar arquitectura

Antes de crear nuevos patrones, revisar primero:

- `docs/architecture/system-overview.md`
- `docs/architecture/repo-map.md`
- `docs/adr/`

Si una decisión estructural no está clara, favorecer la solución más conservadora y explícita.

### 3. Respetar fronteras del sistema

No mezclar responsabilidades entre:

- frontend
- API
- dominio
- orquestación
- workers
- storage
- seguridad
- observabilidad

### 4. No confiar en extensiones de archivo

Nunca asumir el tipo real del archivo por nombre o extensión.
Toda lógica funcional importante debe partir de detección real del archivo.

### 5. No debilitar seguridad por conveniencia

No introducir atajos como:

- desactivar validaciones
- ampliar permisos de workers sin justificación
- exponer rutas directas a archivos internos
- omitir límites de memoria, tiempo o tamaño
- procesar archivos sin aislamiento razonable

### 6. No duplicar lógica de capacidades

La lógica que decide qué opciones están disponibles para un archivo debe vivir en una fuente de verdad clara.
No duplicarla entre frontend, backend y workers.

---

## Modelo mental obligatorio del sistema

Todo cambio debe respetar este flujo conceptual:

1. subida segura del archivo
2. validación inicial
3. detección de tipo real
4. extracción de metadatos
5. resolución de capacidades
6. creación de solicitud de conversión
7. ejecución asíncrona del job
8. validación de salida
9. persistencia del artefacto
10. auditoría y observabilidad

La detección no es conversión.
La conversión no es almacenamiento.
El almacenamiento no es política de producto.

---

## Modelo de jobs y estados

Los jobs tienen un ciclo de vida con estados cerrados:

```
queued → running → succeeded
                 → failed → (retry) → queued
                 → cancelled
                 → expired
```

- **queued**: encolado, esperando ejecución
- **running**: siendo procesado por un worker
- **succeeded**: completado exitosamente, artifact disponible
- **failed**: error durante la ejecución, con mensaje de error clasificado
- **cancelled**: cancelado por el usuario o admin
- **expired**: el artifact fue eliminado por política de retención

Reglas importantes:
- Las transiciones inválidas son rechazadas (`ErrInvalidTransition`)
- Solo jobs en estado `failed` pueden ser reintentados
- Los jobs tienen un límite de reintentos configurado por capacidad
- El polling del frontend debe manejar todos los estados terminales

---

## Instrucciones antes de tocar código

Antes de implementar, el agente debe revisar, en este orden:

1. `README.md`
2. `AGENTS.md`
3. `docs/architecture/system-overview.md`
4. `docs/domain/glossary.md`
5. documentación del módulo afectado
6. tests existentes del área
7. ADRs relevantes

Si la tarea afecta seguridad, también leer:
- `docs/security/file-handling.md`

Si la tarea afecta testing o CI:
- `docs/testing/test-strategy.md`

---

## Qué sí puede hacer un agente

- corregir errores localizados
- añadir tests
- mejorar nombres cuando el cambio ya toca ese código
- completar validaciones faltantes
- extender capacidades siguiendo el catálogo del dominio
- documentar decisiones
- añadir métricas o logs estructurados cuando falten

## Qué no debe hacer un agente

- crear utilidades genéricas sin necesidad real
- mezclar refactor y cambio funcional sin explicarlo
- mover carpetas o módulos de forma masiva
- cambiar contratos públicos sin revisar compatibilidad
- introducir nuevas dependencias grandes por comodidad
- reescribir componentes completos si un cambio puntual basta
- propagar estados ambiguos
- esconder reglas de negocio en helpers o componentes UI

---

## Estructura lógica del repositorio

La estructura exacta puede variar, pero el agente debe respetar estas fronteras:

- `apps/web`: interfaz del usuario (Next.js 15, React 19, Tailwind CSS 4, Vitest)
- `apps/api`: API pública, auth, repositorios, orquestación, workers embebidos (Go 1.25, Chi, SQLite, Asynq)
- `apps/api/internal/`: todos los módulos internos del backend:
  - `api/` — handlers HTTP y middleware
  - `auth/` — autenticación JWT
  - `capabilities/` — catálogo y resolución de capacidades
  - `database/` — conexión y migraciones SQLite
  - `domain/` — entidades, value objects, estados, reglas de negocio
  - `email/` — envío de emails
  - `ingestion/` — validación, detección de formato, metadatos
  - `observability/` — logging (zerolog), métricas (Prometheus), tracing (OTel)
  - `orchestrator/` — gestión de jobs, cola, transiciones de estado
  - `queue/` — interfaz de cola (Asynq/Redis o in-process)
  - `repository/` — acceso a datos (SQLite)
  - `security/` — políticas de archivo, secret keeper
  - `storage/` — filesystem para originales, temporales y artefactos
  - `webhook/` — notificaciones webhook
  - `workers/` — ejecución de conversiones (document, image, video, audio, pdf, ocr)
- `docs/`: documentación viva del sistema (ADR, arquitectura, dominio, seguridad, testing, operación, producto)
- `.github/instructions/`: reglas específicas por capa para agentes

Si el repo real usa otra estructura, el agente debe apoyarse en `docs/architecture/repo-map.md`.

---

## Comandos operativos

### Desarrollo

```bash
# Levantar todo el stack (web + api)
npm run dev

# Solo frontend
npm run dev:web

# Solo backend
npm run dev:api
```

### Backend (Go)

```bash
cd apps/api

# Build
go build ./...

# Tests unitarios
go test ./internal/... -count=1 -timeout=180s

# Tests de un paquete específico
go test ./internal/capabilities/... -v -count=1

# Tests E2E
go test ./internal/api/... -run TestE2E -v -count=1 -timeout=300s

# Lint
go vet ./...
```

### Frontend (Next.js)

```bash
cd apps/web

# Tests unitarios
npm run test

# Tests E2E (Playwright)
npm run test:e2e

# Lint
npm run lint
```

---

## Cómo agregar una nueva capacidad de conversión

1. **Definir la capacidad** en `apps/api/internal/capabilities/catalog.go`:
   - ID único
   - SourceFormats (MIME types de entrada)
   - TargetFormat (formato de salida)
   - Engine (nombre del motor)
   - ExecutionLimits (timeout, max retries)
   - KnownLimitations (documentar restricciones)

2. **Registrar el engine** en `apps/api/internal/workers/registry.go`:
   - Mapear capability ID → engine implementation
   - El engine debe implementar la interfaz `Engine`

3. **Implementar el engine** en el subdirectorio correspondiente de `workers/`:
   - `document/`, `image/`, `video/`, `audio/`, `pdf/`, `ocr/`
   - Usar `exec.CommandContext` con timeout
   - Validar output con `validateOutputArtifact`

4. **Agregar tests**:
   - Unit test del engine con archivo de prueba
   - Test de resolución de capacidades
   - Test E2E si el flujo cambia

5. **Verificar**:
   - `go test ./internal/... -count=1`
   - El engine binario está disponible en el entorno (ver `engines.go`)

---

## Reglas de diseño del dominio

### Entidades base

Usar vocabulario consistente con estas entidades:

- `OriginalFile`
- `DetectedFormat`
- `Capability`
- `ConversionRequest`
- `Job`
- `Artifact`
- `AuditEvent`
- `RetentionPolicy`

No introducir sinónimos innecesarios.

### Registro de capacidades

Toda nueva conversión debe modelarse primero como capacidad.
Una capacidad debería poder responder al menos:

- formato origen
- operación o formato destino
- condiciones de disponibilidad
- límites
- motor responsable
- calidad esperada
- restricciones conocidas

---

## Reglas de cambios por capa

### Frontend
- no decidir reglas finales de negocio
- no inventar capacidades
- usar contratos del backend
- mostrar estados y límites con claridad

### API
- validar inputs
- respetar contratos
- modelar errores de forma consistente
- no hacer procesamiento pesado síncrono si debe ir a jobs

### Dominio
- mantener reglas puras y explícitas
- evitar acoplarlo a infraestructura

### Workers
- ejecutar
- validar salida
- reportar estado
- no decidir políticas de producto

### Storage
- originales inmutables
- artefactos trazables
- temporales con expiración

---

## Testing obligatorio

Cuando cambie comportamiento, añadir o actualizar tests.

Capas mínimas:
- unit tests para reglas de dominio
- integration tests para infraestructura relevante
- contract tests para API
- pruebas end-to-end de flujos críticos
- corpus de archivos reales y corruptos para formatos soportados

No agregar soporte a un formato sin ampliar su cobertura de pruebas.

---

## Observabilidad mínima

Todo componente crítico debe emitir:

- logs estructurados
- métricas clave
- trazas si el stack lo soporta

No loggear contenido sensible del archivo.

---

## Compatibilidad y migraciones

Antes de cambiar un contrato, una migración o un estado público:

1. revisar impacto
2. mantener compatibilidad hacia atrás cuando sea razonable
3. documentar la transición
4. usar feature flags o rollout gradual si el cambio es riesgoso

---

## Checklist antes de cerrar una tarea

El agente debe verificar:

- el cambio vive en la capa correcta
- no se duplicó lógica de capacidades
- no se debilitó seguridad
- no se rompieron contratos sin declararlo
- hay tests suficientes
- los errores son claros
- la observabilidad sigue intacta o mejora
- la documentación quedó actualizada si hacía falta

---

## Regla final

Priorizar siempre soluciones:

- aburridas
- claras
- mantenibles
- seguras
- fáciles de razonar

Si una solución parece ingeniosa pero vuelve más difícil entender el sistema, no pertenece aquí.
