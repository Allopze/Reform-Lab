# CONTRIBUTING.md

## Objetivo

Este proyecto prioriza cambios pequeños, trazables y fáciles de revisar.

Quien contribuya al repositorio, sea humano o agente IA, debe mantener:

- claridad arquitectónica
- seguridad en el manejo de archivos
- estabilidad de contratos
- bajo acoplamiento
- buena cobertura de pruebas

## Principios de contribución

1. El cambio mínimo suficiente.
2. Una responsabilidad por PR siempre que sea posible.
3. Refactor y cambio funcional deben separarse o declararse explícitamente.
4. La seguridad no se negocia por velocidad.
5. Todo cambio que altere comportamiento debe venir con tests.
6. Toda decisión estructural relevante debe documentarse en un ADR.

## Antes de comenzar

Leer:

- `README.md`
- `AGENTS.md`
- `docs/architecture/system-overview.md`
- `docs/domain/glossary.md`

Si el cambio afecta seguridad:
- `docs/security/file-handling.md`

Si el cambio afecta testing:
- `docs/testing/test-strategy.md`

## Tipos de cambios aceptables

- corrección de bugs
- adición de tests
- mejoras de observabilidad
- mejoras de validación
- adición de nuevas capacidades de conversión
- documentación
- refactors acotados con beneficio real

## Tipos de cambios que requieren más cuidado

- cambios en contratos API
- cambios de estados de jobs
- cambios en storage o políticas de retención
- incorporación de nuevos motores de conversión
- cambios de arquitectura
- nuevos formatos soportados
- cambios en permisos, sandbox o seguridad

## Estructura recomendada de PR

### Título
Formato recomendado:

- `fix: ...`
- `feat: ...`
- `refactor: ...`
- `docs: ...`
- `test: ...`
- `chore: ...`

### Descripción
Toda PR debería responder:

- qué cambia
- por qué cambia
- qué capa toca
- qué riesgos introduce
- qué tests se añadieron o actualizaron
- si hay cambios de contrato o migración

## Criterios mínimos antes de merge

- el cambio respeta la arquitectura
- no hay lógica duplicada innecesaria
- los tests relevantes pasan
- la seguridad no se degrada
- la documentación está actualizada si aplica
- los nombres y estados siguen siendo coherentes

## Cuándo crear un ADR

Crear un ADR si la contribución introduce o modifica algo estructural como:

- estrategia de colas
- estructura del dominio
- catálogo de capacidades
- sistema de storage
- sandbox de workers
- contratos públicos relevantes
- multi-tenant
- observabilidad estándar

## Estilo de código

Preferir:

- funciones cortas
- nombres explícitos
- estados cerrados y bien definidos
- validaciones cerca del borde
- errores clasificables
- módulos con una responsabilidad clara

Evitar:

- helpers genéricos ambiguos
- lógica de negocio en UI
- lógica de negocio en workers que debería vivir en dominio
- refactors masivos innecesarios
- constantes mágicas repetidas
- estados implícitos derivados de `null`

## Cambios en nuevas conversiones

Toda nueva capacidad de conversión debería incluir:

- definición de la capacidad
- validaciones y límites
- motor o adaptador responsable
- tests del caso feliz
- tests de error o límites
- documentación si cambia experiencia del usuario

## Regla operativa

Si el cambio no puede explicarse en pocas frases claras, probablemente es demasiado grande para una sola contribución.
