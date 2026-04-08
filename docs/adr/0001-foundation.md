# ADR 0001: Fundaciones arquitectónicas del sistema

## Estado

Propuesto

## Contexto

El proyecto implementa un servicio web de conversión de archivos inteligente.
El principal riesgo de este tipo de producto es que la lógica de formatos, seguridad, jobs y almacenamiento termine mezclada.

Eso produciría:

- deuda técnica temprana
- acoplamiento alto
- baja capacidad de evolución
- errores difíciles de depurar
- superficie de seguridad mal controlada

## Decisión

Se adoptan estas decisiones fundacionales:

1. Modelar el sistema como pipeline de ingestión, capacidades y jobs.
2. Separar detección de formato de ejecución de conversiones.
3. Mantener un catálogo central de capacidades.
4. Ejecutar conversiones como jobs asíncronos por defecto.
5. Tratar el archivo original como inmutable.
6. Aislar workers y motores con límites explícitos.
7. Mantener observabilidad desde el inicio.
8. Favorecer V1 pequeña y sólida antes que amplitud prematura.

## Alternativas consideradas

### A. Backend monolítico con lógica embebida por endpoint
Ventaja:
- implementación rápida al comienzo

Desventajas:
- duplicación de reglas
- difícil trazabilidad
- poca escalabilidad operativa
- alta deuda técnica

### B. Workers decidiendo directamente capacidades
Ventaja:
- aparente simplicidad local

Desventajas:
- frontend y API no pueden explicar bien las opciones
- verdad funcional dispersa
- mayor incoherencia del producto

### C. Pipeline explícito con catálogo central y jobs
Ventajas:
- claridad del dominio
- mejor mantenibilidad
- escalabilidad razonable
- mejor observabilidad
- mejor aislamiento

## Consecuencias

### Positivas
- sistema más fácil de entender
- incorporación ordenada de nuevas capacidades
- mejor control de seguridad
- mejor operación e incident response

### Negativas
- mayor inversión inicial en modelado
- más disciplina requerida para mantener fronteras
- más documentación desde el inicio

## Notas

Este ADR define la base.
Nuevas decisiones sobre storage, colas, multi-tenant, versionado de artefactos o sandbox deben documentarse en ADRs posteriores.
