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

## Capacidades sugeridas para una V1

### PDF
- PDF -> imágenes
- PDF -> texto extraído
- PDF -> texto plano
- PDF -> versión comprimida

### DOCX
- DOCX -> PDF
- DOCX -> TXT
- DOCX -> HTML simple

### Imagen
- JPG/PNG -> WebP
- imagen -> comprimida
- imagen -> thumbnail
- imagen -> OCR si se soporta explícitamente en el futuro

### TXT / Markdown
- TXT -> PDF simple
- Markdown -> HTML
- Markdown -> PDF simple

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
