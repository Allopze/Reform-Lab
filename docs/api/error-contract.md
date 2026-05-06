# Contrato de errores API

## Proposito

Este documento define el contrato JSON que deben usar los clientes de la API al manejar errores.

El objetivo es que la UI, clientes externos y futuras APIs publicas no dependan de parsear texto libre para tomar decisiones de producto.

## Envelope de error

Toda respuesta JSON de error emitida por handlers HTTP debe usar esta forma:

```json
{
  "error": "too many active jobs for this user",
  "code": "too_many_active_jobs_for_this_user",
  "message": "too many active jobs for this user",
  "requestId": "01HF7...",
  "retryable": true
}
```

Campos:

| Campo | Tipo | Obligatorio | Descripcion |
| --- | --- | --- | --- |
| `error` | string | si | Alias legado de `message`. Se conserva por compatibilidad con clientes existentes. |
| `code` | string | si | Codigo estable para logica de cliente, analitica y soporte. |
| `message` | string | si | Mensaje legible para logs o fallback de UX. No debe exponer detalles internos sensibles. |
| `requestId` | string | no | Identificador de correlacion cuando el middleware de request lo haya asignado. |
| `retryable` | boolean | si | Indica si el cliente puede sugerir reintento sin interpretar el status code. |

## Reglas de compatibilidad

- Los clientes nuevos deben usar `code` para branching y `message` solo como fallback legible.
- Los clientes existentes que leen `error` siguen funcionando.
- No cambiar el significado de un `code` publicado. Si cambia la condicion, crear un codigo nuevo.
- No usar datos de usuario, nombres de archivo, IDs internos ni paths dentro de `code`.
- Mantener `message` generico cuando el detalle pueda filtrar informacion sensible.

## Generacion de `code`

Actualmente `respondError` genera `code` a partir del mensaje normalizado:

1. trim y lowercase
2. remocion de puntuacion simple
3. separacion por espacios
4. union con `_`
5. truncado a 64 caracteres

Si el mensaje esta vacio, se usa un fallback por status HTTP como `bad_request`, `unauthorized`, `forbidden`, `not_found`, `conflict`, `rate_limited`, `limit_exceeded`, `service_unavailable`, `internal_error` o `request_failed`.

Como consecuencia, cambiar el texto pasado a `respondError` tambien cambia el `code`. Para endpoints publicos, tratar esos mensajes como parte del contrato hasta que exista un registro explicito de codigos.

## Semantica de `retryable`

`retryable` debe ser `true` para:

- `408 Request Timeout`
- `429 Too Many Requests`
- cualquier status `5xx`

Debe ser `false` para errores de validacion, autorizacion, ownership, formato no soportado, artefactos expirados y conflictos que requieren accion del usuario.

## Guia para clientes

Flujo recomendado:

1. Si la respuesta no es `ok`, intentar parsear JSON.
2. Si existe `code`, usarlo para decidir UX, metricas y soporte.
3. Si `retryable` es `true`, permitir reintento o mostrar un mensaje temporal.
4. Si no hay JSON valido, usar status HTTP y un fallback local generico.
5. Registrar `requestId` en reportes de soporte cuando exista.

Ejemplo TypeScript:

```ts
type ApiError = {
  error: string;
  code: string;
  message: string;
  requestId?: string;
  retryable: boolean;
};

async function readApiError(response: Response): Promise<ApiError> {
  try {
    const body = await response.json();
    return {
      error: String(body.error ?? body.message ?? "request failed"),
      code: String(body.code ?? "request_failed"),
      message: String(body.message ?? body.error ?? "request failed"),
      requestId: typeof body.requestId === "string" ? body.requestId : undefined,
      retryable: Boolean(body.retryable),
    };
  } catch {
    return {
      error: "request failed",
      code: response.status >= 500 ? "internal_error" : "request_failed",
      message: "request failed",
      retryable: response.status === 408 || response.status === 429 || response.status >= 500,
    };
  }
}
```

## Codigos frecuentes

Estos codigos existen por los mensajes actuales de la API y son utiles para UX:

| Codigo | Status comun | Uso esperado |
| --- | --- | --- |
| `invalid_request_body` | 400 | Payload JSON invalido o incompleto. |
| `not_authenticated` | 401 | Sesion ausente o expirada. |
| `forbidden` | 403 | Usuario sin permiso sobre el recurso. |
| `file_not_found` | 404 | Archivo inexistente o no visible para el usuario. |
| `artifact_expired` | 410 | Artefacto eliminado por politica de retencion. |
| `format_not_supported` | 422 | El formato real del archivo no esta soportado. |
| `file_appears_empty_or_corrupted` | 422 | Archivo vacio, corrupto o no inspeccionable con seguridad. |
| `protected_or_encrypted_files_not_supported` | 422 | Archivo protegido o cifrado no soportado. |
| `file_exceeds_size_limit` | 413 | Archivo mayor al limite aplicable. |
| `cumulative_storage_quota_exceeded` | 413 | Cuota acumulada agotada. |
| `too_many_active_jobs_for_this_user` | 429 | Limite de jobs activos para usuario autenticado. |
| `too_many_active_jobs_for_this_guest_session` | 429 | Limite de jobs activos para sesion anonima. |
| `job_intake_is_temporarily_paused_by_admin` | 503 | Intake pausado operativamente. |
| `server_storage_is_full` | 507 | Storage sin headroom suficiente. |

## Reglas para nuevos endpoints

- Usar `respondError` en handlers HTTP.
- Elegir mensajes cortos, estables y orientados a cliente.
- Cubrir cambios de contrato con tests.
- No exponer errores crudos de infraestructura al usuario.
- Preferir status HTTP especificos cuando el cliente pueda actuar distinto: `401`, `403`, `404`, `409`, `410`, `413`, `422`, `429`, `503`, `507`.
