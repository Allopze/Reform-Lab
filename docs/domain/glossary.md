# Glosario del dominio

## Propósito

Este glosario define el vocabulario oficial del proyecto.

Todo el repositorio debe usar estos términos de forma consistente.
Evitar sinónimos innecesarios.

---

## Términos principales

### OriginalFile
Archivo original subido por el usuario.
Es inmutable y actúa como fuente de verdad del input recibido.

### DetectedFormat
Resultado de la detección real del archivo.
No depende solo de la extensión; se basa en validación técnica del contenido.

### FileMetadata
Conjunto de metadatos extraídos del archivo.
Puede incluir, según formato:

- tamaño
- número de páginas
- duración
- resolución
- encoding
- presencia de protección
- hojas
- idioma detectado
- MIME técnico

### Capability
Operación permitida para un archivo dado según su formato real, metadatos y políticas del sistema.

Ejemplos:
- convertir PDF a imágenes
- extraer texto
- comprimir imagen
- convertir DOCX a PDF

### CapabilityResolver
Componente responsable de determinar qué capacidades aplican a un archivo y por qué.

### ConversionRequest
Intención del usuario de ejecutar una operación permitida sobre un archivo.

### Job
Unidad de trabajo asíncrona que representa la ejecución de una operación concreta.

### JobStatus
Estado formal del job.
Estados sugeridos:
- `queued`
- `running`
- `succeeded`
- `failed`
- `cancelled`
- `expired`

### Artifact
Resultado persistido de una operación.
Puede ser un archivo convertido, un ZIP, una preview, un texto extraído u otro derivado válido.

### ArtifactValidation
Proceso de validar que el artefacto generado sea utilizable y cumpla requisitos mínimos.

### AuditEvent
Registro estructurado de una acción relevante del sistema.
Debe permitir trazabilidad operativa y de seguridad.

### RetentionPolicy
Reglas que definen cuánto tiempo se conserva cada tipo de objeto:

- original
- temporal
- artefacto
- preview
- logs asociados

### Ingestion
Fase del sistema que recibe, valida, detecta y clasifica el archivo.

### Worker
Proceso o servicio que ejecuta una operación concreta fuera del request path principal.

### ConversionEngine
Herramienta o adaptador técnico que realiza una transformación.
No debe confundirse con una capacidad de producto.

### CapabilityCatalog
Fuente de verdad del conjunto de capacidades soportadas por el sistema.

---

## Reglas de lenguaje

### Regla 1
Usar `DetectedFormat` para el formato real del archivo, no “extension”, “kind” o nombres ambiguos.

### Regla 2
Usar `Capability` para una posibilidad funcional del producto.
No usar “action”, “option” o “feature” cuando se refiere específicamente a una operación soportada sobre un archivo.

### Regla 3
Usar `ConversionEngine` o `WorkerAdapter` para la herramienta técnica subyacente.
No mezclarlo con el lenguaje de producto.

### Regla 4
Usar `Artifact` para outputs persistidos.
No usar indiscriminadamente “result”, porque puede referirse a estados, errores o respuestas API.

### Regla 5
Usar `JobStatus` con estados cerrados y documentados.
No inventar estados locales sin definición global.

---

## Antónimos o confusiones a evitar

- extensión de archivo != formato detectado
- motor técnico != capacidad de producto
- job != solicitud del usuario
- artefacto != archivo original
- validación de archivo != validación de salida
