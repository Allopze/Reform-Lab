# Documentación base del proyecto

Este paquete contiene un set inicial de documentos para desarrollar un **servicio web de conversión de archivos inteligente** con ayuda de agentes de IA.

## Objetivo del producto

El sistema permite que un usuario suba un archivo y vea únicamente las opciones de procesamiento o conversión que sean coherentes con:

- el tipo real del archivo
- sus metadatos
- las capacidades soportadas por el sistema
- las políticas de seguridad y límites operativos

## Objetivo de este pack

Este pack existe para que humanos y agentes de IA trabajen sobre el mismo proyecto con un marco común de:

- arquitectura
- dominio
- seguridad
- testing
- operación
- evolución del repositorio

## Orden recomendado de lectura

1. `AGENTS.md`
2. `docs/architecture/system-overview.md`
3. `docs/domain/glossary.md`
4. `docs/domain/capabilities-catalog.md`
5. `docs/security/file-handling.md`
6. `docs/testing/test-strategy.md`
7. `CONTRIBUTING.md`

## Archivos incluidos

### Raíz
- `README.md`
- `AGENTS.md`
- `CONTRIBUTING.md`

### GitHub / agentes
- `.github/copilot-instructions.md`
- `.github/instructions/backend.instructions.md`
- `.github/instructions/frontend.instructions.md`
- `.github/instructions/workers.instructions.md`

### Arquitectura
- `docs/architecture/system-overview.md`
- `docs/architecture/repo-map.md`

### Dominio
- `docs/domain/glossary.md`
- `docs/domain/capabilities-catalog.md`

### Seguridad
- `docs/security/file-handling.md`

### Testing
- `docs/testing/test-strategy.md`

### Operación
- `docs/operations/runbooks.md`

### Producto
- `docs/product/non-goals.md`

### Decisiones
- `docs/adr/0001-foundation.md`

## Cómo usar este pack

1. Copiar los archivos al repositorio.
2. Ajustar nombres de módulos y carpetas a la estructura real del proyecto.
3. Reemplazar ejemplos y placeholders por decisiones concretas.
4. Crear ADRs nuevos cada vez que se tome una decisión estructural importante.
5. Mantener `AGENTS.md` y la documentación de arquitectura sincronizados.

## Nota importante

El contenido está escrito para ser:

- legible por humanos
- interpretable por agentes IA
- suficientemente específico para orientar cambios
- suficientemente genérico para no forzar un stack prematuro

No reemplaza documentación técnica concreta del proyecto. La prepara.
