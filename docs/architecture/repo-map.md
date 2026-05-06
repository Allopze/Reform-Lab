# Mapa del repositorio

## Propósito

Este documento describe dónde debería vivir cada tipo de responsabilidad dentro del repositorio.

No es un documento de producto.
Es un documento de navegación y ownership técnico.

## Estructura lógica recomendada

```text
/
├─ apps/
│  ├─ web/
│  └─ api/
├─ services/
│  ├─ domain/
│  ├─ capabilities/
│  ├─ ingestion/
│  ├─ orchestrator/
│  ├─ workers/
│  ├─ storage/
│  ├─ security/
│  └─ observability/
├─ docs/
│  ├─ adr/
│  ├─ architecture/
│  ├─ domain/
│  ├─ security/
│  ├─ testing/
│  ├─ operations/
│  ├─ api/
│  └─ product/
└─ .github/
   ├─ copilot-instructions.md
   └─ instructions/
```

## Qué debe vivir en cada zona

### `apps/web`
- componentes UI
- pantallas
- flujos de usuario
- adaptadores a API
- manejo de estado de cliente

No debería contener:
- verdad final de capacidades
- parsing profundo de archivos
- reglas nucleares del dominio

### `apps/api`
- endpoints
- serialización
- autenticación
- validación de entrada
- composición de casos de uso

No debería contener:
- lógica de conversión concreta
- lógica duplicada del catálogo de capacidades

### `docs/api`
- contratos HTTP
- envelopes de respuesta
- reglas de compatibilidad para clientes
- ejemplos de consumo de API

### `services/domain`
- entidades
- value objects
- estados
- reglas de negocio
- contratos conceptuales

Debe ser una capa estable y entendible.

### `services/capabilities`
- catálogo de capacidades
- reglas de elegibilidad
- restricciones por formato
- explicación de opciones disponibles

### `services/ingestion`
- validación de archivo
- detección real
- extracción de metadatos
- clasificación inicial

### `services/orchestrator`
- creación de jobs
- estados
- reintentos
- cancelación
- scheduling si existe

### `services/workers`
- adaptadores a motores
- ejecución de herramientas
- validación de salida
- reporting de estado

### `services/storage`
- blobs
- temporales
- artefactos
- naming interno
- TTL
- abstracciones de acceso

### `services/security`
- políticas de validación
- límites
- sanitización
- aislamiento
- controles y validadores reutilizables

### `services/observability`
- logger compartido
- convenciones de métricas
- trazas
- helper de auditoría

## Reglas de ubicación

Antes de crear un archivo nuevo, preguntar:

1. ¿Existe ya un módulo con esta responsabilidad?
2. ¿El cambio pertenece al dominio o a la infraestructura?
3. ¿Se está creando un helper genérico solo porque no se encontró el lugar correcto?
4. ¿El archivo nuevo mejora la organización o fragmenta innecesariamente?

## Señales de mala ubicación

- para entender una regla hay que mirar tres carpetas no relacionadas
- la UI decide cosas que el backend no sabe explicar
- un worker contiene reglas de producto
- un controlador conoce demasiados detalles de storage
- una utilidad compartida crece sin ownership claro

## Ownership sugerido

Cada carpeta principal debería tener:

- un README corto
- tests cercanos
- límites explícitos
- responsables técnicos definidos cuando el equipo crezca
