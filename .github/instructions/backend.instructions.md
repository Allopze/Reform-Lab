---
applyTo: "apps/api/**,services/domain/**,services/capabilities/**,services/ingestion/**,services/orchestrator/**,services/storage/**,services/security/**,services/observability/**"
---

# Instrucciones para backend

## Enfoque

El backend debe actuar como capa de coordinación, validación y exposición de contratos.
No debe mezclar dominio con detalles arbitrarios de infraestructura.

## Reglas

- validar inputs en el borde
- mantener contratos de request y response explícitos
- separar errores de validación, dominio, infraestructura y seguridad
- mover trabajo pesado a jobs cuando corresponda
- mantener el dominio razonablemente puro
- modelar estados de job con tipos cerrados
- mantener idempotencia donde aplique

## No hacer

- lógica de negocio en controladores
- acceso directo desordenado a storage desde múltiples capas
- decisiones de capacidades dispersas
- estados ambiguos
- side effects ocultos

## Al tocar capacidades

Toda nueva capacidad debe poder responder:

- origen
- destino u operación
- condiciones
- límites
- motor responsable
- calidad esperada
- restricciones conocidas

## Testing esperado

- unit tests para reglas de dominio
- integration tests para persistencia, cola o storage
- contract tests para endpoints afectados
