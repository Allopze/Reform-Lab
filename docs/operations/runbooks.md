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

## Postmortem mínimo recomendado

- resumen del incidente
- impacto
- línea temporal
- causa raíz
- mitigación inmediata
- acciones preventivas
- owner y fecha objetivo
