CREATE TABLE IF NOT EXISTS email_templates (
    template_key  TEXT PRIMARY KEY,
    subject       TEXT NOT NULL,
    body_html     TEXT NOT NULL,
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO email_templates (template_key, subject, body_html, updated_at)
VALUES (
    'welcome',
    'Bienvenido a {{.AppName}}',
    '<!DOCTYPE html>
<html lang="es">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background-color:#f5f5f4;font-family:system-ui,-apple-system,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f5f5f4;padding:40px 0;">
    <tr><td align="center">
      <table role="presentation" width="600" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
        <tr><td style="background-color:#f97066;padding:32px 40px;">
          <h1 style="margin:0;color:#ffffff;font-size:24px;font-weight:600;">{{.AppName}}</h1>
        </td></tr>
        <tr><td style="padding:40px;">
          <h2 style="margin:0 0 16px;color:#1c1917;font-size:20px;font-weight:600;">Hola {{.Name}},</h2>
          <p style="margin:0 0 16px;color:#44403c;font-size:16px;line-height:1.6;">
            Tu cuenta ha sido creada exitosamente. Ya puedes subir archivos, detectar formatos y convertir con seguridad.
          </p>
          <p style="margin:0;color:#78716c;font-size:14px;line-height:1.5;">
            Si no creaste esta cuenta, puedes ignorar este mensaje.
          </p>
        </td></tr>
        <tr><td style="padding:24px 40px;background-color:#fafaf9;border-top:1px solid #e7e5e4;">
          <p style="margin:0;color:#a8a29e;font-size:12px;text-align:center;">
            &copy; {{.Year}} {{.AppName}}
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>',
    datetime('now')
);
