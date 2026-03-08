# ishosBackend — API Documentation

Base URL: `http://localhost:8080` (default port, configurable via `PORT` env var)

All business endpoints live under `/api/v1/...`.

---

## A note on API versioning

All routes use the `/api/v1/` prefix (e.g. `/api/v1/products`, `/api/v1/orders`).
This follows best practice so that any future breaking changes can be introduced
under a new version without disrupting existing clients.

**When you need a v2:**

- Ship `/api/v2/products` with the new schema without breaking existing clients.
- Deprecate `/api/v1/...` gradually while clients migrate.
- Run both versions side-by-side during a transition period.

In Chi this is straightforward — add a second route group:

```go
r.Route("/api/v1", func(api chi.Router) { /* current v1 routes */ })
r.Route("/api/v2", func(api chi.Router) { /* new v2 routes */ })
```

The docs below use the **current** paths (`/api/v1/...`).

---

## Common headers

| Header          | Value                    | When                                     |
| --------------- | ------------------------ | ---------------------------------------- |
| `Content-Type`  | `application/json`       | All POST / PATCH requests                |
| `Authorization` | `Bearer <PB_AUTH_TOKEN>` | Authenticated endpoints (admin/employee) |

Authentication uses **PocketBase JWT tokens** via the `Authorization` header.
Public endpoints (list products, create order) require no auth.

---

## Error responses

All errors return JSON:

```json
{
  "error": "Description of the problem"
}
```

Common status codes: `400` (bad request), `401` (unauthorized), `403` (forbidden), `404` (not found), `429` (rate limited), `500` (server error).

---

## 1) Products

| Operation                             | Method | Path                               |
| ------------------------------------- | ------ | ---------------------------------- |
| List all (public: available only)     | GET    | `/api/v1/products`                 |
| List all (admin: include unavailable) | GET    | `/api/v1/products?all=true`        |
| Filter by category slug              | GET    | `/api/v1/products?category=gelatos`|
| Get one by ID                         | GET    | `/api/v1/products/{id}`            |
| Create (admin)                        | POST   | `/api/v1/products`                 |
| Update (admin, partial)               | PATCH  | `/api/v1/products/{id}`            |
| Toggle availability (admin)           | PATCH  | `/api/v1/products/{id}/availability`|
| Delete (admin)                        | DELETE | `/api/v1/products/{id}`            |

### POST `/api/v1/products` — Create

Body:

```json
{
  "name": "Gelato de Chocolate",
  "description": "Rich dark chocolate gelato",
  "price": 3.50,
  "category_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "image_path": "products/chocolate_20250215T143000.jpg",
  "stock_status": "in_stock"
}
```

Fields:

| Field          | Type   | Required | Notes                                                           |
| -------------- | ------ | -------- | --------------------------------------------------------------- |
| `name`         | string | yes      | Max 200 chars                                                   |
| `description`  | string | yes      | Product description                                             |
| `price`        | float  | yes      | Must be positive (e.g. `3.50`)                                  |
| `category_id`  | UUID   | yes      | Must reference an existing category                             |
| `image_path`   | string | no       | Supabase object path — get it from `POST /api/v1/upload/image`  |
| `stock_status` | string | no       | `"in_stock"` (default) \| `"out_of_stock"` \| `"discontinued"` |

Response `201 Created`:

```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "name": "Gelato de Chocolate",
  "description": "Rich dark chocolate gelato",
  "price": 3.5,
  "category_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "category_name": "Gelatos",
  "image_url": "https://xxx.supabase.co/storage/v1/object/public/product-images/products/chocolate_20250215T143000.jpg",
  "is_available": true,
  "stock_status": "in_stock",
  "created_at": "2025-02-15T14:30:00Z"
}
```

### PATCH `/api/v1/products/{id}` — Update (partial)

Send **only** the fields you want to change:

```json
{
  "price": 4.00,
  "is_available": false,
  "stock_status": "out_of_stock"
}
```

Patchable fields: `name`, `description`, `price`, `image_path`, `is_available`, `stock_status`.

Response `200 OK`: returns the updated product object.

### PATCH `/api/v1/products/{id}/availability` — Toggle

No body needed. Flips `is_available` from `true` → `false` or vice versa.

Response `200 OK`: returns the updated product.

### DELETE `/api/v1/products/{id}`

Response `200 OK`:

```json
{
  "message": "Product deleted"
}
```

---

## 2) Categories

| Operation                   | Method | Path                        |
| --------------------------- | ------ | --------------------------- |
| List all active (sorted)    | GET    | `/api/v1/categories`        |
| Get by slug (with products) | GET    | `/api/v1/categories/{slug}` |
| Create (admin)              | POST   | `/api/v1/categories`        |
| Update (admin, partial)     | PATCH  | `/api/v1/categories/{id}`   |
| Delete (admin)              | DELETE | `/api/v1/categories/{id}`   |

> **Important:** GET uses `{slug}` (e.g. `gelatos`), but PATCH and DELETE use `{id}` (UUID).

### POST `/api/v1/categories` — Create

Body:

```json
{
  "name": "Gelatos",
  "slug": "gelatos",
  "description": "Italian-style ice cream",
  "image_path": "categories/gelatos.jpg",
  "display_order": 1
}
```

Fields:

| Field           | Type   | Required | Notes                                          |
| --------------- | ------ | -------- | ---------------------------------------------- |
| `name`          | string | yes      | Max 100, unique                                |
| `slug`          | string | yes      | Max 100, unique, URL-friendly (e.g. `gelatos`) |
| `description`   | string | no       | Defaults to empty string                       |
| `image_path`    | string | no       | Supabase object path                           |
| `display_order` | int    | no       | Controls sort order, defaults to `0`           |

Response `201 Created`:

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name": "Gelatos",
  "slug": "gelatos",
  "description": "Italian-style ice cream",
  "image_path": "categories/gelatos.jpg",
  "display_order": 1,
  "is_active": true,
  "created_at": "2025-02-15T14:00:00Z"
}
```

### PATCH `/api/v1/categories/{id}` — Update (partial)

```json
{
  "description": "Authentic Italian gelato",
  "display_order": 2,
  "is_active": false
}
```

To remove an existing `image_path`, send one of these values:

```json
{
  "image_path": ""
}
```

or

```json
{
  "image_path": "NULL"
}
```

Both are normalized by the backend and stored as `NULL` in DB, so `image_path` is omitted in the response JSON.

Patchable fields: `name`, `slug`, `description`, `image_path`, `display_order`, `is_active`.

Response `200 OK`: returns the updated category.

### DELETE `/api/v1/categories/{id}`

Products in this category are **not** deleted — they become uncategorised.

Response `200 OK`:

```json
{
  "message": "Category deleted"
}
```

---

## 3) Orders

| Operation                                    | Method | Path                                                           |
| -------------------------------------------- | ------ | -------------------------------------------------------------- |
| Create order (public, rate-limited)          | POST   | `/api/v1/orders`                                               |
| Track order by number + phone (public)       | GET    | `/api/v1/orders/track?order_number=...&customer_phone=...`    |
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
  "message": "Orden recibida, te notificaremos pronto 🍦"
}
```

A WhatsApp notification is automatically sent to the customer.

### GET `/api/v1/orders/track` — Public tracking

Query params:

- `order_number` (required)
- `customer_phone` (required)

Response `200 OK`:

```json
{
  "order_number": "ORD-20250215-4821",
  "status": "en_proceso",
  "updated_at": "2025-02-15T15:10:00Z",
  "created_at": "2025-02-15T14:30:00Z",
  "total_amount": 10.50,
  "order_type": "para_llevar"
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

Response `200 OK`:

```json
{
  "message": "Logged out successfully. Discard the token on the client."
}
```

### Token lifecycle

Tokens are PocketBase JWTs with a configurable expiration (default: 14 days). When a token expires, the user must log in again via `POST /api/v1/auth/login`.

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

## 8) Image upload (Supabase Storage)

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
  "path": "products/chocolate_20250215T143000.jpg",
  "message": "Image uploaded successfully"
}
```

Use the returned `path` value as `image_path` when creating/updating products or categories.

### DELETE `/api/v1/upload/image`

Body (`application/json`):

```json
{
  "path": "products/chocolate_20250215T143000.jpg"
}
```

Response `200 OK`:

```json
{
  "message": "Image deleted successfully"
}
```

### GET `/api/v1/upload/images` — List all images (Media Library)

Returns all images in the Supabase bucket for the media library. Requires admin auth.

Response `200 OK`:

```json
{
  "images": [
    {
      "name": "products/chocolate_20250215T143000.jpg",
      "size": 245000,
      "created_at": "2025-02-15T14:30:00Z",
      "url": "https://qtfvvltfzjgpenwddqyr.supabase.co/storage/v1/object/public/menu/products/chocolate_20250215T143000.jpg"
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