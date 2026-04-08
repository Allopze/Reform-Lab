# Instrucciones globales para agentes y asistentes en este repositorio

## Contexto del producto

Este repositorio implementa un servicio web de conversión de archivos inteligente.
El usuario sube un archivo y el sistema ofrece únicamente acciones coherentes con el formato real detectado y sus políticas.

## Reglas globales

- no confiar en extensiones de archivo como fuente final de verdad
- no duplicar lógica de capacidades entre frontend, backend y workers
- no mezclar seguridad, almacenamiento y reglas de producto
- no hacer procesamiento pesado en request/response si debe vivir en jobs
- no debilitar límites, validaciones o aislamiento por conveniencia

## Orden de lectura recomendado

1. `AGENTS.md`
2. `docs/architecture/system-overview.md`
3. `docs/domain/glossary.md`
4. `docs/domain/capabilities-catalog.md`

## Cambios preferidos

- pequeños y localizados
- con tests
- con nombres explícitos
- con errores claros
- con respeto por las fronteras entre capas

## Cambios a evitar

- refactors masivos no pedidos
- nuevas dependencias sin justificación
- lógica de negocio en componentes UI
- cambios silenciosos de contrato
- helpers genéricos sin ownership

## Recordatorio operativo

Este proyecto prefiere decisiones simples, seguras y mantenibles sobre soluciones ingeniosas.
