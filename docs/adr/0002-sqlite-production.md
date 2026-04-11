# ADR 0002: SQLite como base de datos de producción

## Estado

Aceptado

## Contexto

El sistema necesita persistir archivos, jobs, artefactos, auditoría y usuarios.
La carga esperada para V1 es baja: decenas de usuarios concurrentes, cientos de conversiones diarias.

Se evaluó si usar SQLite como base de datos principal o adoptar PostgreSQL desde el inicio.

## Decisión

Se adopta SQLite con WAL mode como base de datos en producción para V1.

Razones:

1. **Simplicidad operativa**: no requiere servicio externo, backup es copiar un archivo, deploy es un binario + disco.
2. **Suficiente para la escala esperada**: WAL mode con `_busy_timeout=5000` y single-writer soporta cientos de escrituras por minuto sin contención real.
3. **Portabilidad**: los repositorios usan `*sql.DB` con queries estándar SQL. La migración a PostgreSQL es posible sin reescribir la capa de repositorio.
4. **Reducción de infra**: un componente menos que operar, monitorear y escalar.

## Restricciones aceptadas

- Single-writer: no es posible separar API y workers en hosts diferentes si ambos escriben a la misma DB.
- Sin conexiones remotas: la DB debe vivir en el mismo filesystem que la API.
- Backups requieren `VACUUM INTO` o copiar el archivo con escrituras pausadas.
- No soporta consultas analíticas pesadas concurrentes con escrituras.

## Criterios de migración a PostgreSQL

Migrar si se cumple cualquiera de:

- Se necesitan múltiples instancias de API o workers en hosts separados.
- La contención por escritura causa timeouts visibles (>5s).
- Se requieren consultas analíticas en tiempo real sobre datos operativos.
- Se necesitan capabilities como LISTEN/NOTIFY, full-text search avanzado, o JSON indexing.

## Plan de migración

1. Los repositorios ya usan `*sql.DB` — el driver es intercambiable.
2. Las migraciones SQL usan sintaxis compatible (CREATE TABLE IF NOT EXISTS, tipos básicos).
3. Adaptar: `?` placeholders → `$1` placeholders, `INTEGER PRIMARY KEY` → `UUID`, `datetime` → `timestamptz`.
4. Añadir tabla de tracking de migraciones si no existe.

## Alternativas descartadas

### PostgreSQL desde V1

Ventajas:
- Escala horizontal desde el inicio
- Tooling maduro de backups y monitoreo

Desventajas:
- Complejidad operativa innecesaria para la escala actual
- Requiere servicio externo (managed o self-hosted)
- Overhead de configuración para desarrollo local

### SQLite sin documentar limitaciones

Descartada: ocultar restricciones conocidas es deuda técnica silenciosa.
