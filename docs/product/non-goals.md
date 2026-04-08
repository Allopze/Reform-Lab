# No objetivos del producto

## Propósito

Este documento define lo que el producto **no** intenta resolver por ahora.

Es una defensa contra el alcance difuso y contra la tendencia de agentes o colaboradores a introducir complejidad no pedida.

---

## No objetivos iniciales sugeridos

### 1. No soportar todos los formatos
El producto no debe intentar cubrir todo el universo de archivos en V1.

### 2. No encadenar conversiones arbitrarias
No permitir secuencias libres del tipo:
A -> B -> C -> D
salvo diseño explícito.

### 3. No preservar el 100% de semántica o formato en todos los casos
Algunas conversiones priorizarán legibilidad o extracción sobre fidelidad total.

### 4. No incorporar IA generativa sin un caso claro
El producto base no necesita sumar “IA” a cada paso del pipeline.

### 5. No ofrecer edición compleja de documentos
El foco es conversión y procesamiento, no colaboración ni edición avanzada.

### 6. No exponer infraestructura o motores como producto
Los usuarios piden resultados, no acceso a herramientas internas.

### 7. No prometer compatibilidad con archivos dañados o protegidos
Puede existir manejo parcial, pero no debe asumirse soporte universal.

### 8. No optimizar prematuramente para hiperescala
Diseñar para crecer, sí.
Construir para tráfico monstruoso imaginario desde el día uno, no.

---

## Regla práctica

Si una propuesta agrega complejidad y no mejora claramente:
- claridad del producto
- seguridad
- mantenibilidad
- trazabilidad
- calidad de conversión

entonces no pertenece todavía al roadmap.
