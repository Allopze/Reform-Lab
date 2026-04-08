# PROJECT_GUIDELINES_AI

## Propósito

Este documento define las reglas de trabajo del proyecto para cualquier agente de IA o colaborador con acceso al repositorio.

El objetivo principal del sistema es ofrecer un **servicio web de conversión de archivos inteligente**, donde el usuario sube un archivo y la aplicación muestra únicamente opciones coherentes con el formato detectado, sus capacidades reales y las políticas del sistema.

Estas guías existen para mantener el proyecto:

- claro
- modular
- seguro
- mantenible
- observable
- escalable
- con baja deuda técnica

Cuando exista duda entre velocidad y claridad, se prioriza la claridad.
Cuando exista duda entre conveniencia y seguridad, se prioriza la seguridad.
Cuando exista duda entre una solución rápida y una solución extensible, se prioriza la extensibilidad razonable, no la sobreingeniería.

---

## Instrucción principal para agentes de IA

Antes de modificar código, una IA debe asumir lo siguiente:

1. No debe inventar arquitectura.
2. No debe introducir patrones nuevos sin justificarlo.
3. No debe mezclar responsabilidades entre capas.
4. No debe añadir complejidad para resolver un problema pequeño.
5. No debe romper compatibilidad sin dejarlo explícito.
6. No debe tocar archivos no relacionados con la tarea.
7. No debe reescribir módulos completos si un cambio local es suficiente.
8. No debe asumir el tipo real de un archivo a partir de su extensión.
9. No debe introducir atajos de seguridad, aunque aceleren la entrega.
10. No debe ignorar tests, logs, validaciones, límites o políticas de retención.

Toda modificación debe ser pequeña, reversible, explicable y consistente con el diseño existente.

---

## Qué es este sistema

El sistema **no** es simplemente una web que convierte archivos.

El sistema es una plataforma de procesamiento de archivos con estos bloques conceptuales:

- ingestión de archivos
- detección real de formato
- extracción de metadatos
- resolución de capacidades disponibles
- ejecución de conversiones
- persistencia de artefactos
- seguimiento de jobs
- auditoría
- retención y limpieza

Toda decisión de arquitectura y todo cambio de código deben respetar esta separación.

---

## Principios de arquitectura

### 1. Separación de responsabilidades

El proyecto debe separar claramente:

- interfaz de usuario
- API de producto
- lógica de dominio
- orquestación de jobs
- motores de conversión
- almacenamiento
- observabilidad
- seguridad y validación

No mezclar en un mismo módulo:

- lógica de UI con reglas de negocio
- lógica de negocio con acceso directo a infraestructura
- detección de archivos con ejecución de conversiones
- conversión con facturación, permisos o analítica

### 2. El dominio manda

El código debe hablar en términos del dominio, no en términos de hacks de implementación.

Conceptos base:

- `OriginalFile`
- `DetectedFormat`
- `Capability`
- `ConversionRequest`
- `Job`
- `Artifact`
- `AuditEvent`
- `RetentionPolicy`

Si aparece lógica importante fuera de estos conceptos, debe revisarse.

### 3. Declarativo antes que disperso

Las capacidades de conversión deben definirse de forma declarativa siempre que sea posible.

Ejemplo conceptual:

- formato origen
- formato destino
- nombre visible
- condiciones de disponibilidad
- límites de tamaño
- limitaciones conocidas
- coste estimado
- latencia estimada
- motor responsable
- nivel de calidad esperado

Evitar `if/else` repetidos repartidos por UI, API y workers.

### 4. Asincronía por defecto

Las conversiones deben modelarse como jobs asíncronos, aunque algunas puedan resolverse rápido.

Razones:

- escalabilidad
- reintentos
- cancelación
- observabilidad
- control de errores
- aislamiento

Las APIs no deben depender de mantener procesos largos abiertos innecesariamente.

### 5. Original inmutable

El archivo subido por el usuario es una fuente de verdad y no debe alterarse.

Los resultados derivados deben almacenarse como artefactos separados y versionados cuando aplique.

---

## Reglas de diseño del dominio

### Detección no es conversión

Detectar el archivo y convertirlo son responsabilidades distintas.

Toda IA debe mantener esta secuencia mental:

1. recibir archivo
2. validar subida
3. detectar tipo real
4. extraer metadatos útiles
5. resolver capacidades aplicables
6. crear solicitud de conversión
7. ejecutar job
8. validar salida
9. persistir artefacto
10. registrar auditoría

### Las extensiones no son fuente confiable

No usar solamente nombre de archivo o extensión para decidir comportamiento.

Siempre priorizar:

- firma del archivo
- MIME detectado internamente
- parsers seguros
- metadatos verificables

### Toda capacidad debe poder explicarse

Si la UI ofrece una opción, debe haber una razón explícita y trazable.

Si la UI no ofrece una opción, también debe haber una razón explícita.

Nunca ocultar lógica de producto en condicionales opacos.

---

## Estructura esperada del código

La estructura exacta del repo puede variar, pero debe conservar estas fronteras lógicas:

```text
/apps
  /web                -> frontend
  /api                -> API pública o backend-for-frontend

/packages or /services
  /domain             -> entidades, reglas, contratos
  /capabilities       -> registro y resolución de capacidades
  /ingestion          -> validación, detección, metadatos
  /orchestrator       -> creación y gestión de jobs
  /workers            -> ejecución de conversiones
  /storage            -> acceso a blobs, temporales, artefactos
  /security           -> validaciones, límites, escaneo, sanitización
  /observability      -> logs, métricas, tracing
  /shared             -> utilidades realmente compartidas

/docs
  /adr                -> decisiones arquitectónicas
  /runbooks           -> operación y resolución de incidentes
  /product            -> reglas funcionales
  /security           -> políticas y controles
```

Si el repositorio usa otro esquema, debe existir documentación que mapee la estructura real a estos límites.

---

## Convenciones de cambio para IA

Antes de hacer cambios, una IA debe leer, en este orden si existen:

1. `README`
2. `docs/adr/*`
3. documentación de arquitectura o dominio
4. configuración del workspace
5. manifiestos de dependencias
6. tests relacionados
7. código existente del módulo objetivo

Antes de crear archivos nuevos, confirmar que no exista ya un lugar correcto para esa responsabilidad.

### Una IA no debe

- crear utilidades genéricas prematuramente
- duplicar tipos o contratos ya definidos
- mover archivos masivamente sin necesidad
- cambiar nombres públicos sin razón fuerte
- introducir abstracciones por anticipación vaga
- añadir dependencias grandes para resolver problemas pequeños
- mezclar refactor con cambio funcional sin dejarlo claro

### Una IA sí debe

- reutilizar patrones ya presentes
- mantener cambios pequeños
- añadir tests donde cambia comportamiento
- documentar decisiones relevantes
- dejar mensajes de error claros
- preservar compatibilidad cuando sea posible
- mejorar nombres si el cambio ya toca código confuso

---

## Estándares de código

### Código legible primero

Preferencias generales:

- funciones cortas con propósito claro
- nombres explícitos
- tipos claros
- pocos parámetros
- sin efectos colaterales ocultos
- errores manejados de forma consistente
- validaciones cerca del borde del sistema

### Prohibido

- comentarios que repiten el código
- utilidades “misc”, “helpers” o “common” sin criterio
- lógica de negocio en controladores si debe vivir en dominio
- queries gigantes incrustadas en múltiples lugares
- constantes mágicas repetidas
- estados ambiguos como `done`, `ok`, `bad` sin definición formal

### Recomendado

- enums o tipos cerrados para estados de jobs
- contratos explícitos para requests y responses
- funciones puras para resolución de capacidades
- adaptadores para infraestructura externa
- validadores en el borde de entrada

---

## Política de errores

Todo error debe ser:

- clasificable
- observable
- seguro para exponer o no exponer
- útil para depuración

Separar al menos entre:

- error de validación del input
- error de formato no soportado
- error de permiso
- error de límite excedido
- error transitorio de infraestructura
- error del motor de conversión
- error de salida inválida

No devolver mensajes genéricos cuando el sistema conoce la causa.
No filtrar detalles internos sensibles a usuarios finales.

---

## Seguridad

La seguridad de archivos es parte del núcleo del producto.

### Reglas obligatorias

1. Validar por allowlist de formatos soportados.
2. Detectar tipo real del archivo.
3. Limitar tamaño, tiempo, memoria y número de páginas o frames cuando aplique.
4. Renombrar archivos del lado servidor.
5. No confiar en nombres enviados por usuario.
6. Almacenar originales y temporales fuera de rutas públicas directas.
7. Aislar motores de conversión.
8. Escanear archivos o aplicar controles equivalentes si el riesgo lo exige.
9. Expirar temporales automáticamente.
10. Registrar eventos críticos de seguridad y fallos de validación.

### Regla crítica

Nunca ejecutar herramientas de conversión con permisos amplios por conveniencia.

Todo motor debe correr con:

- mínimos privilegios
- límites de CPU
- límites de memoria
- límites de disco
- timeout explícito
- filesystem acotado
- acceso de red restringido salvo necesidad justificada

---

## Política de almacenamiento

Separar claramente:

- originales
- temporales
- artefactos generados
- previews
- logs
- metadatos en base de datos

### Reglas

- los originales son inmutables
- los temporales tienen TTL obligatorio
- los artefactos deben poder trazarse a un job y a un original
- no almacenar contenido sensible en logs
- no mezclar metadata operacional con blobs

---

## Jobs y workers

Toda ejecución de conversión debe tener un estado explícito.

Estados mínimos sugeridos:

- `queued`
- `running`
- `succeeded`
- `failed`
- `cancelled`
- `expired`

### Reglas

- cada job debe ser idempotente cuando sea posible
- los reintentos deben ser controlados
- los errores permanentes no deben reintentarse indefinidamente
- toda transición de estado debe ser trazable
- los workers no deben contener reglas de producto que correspondan a la capa de capacidades

---

## Registro de capacidades

El sistema debe tener una fuente de verdad para responder:

- qué opciones mostrar al usuario
- por qué una opción existe
- qué motor la resuelve
- qué limitaciones tiene
- qué calidad esperar

Toda IA debe preferir modificar el registro de capacidades antes que propagar condiciones duplicadas por varias capas.

Si una nueva conversión se agrega al producto, el cambio ideal toca:

1. definición de capacidad
2. validaciones asociadas
3. motor o adapter responsable
4. tests
5. documentación funcional si cambia experiencia de usuario

---

## API y contratos

Las APIs deben ser estables, tipadas y versionables.

### Reglas

- no cambiar respuestas públicas sin revisar compatibilidad
- usar contratos explícitos
- documentar campos opcionales y obligatorios
- modelar errores de forma consistente
- evitar respuestas ambiguas o implícitas

### Recomendación

Toda entidad pública relevante debe tener identificadores estables y estados definidos formalmente.

---

## Base de datos y migraciones

### Reglas de migración

- toda migración debe ser revisable y reversible cuando sea posible
- no mezclar cambios de esquema con refactors irrelevantes
- evitar migraciones destructivas sin plan de transición
- preferir cambios compatibles hacia adelante
- documentar migraciones de alto riesgo

### Reglas de modelo

No colapsar todo en tablas genéricas difíciles de mantener.

Entidades sugeridas:

- archivos
- detecciones
- solicitudes de conversión
- jobs
- artefactos
- eventos de auditoría
- cuotas o suscripciones
- políticas de retención

---

## Testing

Un cambio sin test solo es aceptable si no cambia comportamiento y la razón queda clara.

### Capas mínimas

1. unit tests para reglas de dominio
2. integration tests para almacenamiento, cola y persistencia
3. contract tests para API
4. pruebas end-to-end de flujos críticos
5. corpus de archivos reales y corruptos

### Corpus obligatorio

El proyecto debe mantener muestras representativas de:

- archivos válidos
- archivos corruptos
- archivos vacíos
- archivos gigantes
- extensiones engañosas
- archivos protegidos
- formatos parcialmente soportados
- variantes reales del mismo formato

Una IA no debe añadir soporte a un formato sin ampliar el corpus de pruebas correspondiente.

---

## Observabilidad

Sin observabilidad no hay mantenimiento real.

Todo servicio crítico debe emitir:

- logs estructurados
- métricas
- trazas si la plataforma las soporta

### Métricas mínimas sugeridas

- latencia de detección
- latencia de conversión
- tasa de éxito por conversión origen-destino
- tasa de error por motor
- tamaño de cola
- cantidad de reintentos
- uso de CPU y memoria por worker
- ratio de archivos rechazados
- ratio de salidas inválidas

### Regla

No loggear contenido completo de archivos ni datos sensibles del usuario.

---

## Rendimiento y coste

No optimizar prematuramente, pero sí diseñar para medir.

### Toda IA debe considerar

- coste por conversión
- consumo de CPU y memoria
- presión sobre storage
- impacto de archivos grandes
- fan-out de colas
- deduplicación potencial
- caché segura cuando aplique

No introducir procesos costosos en el request path si pueden moverse a jobs asíncronos.

---

## Dependencias

### Política

Toda nueva dependencia debe justificarse por:

- valor claro
- madurez razonable
- mantenimiento activo
- impacto de seguridad aceptable
- reducción real de complejidad

No añadir dependencias para resolver problemas triviales.
No duplicar librerías que ya existen en el repo.
No acoplar el dominio a una librería específica si puede evitarse.

---

## Documentación

La documentación no es opcional cuando cambia el comportamiento del sistema.

### Debe actualizarse cuando aplique

- README del módulo
- ADR si cambia arquitectura o patrón
- documentación de API
- runbooks operativos
- guía de onboarding técnico
- ejemplos de configuración

### Regla para IA

Si el cambio obliga a explicar algo en una review humana, probablemente también exige documentación.

---

## ADRs

Crear un ADR cuando se tome una decisión estructural sobre:

- colas
- storage
- formato de contratos
- seguridad de ejecución
- estrategia de workers
- multi-tenant
- versionado de artefactos
- sistema de capacidades
- observabilidad estándar

Formato mínimo del ADR:

- contexto
- decisión
- alternativas consideradas
- consecuencias

---

## Regla de compatibilidad y evolución

Cambios grandes deben introducirse por fases.

### Preferir

- feature flags
- doble escritura temporal si es necesario
- migraciones graduales
- adapters de compatibilidad
- deprecación explícita

### Evitar

- rewrites completos sin transición
- cambios de contrato invisibles
- borrar código antiguo sin verificar usos

---

## UX funcional mínima del producto

La experiencia de usuario debe responder de forma clara a estas preguntas:

- qué archivo subí realmente
- qué opciones tengo disponibles
- por qué esas opciones sí están disponibles
- por qué otras no
- cuánto puede tardar
- si terminó bien o mal
- dónde está el resultado
- cuánto tiempo estará disponible

Una IA que cambie UI o API no debe degradar esta claridad.

---

## Qué priorizar en V1

El proyecto debe empezar pequeño y sólido.

Prioridades:

1. subida segura de archivos
2. detección real de formato
3. resolución de capacidades coherentes
4. pipeline de jobs asíncronos
5. pocas conversiones, pero confiables
6. trazabilidad completa
7. expiración de temporales
8. logs, métricas y manejo serio de errores

No priorizar de inicio:

- demasiados formatos
- automatizaciones complejas
- cadenas arbitrarias de conversión
- IA generativa sin necesidad real
- personalización excesiva
- optimizaciones prematuras

---

## Checklist antes de mergear cambios

Toda IA debe verificar:

- el cambio respeta la separación de capas
- no introduce lógica duplicada
- los tipos y estados siguen siendo coherentes
- hay tests suficientes
- los errores son claros
- no se debilita seguridad
- no se rompen contratos públicos sin declararlo
- se actualizó documentación si hacía falta
- el cambio es pequeño y entendible
- existe una ruta clara de rollback o mitigación

---

## Checklist específico para nuevas conversiones

Antes de agregar un nuevo par origen-destino, verificar:

- el formato origen está realmente soportado
- existe detección confiable
- la capacidad está registrada declarativamente
- el motor es aislable y seguro
- hay límites razonables de tamaño y tiempo
- la calidad de salida es aceptable
- la salida se valida
- existen archivos de prueba reales
- la UI puede explicar la opción
- la operación es observable

---

## Anti-patrones prohibidos

No introducir ninguno de los siguientes anti-patrones:

- lógica de capacidades duplicada entre frontend y backend sin fuente común
- workers que deciden reglas de producto
- endpoints que hacen trabajo pesado síncrono por comodidad
- utilidades compartidas sin ownership
- enums abiertos sin documentación
- estados implícitos derivados de nulls
- temporales sin limpieza automática
- logs con contenido sensible
- confianza en extensiones de archivo
- refactors masivos no solicitados
- PRs gigantes que mezclan arquitectura, lógica y estilo

---

## Contrato operativo para agentes de IA

Cuando una IA reciba una tarea dentro de este repositorio, debe seguir este orden:

1. entender el requerimiento
2. localizar la capa correcta
3. leer el código existente relacionado
4. identificar contratos y tests afectados
5. proponer el cambio mínimo suficiente
6. implementar respetando el dominio
7. añadir o actualizar tests
8. actualizar documentación si aplica
9. revisar impacto en seguridad, observabilidad y compatibilidad
10. dejar el repositorio más claro que antes

Si no está claro dónde debe vivir el cambio, la IA debe detener la expansión del cambio y resolver primero esa frontera conceptual.

---

## Regla final

Este proyecto favorece decisiones aburridas, explícitas y mantenibles.

La meta no es impresionar con complejidad.
La meta es que dentro de meses cualquier persona o agente pueda:

- entender el sistema rápido
- cambiarlo sin miedo
- operar incidentes sin adivinar
- agregar capacidades sin romper lo existente
- mantener un estándar alto de seguridad y calidad

Si una solución parece ingeniosa pero hace más difícil razonar sobre el sistema, probablemente no pertenece aquí.

