# Authentication

## Password

Admin passwords are stored as bcrypt hashes in `secrets.yaml`. Generate a hash:

```bash
docker run --rm -it alpine sh -c \
  "apk add --no-cache apache2-utils && htpasswd -nbBC 10 '' 'YourPassword' | cut -d: -f2"
```

## TOTP 2FA

1. Go to **Admin → Settings → Security**
2. Click **Enable 2FA**
3. Scan the QR code with Google Authenticator / Authy / 1Password
4. Enter the 6-digit code to verify

TOTP codes are single-use to prevent replay attacks.

## WebAuthn Passkeys

Register biometric or security key authentication:

1. Go to **Admin → Settings → Security → Passkeys**
2. Click **+ Add Passkey**
3. Name it (e.g., "MacBook Touch ID")
4. Authenticate with your device

Passkeys can be used instead of password + TOTP for login.

## IP Lockout

Repeated failed login attempts result in a temporary IP lockout. All lockout events are recorded in the audit log.

## Session Management

- Sessions use signed, secure cookies
- Changing your password invalidates all existing sessions

## Audit Trail

All authentication events (login, logout, lockouts) are recorded in the persistent audit log. View it in **Admin → Settings → Security → Audit Log**.
