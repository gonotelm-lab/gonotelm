# Google

[English](README.md) | 简体中文

GoNoteLM 通过 OpenID Connect 接入 Google 登录。后端用 code 换取 token，校验 `id_token` 的签名、issuer、audience 和 nonce，再从 UserInfo 接口读取用户资料。

## 1. 配置项目

1. 打开项目的 Google Auth Platform：

   ```text
   https://console.cloud.google.com/auth/clients?project=YOUR_PROJECT_ID
   ```

   `YOUR_PROJECT_ID` 替换为实际的项目 ID。如果还没有项目，先创建一个，或者切换到已有项目。

2. 在 **Branding** 中填写应用名和支持邮箱，在 **Audience** 中配置同意屏幕：

   - 允许任意 Google 账号登录，选 **External**；仅限一个 Workspace 组织，选 **Internal**。
   - 应用处于 **Testing** 状态时，只有测试用户能登录。把自己的账号加进去。
   - 对外开放前记得发布。

3. 默认 scope 为 `openid`、`email`、`profile`，都属于 non-sensitive scope，无需提交验证。

## 2. 创建 OAuth client

1. 打开 **Clients**，点 **Create client**。
2. **Application type** 选 **Web application**。
3. 在 **Authorized redirect URIs** 中加入 callback，也就是 `GONOTELM_OAUTH_GOOGLE_REDIRECT_URI` 的值：

   ```text
   http://127.0.0.1:7099/api/v1/auth/callback/google
   ```

4. 点 **Create**，复制 client ID 和 client secret。

## 3. 配置后端

`.env` 里加上：

```env
GONOTELM_OAUTH_GOOGLE_CLIENT_ID=...
GONOTELM_OAUTH_GOOGLE_CLIENT_SECRET=...
GONOTELM_OAUTH_GOOGLE_REDIRECT_URI=http://127.0.0.1:7099/api/v1/auth/callback/google
```

- `GONOTELM_OAUTH_GOOGLE_REDIRECT_URI` 可以留空。`etc/notelm.toml.tpl` 里的默认值就是上面的本地地址，只有 callback 不同时才需要设置。
- client ID 和 client secret 缺一不可。少任意一个，这个 provider 在启动时就会被跳过（日志会打印 `skip login provider without credentials`），`/api/v1/auth/providers` 也不会返回 `google`。
- `authEndpoint` / `tokenEndpoint` / discovery 地址都已内置，`[idp.google]` 中这几项保持为空；只有对着代理做测试时才需要改。
- 修改 `.env` 后需要重启 notelm。`make run-notelm` 会自动读取该文件。

## 4. 验证

```bash
curl -s http://127.0.0.1:7099/api/v1/auth/providers
# {"code":0,"msg":"ok","data":{"providers":[{"name":"google"}],"password_enabled":true}}
```

然后打开：

```text
http://127.0.0.1:7099/api/v1/auth/login?login_provider=google&login_from=web
```

授权完成后，Google 会跳回 `/api/v1/auth/callback/google`，callback 写入 `gnlm_user_sid` session cookie，再重定向到 `return_to`；没有传就回到 `/`。Web UI 走同一个接口（经 `/api` 代理）。

## 5. 生产环境

1. 把线上 callback 加到同一个 OAuth client，或者按环境各建一个：

   ```text
   https://YOUR_DOMAIN/api/v1/auth/callback/google
   ```

2. 让后端指向它：

   ```env
   GONOTELM_OAUTH_GOOGLE_REDIRECT_URI=https://YOUR_DOMAIN/api/v1/auth/callback/google
   ```

   Google 只允许 localhost/`127.0.0.1` 使用 `http`，线上 callback 必须使用 `https`。

3. 把 Web 的 origin 加入 `return_to` 白名单：

   ```env
   GONOTELM_AUTH_RETURN_TO_ALLOW_ORIGINS=https://YOUR_DOMAIN
   ```

4. 发布同意屏幕，并让 API 通过 HTTPS 提供服务。session cookie 带有 `Secure`、`HttpOnly`、`SameSite=Lax`；Google 跳回 callback 属于顶层跳转，只有 `Lax` 才能让浏览器带上这个 cookie。

## 常见问题

### `Error 400: redirect_uri_mismatch`

后端发出的 redirect URI 没有登记在 OAuth client 上。请逐项核对 scheme、host、port 和 path；`127.0.0.1` 与 `localhost` 是两个不同的 host。

### `Access blocked: this app has not completed verification`

应用还在 Testing，账号不在测试用户里；或者应用尚未发布。去 **Audience → Test users** 添加账号，或者直接发布应用。

### 界面没有显示 Google 按钮

client ID 或 secret 为空时，`/api/v1/auth/providers` 不会返回 `google`。确认 notelm 读取到了 `.env`，再检查日志中是否有 `skip login provider without credentials`。

### `oauth state not found` 或 `oauth state mismatch`

`_gnlm_oauth_state` cookie 只能使用一次，10 分钟后失效。请从 `/api/v1/auth/login` 重新发起流程，不要直接刷新 callback 地址。
