# Google

English | [简体中文](README-zh.md)

GoNoteLM signs users in with Google over OpenID Connect. The backend exchanges the authorization code, verifies the `id_token` signature, issuer, audience, and nonce, then reads the profile from the UserInfo endpoint.

## 1. Configure the project

1. Open the Google Auth Platform for your project:

   ```text
   https://console.cloud.google.com/auth/clients?project=YOUR_PROJECT_ID
   ```

   Replace `YOUR_PROJECT_ID` with the project ID. Create or select a project first if you do not have one.

2. Configure the consent screen under **Branding** (app name, support email) and **Audience**:
   - **External** for any Google account, **Internal** for a single Workspace organization.
   - While the app is in **Testing**, only accounts listed as test users can sign in — add your own.
   - Publish the app before opening sign-in to the public.

3. The default scopes are `openid`, `email`, and `profile`. They are non-sensitive, so no scope verification is required.

## 2. Create an OAuth client

1. Open **Clients** and click **Create client**.
2. Choose **Application type → Web application**.
3. Under **Authorized redirect URIs**, add the callback — this is the value of `GONOTELM_OAUTH_GOOGLE_REDIRECT_URI`:

   ```text
   http://127.0.0.1:7099/api/v1/auth/callback/google
   ```

4. Click **Create** and copy the client ID and client secret.

## 3. Configure the backend

Add these to `.env`:

```env
GONOTELM_OAUTH_GOOGLE_CLIENT_ID=...
GONOTELM_OAUTH_GOOGLE_CLIENT_SECRET=...
GONOTELM_OAUTH_GOOGLE_REDIRECT_URI=http://127.0.0.1:7099/api/v1/auth/callback/google
```

- `GONOTELM_OAUTH_GOOGLE_REDIRECT_URI` is optional. The default in `etc/notelm.toml.tpl` already matches the URL above, so only set it when the callback differs.
- The client ID and client secret must both be set. Otherwise the provider is skipped at startup (`skip login provider without credentials` in the log) and absent from `/api/v1/auth/providers`.
- `authEndpoint` / `tokenEndpoint` / discovery URLs are built in; leave the `[idp.google]` overrides empty unless you are testing against a proxy.
- Restart notelm after editing `.env`; `make run-notelm` loads the file automatically.

## 4. Verify

```bash
curl -s http://127.0.0.1:7099/api/v1/auth/providers
# {"code":0,"msg":"ok","data":{"providers":[{"name":"google"}],"password_enabled":true}}
```

Then open:

```text
http://127.0.0.1:7099/api/v1/auth/login?login_provider=google&login_from=web
```

After consent, Google redirects to `/api/v1/auth/callback/google`, which sets the `gnlm_user_sid` session cookie and sends the browser to `return_to` (or `/`). The web UI uses the same endpoint through its `/api` proxy.

## 5. Production

1. Add the production callback to the same OAuth client, or create a separate client per environment:

   ```text
   https://YOUR_DOMAIN/api/v1/auth/callback/google
   ```

2. Point the backend at it:

   ```env
   GONOTELM_OAUTH_GOOGLE_REDIRECT_URI=https://YOUR_DOMAIN/api/v1/auth/callback/google
   ```

   Google only accepts `http` for localhost/`127.0.0.1`; production callbacks must be `https`.

3. Allow the web origin for `return_to`:

   ```env
   GONOTELM_AUTH_RETURN_TO_ALLOW_ORIGINS=https://YOUR_DOMAIN
   ```

4. Publish the consent screen and serve the API over HTTPS. The session cookie is `Secure`, `HttpOnly`, and `SameSite=Lax`, which is what lets it survive Google's top-level redirect back.

## Troubleshooting

### `Error 400: redirect_uri_mismatch`

The redirect URI sent by the backend is not registered on the OAuth client. Compare scheme, host, port, and path; `127.0.0.1` and `localhost` are different hosts.

### `Access blocked: this app has not completed verification`

The app is in Testing and the account is not a test user, or the app has not been published. Add the account under **Audience → Test users**, or publish the app.

### The Google button never appears

`/api/v1/auth/providers` omits `google` when the client ID or secret is empty. Confirm notelm loaded `.env` and look for `skip login provider without credentials`.

### `oauth state not found` or `oauth state mismatch`

The `_gnlm_oauth_state` cookie is one-time and expires after 10 minutes. Restart the flow from `/api/v1/auth/login` in the same browser profile instead of reloading the callback URL.
