# Estrategia de testing

## Propósito

Este documento define cómo probar el sistema de forma suficiente para sostener evolución segura y baja deuda técnica.

En un sistema de conversión de archivos, probar solo funciones sueltas no alcanza.
Se necesita confianza tanto en reglas de negocio como en integración real con archivos y motores.

---

## Objetivos del testing

- detectar regresiones funcionales
- validar reglas del dominio
- asegurar contratos API
- detectar errores de integración
- validar outputs de conversión
- controlar casos límite y archivos corruptos

---

## Pirámide mínima de pruebas

### 1. Unit tests
Para:

- resolución de capacidades
- validaciones del dominio
- estados de job
- mapeos de errores
- políticas de retención

Deben ser rápidos, aislados y legibles.

### 2. Integration tests
Para:

- base de datos
- cola de jobs
- storage
- workers y adaptadores
- validación de flujos entre módulos

### 3. Contract tests
Para:

- endpoints API
- serialización
- errores
- estados públicos
- compatibilidad de contratos

### 4. End-to-end
Para flujos críticos:

- subir archivo
- ver capacidades
- lanzar conversión
- consultar estado
- descargar artefacto
- manejar error visible al usuario

---

## Corpus de archivos

El proyecto debe mantener un corpus de prueba por formato soportado.

### Tipos mínimos de muestra
- válido simple
- válido complejo
- corrupto
- vacío
- tamaño grande
- extensión engañosa
- protegido o cifrado si el formato lo permite
- variante real frecuente del ecosistema

### Regla
No declarar soporte a un formato sin corpus correspondiente.

---

## Qué validar en outputs

No basta con que el worker termine sin excepción.
Debe validarse, según la capacidad:

- existencia del artefacto
- tipo de salida esperado
- integridad básica
- legibilidad mínima
- cantidad de páginas o archivos esperada
- preservación mínima necesaria del contenido

---

## Testing por cambios

### Bug fix
Debe incluir al menos:
- test que reproduzca el bug
- test que pruebe el comportamiento corregido

### Nueva capacidad
Debe incluir:
- test de elegibilidad
- test del caso feliz
- test de error o límites
- test de validación de salida

### Cambio de contrato
Debe incluir:
- tests de contrato
- revisión de compatibilidad
- actualización de snapshots o fixtures si existen

---

## Qué evitar

- tests demasiado acoplados a implementación interna
- snapshots enormes difíciles de revisar
- casos felices sin errores ni límites
- fixtures imposibles de entender
- corpus desordenado sin naming consistente

---

## Naming sugerido del corpus

```text
tests/fixtures/
  pdf/
    valid-basic.pdf
    valid-multipage.pdf
    corrupted-truncated.pdf
    encrypted-sample.pdf
  docx/
    valid-basic.docx
    corrupted-broken-zip.docx
  image/
    valid-photo.jpg
    valid-transparent.png
    oversized-sample.png
```

---

## Regla final

Un cambio en el sistema debe dejar más confianza que antes, no menos.
Si el cambio amplía complejidad, la cobertura también debe ampliarse.
