# API Documentation

Base URL: `http://localhost:8080` by default.

All application endpoints are mounted under `/api/v1`, except `/health`.

## Common behavior

- Authenticated routes expect `Authorization: Bearer <token>`.
- Errors return JSON with at least an `error` field.
- JSON request bodies are capped at 1 MiB.
- Security headers are applied globally.

Role legend:

- Public: no auth required
- Staff: employee or admin
- Admin: admin only

## Health

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/health` | Public | Liveness probe |

## Authentication

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| POST | `/api/v1/auth/login` | Public | Login and receive employee session token |
| POST | `/api/v1/auth/logout` | Public | Logout and revoke current token |
| GET | `/api/v1/auth/session` | Public | Return current session if token is valid |
| POST | `/api/v1/auth/lockout/reset` | Admin | Reset login lockout for an employee identity |

## Public catalog

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/api/v1/products` | Public | List products |
| GET | `/api/v1/products/{id}` | Public | Get product by ID |
| GET | `/api/v1/categories` | Public | List categories |
| GET | `/api/v1/categories/{slug}` | Public | Get category by slug |
| GET | `/api/v1/flavors` | Public | List flavors |
| GET | `/api/v1/flavors/{id}` | Public | Get flavor by ID |
| GET | `/api/v1/addons` | Public | List addons |
| GET | `/api/v1/addons/{id}` | Public | Get addon by ID |

## Orders

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| POST | `/api/v1/orders` | Public | Create order; optionally captures authenticated staff identity when present |
| GET | `/api/v1/orders/track` | Public | Secure public tracking by `order_number` and `tracking_token` |
| GET | `/api/v1/orders` | Staff | List orders |
| GET | `/api/v1/orders/{id}` | Staff | Get order by ID |
| PATCH | `/api/v1/orders/{id}` | Staff | Update order details and items |
| POST | `/api/v1/orders/{id}/approve` | Staff | Approve order |
| POST | `/api/v1/orders/{id}/reject` | Staff | Reject order |
| PATCH | `/api/v1/orders/{id}/status` | Staff | Advance or set order status |
| PATCH | `/api/v1/orders/{id}/notes` | Staff | Update internal notes |
| DELETE | `/api/v1/orders/{id}` | Staff | Delete order |

## Settings

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/api/v1/settings/store/public` | Public | Public store state and active offers |
| GET | `/api/v1/settings/tabs` | Staff | Current admin tab order |
| PATCH | `/api/v1/settings/tabs` | Admin | Update admin tab order |
| GET | `/api/v1/settings/panel-config` | Staff | Get panel config |
| PATCH | `/api/v1/settings/panel-config` | Admin | Update panel config |
| GET | `/api/v1/settings/store` | Staff | Get store settings |
| PATCH | `/api/v1/settings/store` | Admin | Update store settings |

## Employees

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/api/v1/employees` | Admin | List employees |
| POST | `/api/v1/employees` | Admin | Create employee |
| PATCH | `/api/v1/employees/{id}` | Admin | Update employee |
| PATCH | `/api/v1/employees/{id}/deactivate` | Admin | Deactivate employee |
| DELETE | `/api/v1/employees/{id}` | Admin | Delete employee |

## Customer directory

These routes are exposed as `users`, but they are backed by the `Customer` Ent entity.

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/api/v1/users` | Staff | List customers |
| GET | `/api/v1/users/{id}` | Staff | Get customer by ID |
| GET | `/api/v1/users/{id}/orders` | Admin | Get orders associated with a customer |
| POST | `/api/v1/users` | Admin | Create customer record |
| PATCH | `/api/v1/users/{id}` | Admin | Update customer record |
| DELETE | `/api/v1/users/{id}` | Admin | Delete customer record |

## Catalog management

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| POST | `/api/v1/categories` | Staff | Create category |
| PATCH | `/api/v1/categories/{id}` | Staff | Update category by ID |
| DELETE | `/api/v1/categories/{id}` | Staff | Delete category by ID |
| POST | `/api/v1/products` | Staff | Create product |
| PATCH | `/api/v1/products/{id}` | Staff | Update product |
| PATCH | `/api/v1/products/{id}/availability` | Staff | Toggle product availability |
| DELETE | `/api/v1/products/{id}` | Staff | Delete product |
| POST | `/api/v1/products/{id}/flavors/{flavorId}` | Staff | Link flavor to product |
| DELETE | `/api/v1/products/{id}/flavors/{flavorId}` | Staff | Unlink flavor from product |
| POST | `/api/v1/products/{id}/addons/{addonId}` | Staff | Link addon to product |
| DELETE | `/api/v1/products/{id}/addons/{addonId}` | Staff | Unlink addon from product |
| POST | `/api/v1/flavors` | Staff | Create flavor |
| PATCH | `/api/v1/flavors/{id}` | Staff | Update flavor |
| DELETE | `/api/v1/flavors/{id}` | Staff | Delete flavor |
| POST | `/api/v1/addons` | Staff | Create addon |
| PATCH | `/api/v1/addons/{id}` | Staff | Update addon |
| DELETE | `/api/v1/addons/{id}` | Staff | Delete addon |

## Analytics and export

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/api/v1/analytics/overview` | Admin | KPI summary |
| GET | `/api/v1/analytics/orders-over-time` | Admin | Orders timeline |
| GET | `/api/v1/analytics/top-products` | Admin | Top product breakdown |
| GET | `/api/v1/export/orders` | Admin | Export orders |
| DELETE | `/api/v1/export/orders` | Admin | Purge old orders |

## Uploads

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| POST | `/api/v1/upload/image` | Staff | Upload image |
| DELETE | `/api/v1/upload/image` | Admin | Delete image |
| GET | `/api/v1/upload/images` | Staff | List images |

## Notes for frontend consumers

- Public tracking is `GET /api/v1/orders/track`, not `/orders/{id}`.
- Public store settings are `GET /api/v1/settings/store/public`.
- Category reads use slug, but category updates and deletes use ID.

---

## 3) Orders

| Operation                                    | Method | Path                                                           |
| -------------------------------------------- | ------ | -------------------------------------------------------------- |
| Create order (public, rate-limited)          | POST   | `/api/v1/orders`                                               |
| Track order by number + token (public)       | GET    | `/api/v1/orders/track?order_number=...&tracking_token=...`    |
| List all (employee/admin, paginated)         | GET    | `/api/v1/orders`                                               |
| Filter by status                             | GET    | `/api/v1/orders?status=pendiente_revision`                     |
| Get one (employee/admin)                     | GET    | `/api/v1/orders/{id}`                                          |
| Update order details/items (employee/admin)  | PATCH  | `/api/v1/orders/{id}`                                          |
| Approve (employee/admin)                     | POST   | `/api/v1/orders/{id}/approve`                                  |
| Reject (employee/admin)                      | POST   | `/api/v1/orders/{id}/reject`                                   |
| Update status (employee/admin)               | PATCH  | `/api/v1/orders/{id}/status`                                   |
| Update notes only (employee/admin)           | PATCH  | `/api/v1/orders/{id}/notes`                                    |
| Delete order (admin only)                    | DELETE | `/api/v1/orders/{id}`                                          |

> `GET /api/v1/orders` supports `status`, `page`, and `limit` query params.

### POST `/api/v1/orders` — Create (public, no auth)

Rate limited per IP (default: 5/hour) and per phone (default: 20/day).

Body:

```json
{
  "customer_name": "María García",
  "customer_phone": "+50688887777",
  "customer_email": "maria@example.com",
  "payment_method": "efectivo",
  "notes": "Sin nueces por favor",
  "items": [
    {
      "product_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      "quantity": 2,
      "customizations": { "topping": "sprinkles", "size": "large" }
    },
    {
      "product_id": "b23dc10b-48aa-1234-a567-0e02b2c3d480",
      "quantity": 1
    }
  ]
}
```

Fields:

| Field                     | Type   | Required | Notes                                                         |
| ------------------------- | ------ | -------- | ------------------------------------------------------------- |
| `customer_name`           | string | yes      | Max 200                                                       |
| `customer_phone`          | string | yes      | Max 20                                                        |
| `customer_email`          | string | no       |                                                               |
| `payment_method`          | string | yes      | `"efectivo"` \| `"tarjeta"` \| `"transferencia"` \| `"otro"` |
| `order_type`              | string | no       | `"para_llevar"` (default) \| `"en_local"`                  |
| `table_number`            | int    | no       | Required when `order_type="en_local"`                        |
| `notes`                   | string | no       |                                                               |
| `items`                   | array  | yes      | At least 1 item                                               |
| `items[].product_id`      | UUID   | yes      | Must exist and be available                                   |
| `items[].quantity`         | int    | yes      | Positive integer                                              |
| `items[].customizations`  | object | no       | Freeform JSON for toppings, flavours, etc.                    |

The backend calculates `total_amount` server-side from current product prices × quantities.

Response `201 Created`:

```json
{
  "order_number": "ORD-20250215-4821",
  "status": "pendiente_revision",
  "total": 10.50,
  "tracking_token": "<token>",
  "tracking_expires_at": "2026-05-10T14:30:00Z",
  "message": "Orden recibida, te notificaremos pronto 🍦"
}
```

A WhatsApp notification is automatically sent to the customer.

---

## 4) Analytics

All analytics routes are **admin-only**.

| Operation                    | Method | Path                                  |
| --------------------------- | ------ | ------------------------------------- |
| Overview KPIs              | GET    | `/api/v1/analytics/overview`          |
| Orders over time series    | GET    | `/api/v1/analytics/orders-over-time`  |
| Top-selling products       | GET    | `/api/v1/analytics/top-products`      |

### GET `/api/v1/orders/track` — Public tracking

Query params:

- `order_number` (required)
- `tracking_token` (required)

Response `200 OK`:

```json
{
  "order_number": "ORD-20250215-4821",
  "status": "en_proceso",
  "updated_at": "2025-02-15T15:10:00Z",
  "created_at": "2025-02-15T14:30:00Z",
  "total_amount": 10.50,
  "order_type": "para_llevar",
  "items": [
    {
      "product_name": "Gelato de Pistacho",
      "quantity": 2,
      "unit_price": 4.75,
      "subtotal": 9.50,
      "customizations": {
        "flavor_name": "Pistacho",
        "extra_addon_names": ["Chispas de chocolate"]
      }
    }
  ]
}
```

### PATCH `/api/v1/orders/{id}` — Update order details

Allows staff/admin to update customer details, payment/order type, notes, and (optionally) the full items list.

```json
{
  "customer_name": "María García",
  "customer_phone": "+50688887777",
  "customer_email": "maria@example.com",
  "payment_method": "tarjeta",
  "order_type": "en_local",
  "table_number": 4,
  "notes": "Sin nueces por favor",
  "items": [
    {
      "product_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      "quantity": 2,
      "customizations": { "topping": "sprinkles" }
    }
  ]
}
```

Response `200 OK`: returns the updated order with nested items.

### POST `/api/v1/orders/{id}/approve`

No body needed. Transitions: `pendiente_revision` → `recibida`, sets `payment_status` to `confirmado`.

Response `200 OK`: returns the full updated order object.

### POST `/api/v1/orders/{id}/reject`

Body:

```json
{
  "reason": "Producto no disponible en este momento"
}
```

`reason` is **required**. Sets status to `cancelada`.

Response `200 OK`: returns the updated order object.

### PATCH `/api/v1/orders/{id}/status` — Update status

Body:

```json
{
  "status": "en_proceso"
}
```

Allowed values: `"en_proceso"`, `"lista"`, `"entregada"`.

Full status flow:

```
pendiente_revision → recibida → en_proceso → lista → entregada
                   ↘ cancelada (via reject)
```

When status is set to `"lista"`, a WhatsApp notification is sent to the customer.

Response `200 OK`: returns the updated order.

### PATCH `/api/v1/orders/{id}/notes` — Update notes only

Body:

```json
{
  "notes": "Cliente llegará en 15 minutos"
}
```

To clear notes, send an empty string.

Response `200 OK`: returns the updated order.

### DELETE `/api/v1/orders/{id}` — Delete (admin)

Response `200 OK`:

```json
{
  "message": "Order deleted"
}
```

---

## 4) Order Items

Order items are created **automatically** when an order is created (via the `items` array in `POST /api/v1/orders`). There are **no separate CRUD endpoints** for order items.

When you `GET /api/v1/orders/{id}`, items come nested in the response:

```json
{
  "id": "...",
  "order_number": "ORD-20250215-4821",
  "status": "recibida",
  "total_amount": 10.50,
  "items": [
    {
      "id": "...",
      "product_name": "Gelato de Chocolate",
      "quantity": 2,
      "unit_price": 3.50,
      "subtotal": 7.00,
      "customizations": { "topping": "sprinkles" }
    },
    {
      "id": "...",
      "product_name": "Sorbete de Mango",
      "quantity": 1,
      "unit_price": 3.50,
      "subtotal": 3.50,
      "customizations": null
    }
  ]
}
```

Each order item stores a snapshot of the product name and price at the time of the order.

---

## 5) Rate Limits

Rate limits are **not exposed as a REST resource**. They are enforced automatically
as Chi middleware on `POST /api/v1/orders`:

- **Per IP:** max `ORDERS_PER_HOUR` (default: 5) orders per hour
- **Per phone:** max `ORDERS_PER_DAY` (default: 20) orders per day

When rate-limited, the response is:

```json
{
  "error": "too many orders from your IP, try later"
}
```

Status code: `429 Too Many Requests`.

Configure limits via environment variables `ORDERS_PER_HOUR` and `ORDERS_PER_DAY`.

---

## 6) Auth

Authentication is handled by **PocketBase** running as an embedded Go library. The Chi API exposes all app auth endpoints. The PocketBase Admin UI server can be enabled separately for development (`ENABLE_PB_ADMIN_UI=true`).

### POST `/api/v1/auth/login` — Login

**POST** `/api/v1/auth/login`

Body:

```json
{
  "email": "admin@ishos.com",
  "password": "securePassword123"
}
```

The Chi handler validates credentials via PocketBase's Go API (password hashing, record lookup) and returns a PocketBase-compatible JWT token.

Response `200 OK`:

```json
{
  "token": "<your-jwt-token>",
  "employee": {
    "id": "pb-record-id",
    "email": "admin@ishos.com",
    "name": "Admin",
    "phone": "+50688881234",
    "role": "admin"
  }
}
```

The frontend stores the returned `token` and sends it as `Authorization: Bearer <token>` on all protected requests.

### GET `/api/v1/auth/session` — Current user

Requires: `Authorization: Bearer <token>`

Returns the authenticated employee's data from the request context (set by the auth middleware).

Response `200 OK`:

```json
{
  "id": "pb-record-id",
  "email": "admin@ishos.com",
  "name": "Admin",
  "phone": "+50688881234",
  "role": "admin",
  "active": true
}
```

### POST `/api/v1/auth/logout` — Logout

With token-based auth, the server has no session to destroy. This endpoint returns a confirmation — the frontend should discard the stored token.


### POST `/api/v1/auth/lockout/reset` — Admin unlock

Requires: `Authorization: Bearer <token>` with `admin` role.

Use this endpoint to immediately clear login lockout waits for one employee.

Body (target by PocketBase employee ID):

```json
{
  "employee_id": "pb-record-id"
}
```

Body (target by employee email):

```json
{
  "email": "employee@ishos.com"
}
```

Response `200 OK`:

```json
{
  "message": "Login lockout reset successfully",
  "scope": "employee",
  "employee_id": "pb-record-id",
  "email": "employee@ishos.com"
}
```
Response `200 OK`:

```json
{
  "message": "Logged out successfully. Discard the token on the client."
}
```

### Token lifecycle

Tokens are PocketBase JWTs with configurable expiration via `AUTH_TOKEN_TTL_HOURS` (default: 168h / 7 days). Browser cookie persistence is configured separately via `AUTH_COOKIE_TTL_HOURS` (default: 24h). When a token expires, the user must log in again via `POST /api/v1/auth/login`.

---

## 7) Employees (PocketBase auth collection)

Employees are stored in the PocketBase `employees` auth collection (embedded SQLite). PocketBase handles password hashing, token generation, and account management. All CRUD goes through Chi handlers that call PocketBase's Go API.

| Operation                     | Method | Path                               |
| ---------------------------- | ------ | ---------------------------------- |
| List employees               | GET    | `/api/v1/employees`                |
| Create employee              | POST   | `/api/v1/employees`                |
| Update employee              | PATCH  | `/api/v1/employees/{id}`           |
| Deactivate employee          | PATCH  | `/api/v1/employees/{id}/deactivate`|
| Permanently delete employee  | DELETE | `/api/v1/employees/{id}`           |

### POST `/api/v1/employees` — Create

Body:

```json
{
  "email": "maria@ishos.com",
  "password": "securePassword123",
  "name": "María López",
  "phone": "+50688881234",
  "role": "employee"
}
```

Fields:

| Field      | Type   | Required | Notes                      |
| ---------- | ------ | -------- | -------------------------- |
| `email`    | string | yes      |                            |
| `password` | string | yes      | PocketBase handles hashing |
| `name`     | string | no       |                            |
| `phone`    | string | no       |                            |
| `role`     | string | yes      | `"employee"` \| `"admin"` |
| `state`    | string | no       | `"active"` (default) \| `"inactive"` |

Response `201 Created`:

```json
{
  "id": "pb-record-id",
  "email": "maria@ishos.com",
  "name": "María López",
  "phone": "+50688881234",
  "role": "employee",
  "message": "Employee created. They can log in at the admin panel."
}
```

### PATCH `/api/v1/employees/{id}` — Update employee

Body (all fields optional):

```json
{
  "name": "María López",
  "phone": "+50688881234",
  "role": "employee",
  "state": "active"
}
```

Response `200 OK`: returns the updated employee.

### PATCH `/api/v1/employees/{id}/deactivate` — Deactivate

Sets the employee record's `disabled` flag to `true`. The employee can no longer log in because the auth middleware checks the disabled flag on every request. The record is preserved.

Response `200 OK`:

```json
{
  "message": "Employee deactivated. They can no longer log in."
}
```

### DELETE `/api/v1/employees/{id}` — Permanent delete

Removes the employee record from PocketBase.

Response `200 OK`:

```json
{
  "message": "Employee deleted permanently."
}
```

---

## 8) Image upload (Cloudflare R2)

| Operation      | Method | Path                    |
| -------------- | ------ | ----------------------- |
| Upload image   | POST   | `/api/v1/upload/image`  |
| Delete image  | DELETE | `/api/v1/upload/image`  |
| List images   | GET    | `/api/v1/upload/images` |

### POST `/api/v1/upload/image`

**Content-Type: `multipart/form-data`** (not JSON!)

Form fields:

| Field    | Type   | Required | Notes                          |
| -------- | ------ | -------- | ------------------------------ |
| `image`  | file   | yes      | JPEG, PNG, or WebP — max 5 MB |
| `folder` | string | no       | Default: `"products"`; lowercase letters/numbers plus `/`, `_`, `-` only |

Response `200 OK`:

```json
{
  "path": "products/chocolate.jpg",
  "message": "Image uploaded successfully"
}
```

Use the returned `path` value as `image_path` when creating/updating products or categories.

### DELETE `/api/v1/upload/image`

Body (`application/json`):

```json
{
  "path": "products/chocolate.jpg"
}
```

Response `200 OK`:

```json
{
  "message": "Image deleted successfully"
}
```

### GET `/api/v1/upload/images` — List all images (Media Library)

Returns all images in the Cloudflare R2 bucket for the media library. Requires admin auth.

Response `200 OK`:

```json
{
  "images": [
    {
      "name": "products/chocolate.jpg",
      "size": 245000,
      "created_at": "2025-02-15T14:30:00Z",
      "url": "https://pub-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.r2.dev/products/chocolate.jpg"
    }
  ]
}
```

Use the `url` field directly in your frontend `<img>` tags, or use `name` as the `image_path` when creating/updating products.

---

## 9) Health Check

| Operation    | Method | Path      |
| ------------ | ------ | --------- |
| Health check | GET    | `/health` |

Response `200 OK`:

```json
{
  "status": "ok"
}
```

---

---

## 10) Flavors

Flavors represent the available gelato flavors that can be linked to products.

| Operation                        | Method | Path                                       |
| -------------------------------- | ------ | ------------------------------------------ |
| List all active flavors (public) | GET    | `/api/v1/flavors`                          |
| Get flavor by ID (public)        | GET    | `/api/v1/flavors/{id}`                     |
| Create flavor (staff)            | POST   | `/api/v1/flavors`                          |
| Update flavor (staff)            | PATCH  | `/api/v1/flavors/{id}`                     |
| Delete flavor (staff)            | DELETE | `/api/v1/flavors/{id}`                     |
| Link flavor to product (staff)   | POST   | `/api/v1/products/{id}/flavors/{flavorId}` |
| Unlink flavor from product (staff)| DELETE | `/api/v1/products/{id}/flavors/{flavorId}`|

### POST `/api/v1/flavors` — Create

Body:

```json
{
  "name": "Chocolate",
  "description": "Rich dark chocolate",
  "is_active": true
}
```

Response `201 Created`: returns the created flavor object.

### PATCH `/api/v1/flavors/{id}` — Update (partial)

Send only the fields to change:

```json
{
  "name": "Dark Chocolate",
  "is_active": false
}
```

Response `200 OK`: returns the updated flavor.

### POST `/api/v1/products/{id}/flavors/{flavorId}` — Link flavor to product

No body needed. Associates the flavor with the product.

Response `200 OK`:

```json
{
  "message": "Flavor linked to product"
}
```

### DELETE `/api/v1/products/{id}/flavors/{flavorId}` — Unlink

Response `200 OK`:

```json
{
  "message": "Flavor unlinked from product"
}
```

---

## 11) Addons

Addons represent toppings or extras that can be linked to products, organized by `group_name`.

| Operation                        | Method | Path                                       |
| -------------------------------- | ------ | ------------------------------------------ |
| List all active addons (public)  | GET    | `/api/v1/addons`                           |
| Get addon by ID (public)         | GET    | `/api/v1/addons/{id}`                      |
| Create addon (staff)             | POST   | `/api/v1/addons`                           |
| Update addon (staff)             | PATCH  | `/api/v1/addons/{id}`                      |
| Delete addon (staff)             | DELETE | `/api/v1/addons/{id}`                      |
| Link addon to product (staff)    | POST   | `/api/v1/products/{id}/addons/{addonId}`   |
| Unlink addon from product (staff)| DELETE | `/api/v1/products/{id}/addons/{addonId}`   |

### POST `/api/v1/addons` — Create

Body:

```json
{
  "name": "Sprinkles",
  "group_name": "toppings",
  "description": "Colorful sugar sprinkles",
  "is_active": true
}
```

| Field        | Type   | Required | Notes                                      |
| ------------ | ------ | -------- | ------------------------------------------ |
| `name`       | string | yes      | Max 200 chars                              |
| `group_name` | string | no       | Groups addons visually in UI (e.g. `"toppings"`, `"sauces"`) |
| `description`| string | no       |                                            |
| `is_active`  | bool   | no       | Default `true`                             |

Response `201 Created`: returns the created addon.

### PATCH `/api/v1/addons/{id}` — Update (partial)

```json
{
  "group_name": "extras",
  "is_active": false
}
```

Response `200 OK`: returns the updated addon.

---

## 12) Analytics

All analytics endpoints are **admin-only**.

| Operation                 | Method | Path                                  |
| ------------------------- | ------ | ------------------------------------- |
| KPI overview              | GET    | `/api/v1/analytics/overview`          |
| Orders over time          | GET    | `/api/v1/analytics/orders-over-time`  |
| Top-selling products      | GET    | `/api/v1/analytics/top-products`      |

### GET `/api/v1/analytics/overview`

Response `200 OK`:

```json
{
  "total_orders": 128,
  "orders_today": 7,
  "total_revenue": 450.75,
  "revenue_today": 24.50,
  "pending_orders": 3
}
```

### GET `/api/v1/analytics/orders-over-time`

Response `200 OK`:

```json
{
  "data": [
    { "date": "2026-04-06", "count": 4 },
    { "date": "2026-04-07", "count": 6 }
  ]
}
```

### GET `/api/v1/analytics/top-products`

Response `200 OK`:

```json
{
  "products": [
    { "product_name": "Gelato de Chocolate", "total_sold": 42 },
    { "product_name": "Sorbete de Mango",    "total_sold": 31 }
  ]
}
```

---

## 13) Users (Customer Directory)

Users are customer records stored in PostgreSQL via Ent ORM (separate from employees, which are in PocketBase).

| Operation                     | Method | Path                        |
| ----------------------------- | ------ | --------------------------- |
| List users (staff)            | GET    | `/api/v1/users`             |
| Get user by ID (staff)        | GET    | `/api/v1/users/{id}`        |
| Get user's orders (admin)     | GET    | `/api/v1/users/{id}/orders` |
| Create user (admin)           | POST   | `/api/v1/users`             |
| Update user (admin)           | PATCH  | `/api/v1/users/{id}`        |
| Delete user (admin)           | DELETE | `/api/v1/users/{id}`        |

### POST `/api/v1/users` — Create

Body:

```json
{
  "name": "María García",
  "email": "maria@example.com",
  "phone": "+50688887777"
}
```

Response `201 Created`: returns the created user.

### GET `/api/v1/users/{id}/orders`

Returns the orders associated with this user's email or phone, linked at order-creation time.

Response `200 OK`: returns an array of order objects.

---

## 14) Admin Settings

Settings are persisted in PostgreSQL via the `AdminSetting` Ent entity. They control dashboard tab order and token TTL overrides.

| Operation                         | Method | Path                            |
| --------------------------------- | ------ | ------------------------------- |
| Get tab order (staff)             | GET    | `/api/v1/settings/tabs`         |
| Update tab order (admin)          | PATCH  | `/api/v1/settings/tabs`         |
| Get panel config (staff)          | GET    | `/api/v1/settings/panel-config` |
| Update panel config (admin)       | PATCH  | `/api/v1/settings/panel-config` |

### GET & PATCH `/api/v1/settings/tabs`

Stores the admin dashboard tab display order.

PATCH body:

```json
{
  "tabs": ["orders", "products", "categories", "flavors", "addons", "employees", "users", "tools", "settings"]
}
```

Response `200 OK`: returns the persisted tab order.

### GET & PATCH `/api/v1/settings/panel-config`

Stores configurable TTL overrides for auth and tracking tokens.

PATCH body (all fields optional):

```json
{
  "auth_cookie_ttl_hours": 24,
  "auth_token_ttl_hours": 168,
  "tracking_token_ttl_hours": 720
}
```

Response `200 OK`: returns the current panel configuration.

---

## 15) Export / Purge

Admin-only endpoints for order data management.

| Operation              | Method | Path                    |
| ---------------------- | ------ | ----------------------- |
| Export orders (CSV)    | GET    | `/api/v1/export/orders` |
| Purge old orders       | DELETE | `/api/v1/export/orders` |

### GET `/api/v1/export/orders`

Returns a CSV file download containing all orders. Used for backups and reporting.

Response: `text/csv` attachment with orders data.

### DELETE `/api/v1/export/orders`

Permanently deletes orders that meet the purge criteria (e.g., delivered/cancelled orders older than a configured threshold).

Response `200 OK`:

```json
{
  "message": "Orders purged successfully",
  "count": 14
}
```

> **Warning:** This operation is irreversible. Export first with `GET /export/orders` before purging.

---

## Bruno — Testing Guide

### Setup

1. Open Bruno, create a new collection called **ishosBackend**.
2. Create an environment called **dev** with variables:
   - `BASE_URL` = `http://localhost:8080`
   - `AUTH_TOKEN` = (filled in after login)

### Testing Auth (do this first)

**1. Login**

- Method: `POST`
- URL: `{{BASE_URL}}/api/v1/auth/login`
- Headers: `Content-Type: application/json`
- Body:

```json
{
  "email": "admin@ishos.com",
  "password": "securePassword123"
}
```

- Expected status: `200`
- Save `$.token` → variable `AUTH_TOKEN`

**2. Get Session**

- Method: `GET`
- URL: `{{BASE_URL}}/api/v1/auth/session`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Expected status: `200`
- Verify `role` is `"admin"`

**3. Logout**

- Method: `POST`
- URL: `{{BASE_URL}}/api/v1/auth/logout`
- Expected status: `200`

> For all subsequent **authenticated** requests, add the header:
> `Authorization: Bearer {{AUTH_TOKEN}}`

---

### Testing Categories (full CRUD)

**1. Create Category**

- Method: `POST`
- URL: `{{BASE_URL}}/api/v1/categories`
- Headers: `Content-Type: application/json`, `Authorization: Bearer {{AUTH_TOKEN}}`
- Body:

```json
{
  "name": "Gelatos",
  "slug": "gelatos",
  "description": "Italian-style ice cream",
  "display_order": 1
}
```

- Expected status: `201`
- Save `$.id` → variable `CATEGORY_ID`

**2. List Categories**

- Method: `GET`
- URL: `{{BASE_URL}}/api/v1/categories`
- Expected status: `200`
- Verify your category appears in the array

**3. Get Category by Slug**

- Method: `GET`
- URL: `{{BASE_URL}}/api/v1/categories/gelatos`
- Expected status: `200`

**4. Update Category**

- Method: `PATCH`
- URL: `{{BASE_URL}}/api/v1/categories/{{CATEGORY_ID}}`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Body:

```json
{
  "description": "Premium Italian gelato"
}
```

- Expected status: `200`
- Verify `description` changed in response

**5. Delete Category**

- Method: `DELETE`
- URL: `{{BASE_URL}}/api/v1/categories/{{CATEGORY_ID}}`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Expected status: `200`

---

### Testing Products (full CRUD)

> Create a category first (see above) and save its `id` as `CATEGORY_ID`.

**1. Create Product**

- Method: `POST`
- URL: `{{BASE_URL}}/api/v1/products`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Body:

```json
{
  "name": "Gelato de Chocolate",
  "description": "Rich dark chocolate gelato",
  "price": 3.50,
  "category_id": "{{CATEGORY_ID}}",
  "stock_status": "in_stock"
}
```

- Expected status: `201`
- Save `$.id` → variable `PRODUCT_ID`

**2. List Products**

- Method: `GET`
- URL: `{{BASE_URL}}/api/v1/products`
- Expected status: `200`

**3. List Products by Category**

- Method: `GET`
- URL: `{{BASE_URL}}/api/v1/products?category=gelatos`
- Expected status: `200`

**4. Get Single Product**

- Method: `GET`
- URL: `{{BASE_URL}}/api/v1/products/{{PRODUCT_ID}}`
- Expected status: `200`

**5. Update Product**

- Method: `PATCH`
- URL: `{{BASE_URL}}/api/v1/products/{{PRODUCT_ID}}`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Body:

```json
{
  "price": 4.00,
  "description": "Extra rich dark chocolate gelato"
}
```

- Expected status: `200`
- Verify `price` is now `4.00`

**6. Toggle Availability**

- Method: `PATCH`
- URL: `{{BASE_URL}}/api/v1/products/{{PRODUCT_ID}}/availability`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- No body needed
- Expected status: `200`
- Verify `is_available` flipped

**7. Delete Product**

- Method: `DELETE`
- URL: `{{BASE_URL}}/api/v1/products/{{PRODUCT_ID}}`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Expected status: `200`

---

### Testing Orders (create + status flow)

> Create a category and product first (see above). Make sure the product is available.

**1. Create Order (public, no auth needed)**

- Method: `POST`
- URL: `{{BASE_URL}}/api/v1/orders`
- Body:

```json
{
  "customer_name": "María García",
  "customer_phone": "+50688887777",
  "customer_email": "maria@example.com",
  "payment_method": "efectivo",
  "notes": "Extra sprinkles please",
  "items": [
    {
      "product_id": "{{PRODUCT_ID}}",
      "quantity": 2,
      "customizations": { "topping": "sprinkles" }
    }
  ]
}
```

- Expected status: `201`
- Save `$.order_number` for reference

**2. List Orders (requires auth)**

- Method: `GET`
- URL: `{{BASE_URL}}/api/v1/orders`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Expected status: `200`
- Find your order in the list, grab its UUID `id` → save as `ORDER_ID`

**3. Get Single Order**

- Method: `GET`
- URL: `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Expected status: `200`
- Verify items are nested in the response

**4. Approve Order**

- Method: `POST`
- URL: `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}/approve`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- No body needed
- Expected status: `200`
- Verify `status` = `"recibida"` and `payment_status` = `"confirmado"`

**5. Update Status → en_proceso**

- Method: `PATCH`
- URL: `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}/status`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Body:

```json
{
  "status": "en_proceso"
}
```

- Expected status: `200`

**6. Update Status → lista**

- Method: `PATCH`
- URL: `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}/status`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Body:

```json
{
  "status": "lista"
}
```

- Expected status: `200`
- WhatsApp notification is sent to customer

**7. Update Status → entregada**

- Method: `PATCH`
- URL: `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}/status`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Body:

```json
{
  "status": "entregada"
}
```

- Expected status: `200`

**Alternative: Reject an Order (instead of approve)**

- Method: `POST`
- URL: `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}/reject`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Body:

```json
{
  "reason": "Product currently unavailable"
}
```

- Expected status: `200`
- Verify `status` = `"cancelada"`

---

### Testing Employees

**1. Create Employee**

- Method: `POST`
- URL: `{{BASE_URL}}/api/v1/employees`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Body:

```json
{
  "email": "carlos@ishos.com",
  "password": "Secure1234!",
  "name": "Carlos Mora",
  "phone": "+50677776666",
  "role": "employee"
}
```

- Expected status: `201`
- Save `$.id` → variable `EMPLOYEE_ID`

**2. List Employees**

- Method: `GET`
- URL: `{{BASE_URL}}/api/v1/employees`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Expected status: `200`

**3. Deactivate Employee**

- Method: `POST`
- URL: `{{BASE_URL}}/api/v1/employees/{{EMPLOYEE_ID}}/deactivate`
- Headers: `Authorization: Bearer {{AUTH_TOKEN}}`
- Expected status: `200`

---

### Testing Rate Limits

Send more than 5 `POST /api/v1/orders` requests within one hour from the same IP. The 6th request should return:

- Status: `429`
- Body:

```json
{
  "error": "too many orders from your IP, try later"
}
```

---

### Full Bruno test flow (recommended order)

| Step | Action                         | Method | URL                                            | Auth   | Save variable |
| ---- | ------------------------------ | ------ | ---------------------------------------------- | ------ | ------------- |
| 1    | Login                          | POST   | `{{BASE_URL}}/api/v1/auth/login`               | No     | `AUTH_TOKEN`  |
| 2    | Get Session                    | GET    | `{{BASE_URL}}/api/v1/auth/session`             | Bearer | —             |
| 3    | Create Category                | POST   | `{{BASE_URL}}/api/v1/categories`               | Bearer | `CATEGORY_ID` |
| 4    | Create Product                 | POST   | `{{BASE_URL}}/api/v1/products`                 | Bearer | `PRODUCT_ID`  |
| 5    | List Products                  | GET    | `{{BASE_URL}}/api/v1/products`                 | No     | —             |
| 6    | Update Product price           | PATCH  | `{{BASE_URL}}/api/v1/products/{{PRODUCT_ID}}`  | Bearer | —             |
| 7    | Create Order                   | POST   | `{{BASE_URL}}/api/v1/orders`                   | No     | —             |
| 8    | List Orders (get UUID)         | GET    | `{{BASE_URL}}/api/v1/orders`                   | Bearer | `ORDER_ID`    |
| 9    | Approve Order                  | POST   | `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}/approve` | Bearer | —         |
| 10   | Status → `en_proceso`          | PATCH  | `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}/status`  | Bearer | —         |
| 11   | Status → `lista`               | PATCH  | `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}/status`  | Bearer | —         |
| 12   | Status → `entregada`           | PATCH  | `{{BASE_URL}}/api/v1/orders/{{ORDER_ID}}/status`  | Bearer | —         |
| 13   | Create Employee                | POST   | `{{BASE_URL}}/api/v1/employees`                | Bearer | `EMPLOYEE_ID` |
| 14   | Deactivate Employee            | POST   | `{{BASE_URL}}/api/v1/employees/{{EMPLOYEE_ID}}/deactivate`| Bearer | —            |
| 15   | Delete Product                 | DELETE | `{{BASE_URL}}/api/v1/products/{{PRODUCT_ID}}`  | Bearer | —             |
| 16   | Delete Category                | DELETE | `{{BASE_URL}}/api/v1/categories/{{CATEGORY_ID}}`| Bearer | —            |

For each step, assert the expected HTTP status and verify key fields in the response.

---

## HTTP Status Code Reference

| Status | Meaning               | When                                    |
| ------ | --------------------- | --------------------------------------- |
| `200`  | OK                    | Successful GET, PATCH, DELETE           |
| `201`  | Created               | Successful POST (resource created)      |
| `400`  | Bad Request           | Validation error, missing fields        |
| `401`  | Unauthorized          | No valid Bearer token                   |
| `403`  | Forbidden             | Valid token but insufficient role        |
| `404`  | Not Found             | Resource does not exist                 |
| `429`  | Too Many Requests     | Rate limit exceeded on order creation   |
| `500`  | Internal Server Error | Unexpected server failure               |