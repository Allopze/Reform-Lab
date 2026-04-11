INSERT OR IGNORE INTO email_templates (template_key, subject, body_html, updated_at)
VALUES (
    'conversion-complete',
    'Tu archivo esta listo — {{.AppName}}',
    '<!DOCTYPE html>
<html lang="es">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#f5f5f4;font-family:system-ui,-apple-system,sans-serif;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f5f5f4;">
    <tr><td align="center" style="padding:40px 16px;">
      <table role="presentation" width="560" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:12px;overflow:hidden;border:1px solid #e7e5e4;">
        <tr><td style="background-color:#1c1917;padding:28px 32px;">
          <span style="font-size:20px;font-weight:700;color:#ffffff;letter-spacing:-0.3px;">{{.AppName}}</span>
        </td></tr>
        <tr><td style="padding:32px;">
          <h1 style="margin:0 0 8px;font-size:22px;font-weight:700;color:#1c1917;">Conversion completada</h1>
          <p style="margin:0 0 20px;font-size:15px;color:#57534e;line-height:1.6;">
            Hola {{.Name}}, tu archivo <strong>{{.FileName}}</strong> fue convertido a <strong>{{.OutputFormat}}</strong> exitosamente.
          </p>
          <p style="margin:0 0 20px;font-size:15px;color:#57534e;line-height:1.6;">
            Puedes descargarlo desde tu panel de usuario.
          </p>
          <table role="presentation" cellspacing="0" cellpadding="0">
            <tr><td style="background-color:#f23c2f;border-radius:8px;">
              <a href="{{.AppURL}}" target="_blank" style="display:inline-block;padding:12px 24px;font-size:14px;font-weight:600;color:#ffffff;text-decoration:none;">Ir a Reform Lab</a>
            </td></tr>
          </table>
        </td></tr>
        <tr><td style="padding:20px 32px;border-top:1px solid #e7e5e4;">
          <p style="margin:0;font-size:12px;color:#a8a29e;">&copy; {{.Year}} {{.AppName}}. Todos los derechos reservados.</p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>',
    datetime('now')
);

INSERT OR IGNORE INTO email_templates (template_key, subject, body_html, updated_at)
VALUES (
    'conversion-failed',
    'Hubo un problema con tu archivo — {{.AppName}}',
    '<!DOCTYPE html>
<html lang="es">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#f5f5f4;font-family:system-ui,-apple-system,sans-serif;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f5f5f4;">
    <tr><td align="center" style="padding:40px 16px;">
      <table role="presentation" width="560" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:12px;overflow:hidden;border:1px solid #e7e5e4;">
        <tr><td style="background-color:#1c1917;padding:28px 32px;">
          <span style="font-size:20px;font-weight:700;color:#ffffff;letter-spacing:-0.3px;">{{.AppName}}</span>
        </td></tr>
        <tr><td style="padding:32px;">
          <h1 style="margin:0 0 8px;font-size:22px;font-weight:700;color:#1c1917;">Conversion fallida</h1>
          <p style="margin:0 0 20px;font-size:15px;color:#57534e;line-height:1.6;">
            Hola {{.Name}}, la conversion de tu archivo <strong>{{.FileName}}</strong> a <strong>{{.OutputFormat}}</strong> no pudo completarse.
          </p>
          <p style="margin:0 0 8px;font-size:13px;font-weight:600;color:#78716c;">Detalle del error:</p>
          <div style="background-color:#fef2f2;border:1px solid #fecaca;border-radius:8px;padding:12px 16px;margin:0 0 20px;">
            <p style="margin:0;font-size:13px;color:#991b1b;font-family:monospace;">{{.ErrorMessage}}</p>
          </div>
          <p style="margin:0 0 20px;font-size:15px;color:#57534e;line-height:1.6;">
            Puedes intentarlo de nuevo desde tu panel de usuario.
          </p>
          <table role="presentation" cellspacing="0" cellpadding="0">
            <tr><td style="background-color:#f23c2f;border-radius:8px;">
              <a href="{{.AppURL}}" target="_blank" style="display:inline-block;padding:12px 24px;font-size:14px;font-weight:600;color:#ffffff;text-decoration:none;">Ir a Reform Lab</a>
            </td></tr>
          </table>
        </td></tr>
        <tr><td style="padding:20px 32px;border-top:1px solid #e7e5e4;">
          <p style="margin:0;font-size:12px;color:#a8a29e;">&copy; {{.Year}} {{.AppName}}. Todos los derechos reservados.</p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>',
    datetime('now')
);

INSERT OR IGNORE INTO email_templates (template_key, subject, body_html, updated_at)
VALUES (
    'password-reset',
    'Restablecer tu contrasena — {{.AppName}}',
    '<!DOCTYPE html>
<html lang="es">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#f5f5f4;font-family:system-ui,-apple-system,sans-serif;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f5f5f4;">
    <tr><td align="center" style="padding:40px 16px;">
      <table role="presentation" width="560" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:12px;overflow:hidden;border:1px solid #e7e5e4;">
        <tr><td style="background-color:#1c1917;padding:28px 32px;">
          <span style="font-size:20px;font-weight:700;color:#ffffff;letter-spacing:-0.3px;">{{.AppName}}</span>
        </td></tr>
        <tr><td style="padding:32px;">
          <h1 style="margin:0 0 8px;font-size:22px;font-weight:700;color:#1c1917;">Restablecer contrasena</h1>
          <p style="margin:0 0 20px;font-size:15px;color:#57534e;line-height:1.6;">
            Hola {{.Name}}, recibimos una solicitud para restablecer la contrasena de tu cuenta en {{.AppName}}.
          </p>
          <p style="margin:0 0 20px;font-size:15px;color:#57534e;line-height:1.6;">
            Si no realizaste esta solicitud, ignora este correo. Tu contrasena no cambiara.
          </p>
          <table role="presentation" cellspacing="0" cellpadding="0">
            <tr><td style="background-color:#f23c2f;border-radius:8px;">
              <a href="{{.ResetURL}}" target="_blank" style="display:inline-block;padding:12px 24px;font-size:14px;font-weight:600;color:#ffffff;text-decoration:none;">Restablecer contrasena</a>
            </td></tr>
          </table>
          <p style="margin:20px 0 0;font-size:13px;color:#a8a29e;line-height:1.5;">
            Este enlace expira en 1 hora. Si tienes problemas con el boton, copia y pega esta URL en tu navegador:
          </p>
          <p style="margin:4px 0 0;font-size:13px;color:#78716c;word-break:break-all;">{{.ResetURL}}</p>
        </td></tr>
        <tr><td style="padding:20px 32px;border-top:1px solid #e7e5e4;">
          <p style="margin:0;font-size:12px;color:#a8a29e;">&copy; {{.Year}} {{.AppName}}. Todos los derechos reservados.</p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>',
    datetime('now')
);
