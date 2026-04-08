---
applyTo: "apps/web/**"
---

# Instrucciones para frontend

## Enfoque

El frontend representa el estado del sistema y guía al usuario.
No debe convertirse en la fuente de verdad de las reglas de negocio.

## Reglas

- consumir capacidades y estados desde contratos del backend
- mostrar con claridad qué opciones están disponibles y por qué
- hacer visibles límites relevantes cuando afecten UX
- reflejar estados de job de forma consistente
- mantener componentes simples y con responsabilidades claras

## No hacer

- inventar reglas finales sobre formatos
- decidir soporte de conversión con condicionales locales opacos
- esconder errores importantes
- duplicar validaciones críticas que solo existen en backend
- mezclar lógica de UI con parsing de archivos o seguridad

## UX mínima esperada

El usuario debe entender:

- qué archivo se detectó
- qué opciones hay
- por qué hay o no hay opciones
- si la conversión está en cola, corriendo, falló o terminó
- cuánto tiempo estará disponible el artefacto

## Testing esperado

- tests de componentes críticos
- tests de flujos principales
- cobertura de estados de error y carga
