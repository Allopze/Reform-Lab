---
applyTo: "services/workers/**"
---

# Instrucciones para workers

## Enfoque

Los workers ejecutan conversiones.
No definen política de producto.

## Reglas

- recibir instrucciones claras y autocontenidas
- aplicar límites de tiempo, CPU, memoria y disco
- validar salida antes de marcar éxito
- reportar estados con precisión
- generar logs estructurados y métricas
- limpiar temporales

## No hacer

- decidir qué capacidades existen
- inferir permisos de negocio
- leer configuración dispersa desde múltiples lugares
- ejecutar herramientas con privilegios amplios por comodidad
- dejar archivos temporales huérfanos

## Resultado esperado de un worker

Cada ejecución debería producir:

- estado final claro
- artefacto válido o error clasificado
- métricas básicas
- trazabilidad al job y al archivo original

## Testing esperado

- pruebas del caso feliz
- pruebas de archivos corruptos o no soportados
- pruebas de timeout o límites
- validación de salida
