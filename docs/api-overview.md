# API Overview

Base URL: `/api/v1`

## Public

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/forgot-password`
- `GET /packages`
- `POST /admin/auth/login`
- `POST /payments/callback/alipay`

## User

- `GET /user/profile`
- `POST /user/devices/bind`
- `GET /user/orders`
- `GET /user/orders/:orderNo`
- `POST /user/orders`
- `POST /user/orders/:orderNo/cancel`
- `POST /user/cards/redeem`

## Admin

- `GET /admin/dashboard`
- `GET /admin/users`
- `PUT /admin/users/:id/status`
- `POST /admin/users/:id/member`
- `GET /admin/packages`
- `POST /admin/packages`
- `PUT /admin/packages/:id`
- `GET /admin/orders`
- `GET /admin/payments`
- `POST /admin/cards/batch`
- `GET /admin/cards`
- `POST /admin/configs`
- `GET /admin/configs`
