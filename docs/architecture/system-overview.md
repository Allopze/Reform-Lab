# Visión general del sistema

## Resumen

El sistema es una plataforma de procesamiento de archivos enfocada en mostrar al usuario únicamente operaciones coherentes con el tipo real del archivo y con las capacidades del producto.

No debe modelarse como una simple lista de conversiones.
Debe modelarse como un pipeline controlado de ingestión, detección, resolución de capacidades y ejecución de jobs.

## Flujo de alto nivel

1. El usuario sube un archivo.
2. El sistema valida la subida.
3. El sistema detecta el formato real.
4. El sistema extrae metadatos relevantes.
5. El sistema resuelve capacidades disponibles.
6. El usuario selecciona una operación.
7. Se crea una solicitud de conversión.
8. Se crea y encola un job.
9. Un worker ejecuta la operación.
10. El sistema valida la salida.
11. El artefacto generado se persiste.
12. El usuario puede descargarlo o consultar su estado.

## Bloques principales

### 1. Frontend
Responsable de:

- subida del archivo
- presentación de capacidades
- seguimiento del estado
- descarga de artefactos
- manejo claro de errores de usuario

### 2. API
Responsable de:

- autenticación y autorización
- validación de inputs
- exposición de contratos
- creación de solicitudes de conversión
- consulta de estados y artefactos

### 3. Ingestion
Responsable de:

- validación de archivo
- detección de tipo real
- extracción de metadatos
- aplicación de límites básicos

### 4. Capabilities
Responsable de:

- resolver qué operaciones aplican
- explicar por qué aplican o no aplican
- vincular capacidades con motores
- definir límites y restricciones

### 5. Orchestrator
Responsable de:

- crear jobs
- encolar trabajos
- manejar transiciones de estado
- reintentos
- cancelaciones
- expiraciones

### 6. Workers
Responsables de:

- ejecutar conversiones
- aplicar límites de ejecución
- validar salida
- reportar resultados

### 7. Storage
Responsable de:

- originales
- temporales
- artefactos
- previews si existen
- acceso a blobs y metadatos asociados

### 8. Observability
Responsable de:

- logs estructurados
- métricas
- trazas
- eventos de auditoría

## Principios estructurales

### Separación de responsabilidades
No mezclar:

- detección con conversión
- conversión con política de producto
- storage con lógica de dominio
- UI con verdad funcional

### Asincronía por defecto
Las conversiones deben modelarse como jobs asíncronos aunque algunas se puedan resolver rápido.

### Original inmutable
El archivo original no se modifica.
Los resultados derivados son artefactos nuevos.

### Fuente de verdad de capacidades
La lógica que decide qué puede hacerse con un archivo debe centralizarse.

## Ejemplo de flujo conceptual

```text
Upload
  -> Validate
  -> Detect real format
  -> Extract metadata
  -> Resolve capabilities
  -> User chooses action
  -> Create conversion request
  -> Enqueue job
  -> Worker executes
  -> Validate output
  -> Store artifact
  -> Notify or expose result
```

## Riesgos de arquitectura a evitar

- lógica de capacidades repartida por tres capas
- endpoints que hacen procesamiento pesado síncrono
- workers que deciden reglas de negocio
- storage usado como fuente de verdad del dominio
- ausencia de límites de ejecución
- temporales sin política de limpieza
- contratos públicos implícitos o inconsistentes

## Decisiones iniciales sugeridas

- pocas capacidades en V1
- alta calidad y trazabilidad
- pipeline explícito
- catálogo de capacidades central
- modelo de jobs claro
- observabilidad desde la primera versión
