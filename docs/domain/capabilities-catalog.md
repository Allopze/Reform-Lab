# Catálogo de capacidades

## Propósito

Este documento define cómo debe modelarse el catálogo de capacidades del sistema.

La lógica de capacidades es crítica:
es lo que permite al producto responder de forma coherente a la pregunta:

**"¿Qué se puede hacer con este archivo?"**

Este catálogo debe ser una fuente de verdad centralizada.
No debe duplicarse entre frontend, API y workers.

---

## Qué es una capacidad

Una capacidad es una operación que el sistema puede ofrecer sobre un archivo dado en función de:

- formato detectado
- metadatos relevantes
- políticas del producto
- límites operativos
- disponibilidad del motor responsable

No toda capacidad es una conversión estricta.
Puede ser también:

- extracción
- optimización
- compresión
- rasterización
- validación
- preview
- reempaquetado

---

## Campos mínimos de una capacidad

Toda capacidad debería definir como mínimo:

- `id`
- `displayName`
- `presentationOrder`
- `sourceFormat`
- `operationType`
- `targetFormat` cuando aplique
- `availabilityRules`
- `sizeLimits`
- `executionLimits`
- `expectedQuality`
- `knownLimitations`
- `engine`
- `outputKind`
- `visibilityPolicy`

---

## Ejemplo conceptual

```yaml
id: pdf-to-images
displayName: Convertir PDF a imágenes
sourceFormat: application/pdf
operationType: convert
targetFormat: image/png
availabilityRules:
  - file.pageCount > 0
  - file.isEncrypted == false
sizeLimits:
  maxFileSizeMb: 100
executionLimits:
  timeoutSeconds: 180
expectedQuality: high
knownLimitations:
  - no preserve interactive elements
engine: pdf-rasterizer
outputKind: multi-file-zip
visibilityPolicy: visible
```

---

## Reglas del catálogo

### 1. Declarativo antes que imperativo
Siempre que sea posible, definir capacidades de forma declarativa.

### 2. Explicable
Toda capacidad debe poder explicar por qué aparece o por qué no aparece.

### 3. Trazable
Debe quedar claro qué motor o adaptación técnica la implementa.

### 4. Limitada
Una capacidad sin límites de tamaño, tiempo o complejidad está incompleta.

### 5. Observable
Debe poder medirse su tasa de uso, éxito, error y latencia.

### 6. Orden estable de presentación
Cuando la UI necesite mostrar capacidades en lista plana, el backend debe entregar un `presentationOrder`
estable para evitar que el frontend invente prioridades locales o colapse capacidades distintas que comparten formato de salida.

---

## Tipos sugeridos de operación

- `convert`
- `extract`
- `compress`
- `optimize`
- `preview`
- `validate`
- `package`

No crear nuevos tipos si uno existente cubre bien el caso.

---

## Capacidades implementadas actualmente

Estado alineado con la fuente de verdad del backend:

- `apps/api/internal/capabilities/catalog.go`
- `apps/api/internal/capabilities/resolver.go`
- `apps/api/cmd/server/main.go`
- `apps/api/cmd/worker/main.go`

La visibilidad real de una capacidad sigue dependiendo de:

- formato detectado real del archivo
- límites de tamaño
- protección del archivo
- disponibilidad del engine en runtime
- feature flags operativas

Notas relevantes del comportamiento actual:

- las operaciones `compress` y `preview` pueden mantener el mismo formato de salida
- la restricción de formato igual aplica solo a `convert`
- Markdown se habilita solo cuando la detección por contenido es suficientemente confiable
- HTML detectado como `text/html` entra al flujo documental real sin depender de extensión
- el OCR base actual usa Tesseract y, para PDF, rasteriza páginas antes de reconstruir texto, JSON o PDF searchable
- cuando una salida multi-página o multi-slide termina en ZIP real, el artefacto se persiste con la extensión y MIME reales del archivo generado
- `GET /api/jobs/{jobId}` expone el nombre, MIME y tamaño reales del artefacto cuando la conversión termina, para que frontend y dashboard no tengan que inferir ZIPs ni nombres finales
- `GET /api/catalog` expone el catálogo declarativo agrupado por familia para que la UI y documentación no dupliquen manualmente la fuente de verdad

### PDF

- PDF -> JPG
- PDF -> PNG
- PDF -> DOCX
- PDF -> TXT extraído
- PDF -> PDF comprimido
- PDF -> HTML para preview
- PDF -> OCR a TXT
- PDF -> OCR a JSON por página
- PDF -> PDF searchable tras OCR

### Documentos y texto

- DOC/DOCX/ODT/RTF -> PDF
- DOC/DOCX/ODT/RTF -> TXT
- DOC/ODT/RTF -> DOCX
- DOCX -> HTML simple
- DOCX -> Markdown
- TXT -> PDF simple
- HTML -> PDF
- HTML -> TXT limpio
- Markdown detectado por contenido -> HTML
- Markdown detectado por contenido -> PDF simple
- Markdown detectado por contenido -> DOCX
- PPTX/ODP -> PDF
- PPTX/ODP -> JPG por slide
- PPTX/ODP -> PNG por slide
- XLSX/ODS/CSV -> PDF
- XLSX/ODS -> CSV
- ODS/CSV -> XLSX
- XLSX/ODS/CSV -> HTML

### Imagen

- JPEG/PNG/WebP/GIF/BMP/TIFF -> PNG
- PNG/WebP/GIF/BMP/TIFF -> JPG
- JPEG/PNG -> WebP
- JPEG/PNG -> AVIF
- JPEG/PNG/WebP/GIF/BMP/TIFF -> PDF
- HEIC/HEIF -> JPG
- HEIC/HEIF -> PNG
- HEIC/HEIF -> WebP
- SVG -> PNG
- SVG -> WebP
- SVG -> PDF vectorial
- JPEG -> versión comprimida
- PNG -> versión comprimida
- JPEG -> thumbnail JPG
- PNG -> thumbnail PNG
- imagen -> perfil web JPG 640px
- imagen -> perfil web WebP 640px
- imagen -> perfil web AVIF 640px
- imagen -> perfil web JPG 1600px
- imagen -> perfil web WebP 1600px
- imagen -> perfil web AVIF 1600px
- imagen -> OCR a TXT
- imagen -> OCR a JSON con bloques, líneas y palabras

### Audio

- WAV/OGG/FLAC/AAC -> MP3
- MP3/OGG/FLAC/AAC -> WAV
- MP3/WAV/FLAC/AAC -> OGG
- MP3/WAV/OGG/FLAC -> AAC
- MP3/WAV/OGG/Opus/FLAC/AAC/M4A -> M4A
- MP3/WAV/OGG/AAC -> FLAC
- MP3/WAV/OGG/Opus/FLAC/AAC/M4A -> Opus
- audio -> waveform PNG estático

### Video

- MOV/WEBM/AVI -> MP4
- MP4/MOV/AVI -> WebM
- MP4/MOV/WEBM/AVI -> GIF
- MP4/MOV/WEBM/AVI -> audio MP3
- MP4/MOV/WEBM/AVI -> audio WAV
- MP4/MOV/WEBM/AVI -> audio AAC
- MP4/MOV/WEBM/AVI -> audio M4A
- MP4/MOV/WEBM/AVI -> audio FLAC
- MP4/MOV/WEBM/AVI -> audio Opus
- MP4/MOV/WEBM/AVI -> ZIP de thumbnails JPG
- MP4/MOV/WEBM/AVI -> preview corto MP4
- MP4/MOV/WEBM/AVI -> preview corto WebM
- MP4/MOV/WEBM/AVI -> contact sheet JPG
- MP4/MOV/WEBM/AVI -> waveform PNG estático del audio principal

## Capacidades candidatas para siguientes iteraciones
En esta pasada ya existen perfiles web de imagen, salidas HEIC/HEIF y SVG, audio/video a M4A/Opus y conversión base de presentaciones y hojas de cálculo.
El corpus real ya cubre variantes complejas y corruptas para HEIF, presentaciones, hojas de cálculo, DOC legacy, documentos protegidos OOXML/ODF, ZIP bombs controlados y corruptos por familia base (PDF, documento, imagen, audio y video).
El backlog inmediato queda concentrado en transcripción y subtitulado.

### Audio y video

- audio -> transcripción TXT o JSON si se incorpora STT
- video -> transcripción TXT o JSON si se incorpora STT
- video -> subtítulos SRT o VTT generados automáticamente

### Imagen

- presets web adicionales si producto necesita variantes más pequeñas o placeholders específicos

---

## Campos funcionales recomendados

Además de los mínimos, puede ser útil modelar:

- `requiresPassword`
- `supportsBatchOutput`
- `userFacingDescription`
- `costClass`
- `latencyClass`
- `retentionClass`
- `riskLevel`
- `beta`
- `featureFlag`

---

## Reglas de resolución

El `CapabilityResolver` debería considerar al menos:

- tipo real del archivo
- metadatos relevantes
- límites de tamaño
- protección o cifrado
- disponibilidad del motor
- flags del entorno
- plan del usuario si aplica en el futuro

No debería depender de la UI ni de detalles de presentación.

---

## Qué no debe pasar

- una capacidad visible en frontend que backend no reconoce
- una capacidad implementada por un worker que el catálogo no declara
- lógica de elegibilidad repetida en tres sitios
- opciones mostradas “porque sí” sin una razón trazable

---

## Checklist para agregar una nueva capacidad

1. definir la capacidad en la fuente de verdad
2. documentar sus límites y restricciones
3. implementar o registrar el motor responsable
4. exponerla vía API
5. reflejarla en UI
6. añadir tests de elegibilidad
7. añadir tests de ejecución
8. añadir observabilidad mínima

---

## Preguntas que toda capacidad debe responder

- ¿sobre qué formatos aplica?
- ¿qué hace exactamente?
- ¿cuándo está disponible?
- ¿cuándo no está disponible?
- ¿qué motor la ejecuta?
- ¿qué calidad se espera?
- ¿qué salida produce?
- ¿qué riesgos o limitaciones tiene?
