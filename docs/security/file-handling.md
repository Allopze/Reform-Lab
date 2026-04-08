# Seguridad y manejo de archivos

## Propósito

El manejo de archivos es parte central del producto y una superficie crítica de riesgo.

Este documento define reglas mínimas para:

- validar archivos
- detectar formatos
- limitar daños potenciales
- aislar ejecución
- reducir exposición de datos
- mantener trazabilidad

---

## Principios

1. Nunca confiar en la extensión enviada por el usuario.
2. Validar tipo real del archivo.
3. Aplicar allowlist de formatos soportados.
4. Limitar tamaño, tiempo, memoria y complejidad.
5. Mantener aislamiento razonable de motores de conversión.
6. Tratar originales, temporales y artefactos como clases distintas.
7. Evitar exposición innecesaria de nombres y paths internos.
8. No registrar contenido sensible en logs.

---

## Flujo seguro de ingestión

1. recibir subida
2. asignar identificador interno
3. renombrar internamente
4. validar tamaño y límites iniciales
5. detectar tipo real
6. extraer metadatos seguros
7. rechazar si no cumple política
8. persistir en storage controlado
9. registrar evento de auditoría

---

## Validaciones mínimas de entrada

### Archivo
- tamaño máximo permitido
- allowlist por formato o familia
- detección de tipo real
- consistencia básica entre extensión y formato cuando aplique
- rechazo de archivos vacíos si no son válidos para el caso
- límites por número de páginas, frames, hojas o duración cuando aplique

### Nombre
- no confiar en el nombre original para rutas internas
- sanitizarlo para uso de display si se conserva
- no usarlo como clave principal

### Contenido
- no ejecutar nada proveniente del archivo
- no descomprimir o parsear de forma ilimitada
- tratar archivos compuestos con controles adicionales

---

## Almacenamiento

### Originales
- inmutables
- acceso controlado
- fuera de rutas públicas directas

### Temporales
- TTL obligatorio
- nombre interno
- limpieza automática
- scope mínimo por ejecución

### Artefactos
- trazables al job y al original
- con política de retención definida
- con validación de salida antes de exponerse

---

## Aislamiento de workers

Todo worker o motor debería ejecutarse con:

- mínimos privilegios
- timeout explícito
- límites de CPU
- límites de memoria
- límites de disco
- filesystem acotado
- acceso de red restringido salvo justificación
- limpieza posterior a la ejecución

No correr motores peligrosos con permisos amplios por comodidad.

---

## Manejo de errores

Separar al menos entre:

- archivo no soportado
- archivo inválido o corrupto
- límite excedido
- archivo protegido o cifrado no soportado
- fallo transitorio del motor
- salida inválida
- error interno de infraestructura

Los usuarios no deben ver detalles internos sensibles.
Los sistemas internos sí deben poder trazar la causa.

---

## Logs y auditoría

Registrar:

- recepción del archivo
- detección de formato
- capacidad solicitada
- creación del job
- inicio y fin de ejecución
- validación de salida
- errores clasificados
- expiración o borrado según política

No registrar:

- contenido completo del archivo
- secretos
- rutas internas sensibles
- datos personales innecesarios

---

## Controles complementarios sugeridos

Según el riesgo y el tipo de despliegue, considerar:

- escaneo de malware
- sandbox reforzado
- rate limiting por usuario
- cuotas por tamaño o volumen
- firma de URLs temporales
- cifrado en reposo
- borrado diferido y verificable

---

## Reglas para agentes IA

Ningún agente debe:

- relajar validaciones para “hacer pasar” tests
- quitar timeouts porque una conversión tarda mucho
- ampliar permisos sin documentarlo
- exponer nombres internos o paths del sistema
- introducir logs con contenido sensible
