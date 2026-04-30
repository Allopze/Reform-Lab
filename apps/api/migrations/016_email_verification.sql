-- 016_email_verification.sql
-- Optional email verification for registered accounts.

ALTER TABLE users ADD COLUMN email_verified_at TEXT;

CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    used_at    TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_expires_at ON email_verification_tokens(expires_at);

INSERT OR IGNORE INTO email_templates (template_key, subject, body_html, updated_at)
VALUES (
    'email-verification',
    'Verifica tu correo — {{.AppName}}',
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
          <h1 style="margin:0 0 8px;font-size:22px;font-weight:700;color:#1c1917;">Verifica tu correo</h1>
          <p style="margin:0 0 20px;font-size:15px;color:#57534e;line-height:1.6;">
            Hola {{.Name}}, confirma que este correo pertenece a tu cuenta en {{.AppName}}.
          </p>
          <table role="presentation" cellspacing="0" cellpadding="0">
            <tr><td style="background-color:#f23c2f;border-radius:8px;">
              <a href="{{.VerifyURL}}" target="_blank" style="display:inline-block;padding:12px 24px;font-size:14px;font-weight:600;color:#ffffff;text-decoration:none;">Verificar correo</a>
            </td></tr>
          </table>
          <p style="margin:20px 0 0;font-size:13px;color:#a8a29e;line-height:1.5;">
            Este enlace expira en 24 horas. Si no creaste esta cuenta, ignora este correo.
          </p>
          <p style="margin:4px 0 0;font-size:13px;color:#78716c;word-break:break-all;">{{.VerifyURL}}</p>
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
