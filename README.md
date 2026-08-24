# 🏃‍♂️ Personal Finance Backend (Go)

Backend for a personal finance mobile app, built with Go, Gin and PostgreSQL.

## 📦 Requirements

- Go 1.24 or newer
- PostgreSQL ([Neon](https://neon.tech) works well for hosted environments)
- Git

## ⚙️ Environment variables

Create a `.env` file in the project root:

```env
PORT=8000
POSTGRES_URL=postgres://user:password@your-host.neon.tech:5432/your_database
SECRET_WORD=super_secret_key
ENV=development
```

## Setup

1. Clone the repository

```bash
git clone git@github.com:Befo0/pdm-backend.git
cd pdm-backend
```

2. Install dependencies

```bash
go mod tidy
```

3. Run the database migrations

```bash
go run cmd/migrations/main.go
```

4. Start the server

```bash
go run main.go
```

The API listens on http://localhost:8000

To drop every table and start over:

```bash
go run cmd/resetdb/main.go
```

## API

All routes except `POST /auth/login` and `POST /auth/register` require an
`Authorization: Bearer <token>` header.

| Method | Path | Description |
| --- | --- | --- |
| POST | `/auth/register` | Create an account and its personal finance |
| POST | `/auth/login` | Exchange credentials for a token |
| PATCH | `/users/me` | Update name and email |
| PATCH | `/users/me/password` | Change password |
| GET | `/finances/summary` | Dashboard totals for a month |
| GET | `/finances/breakdown` | Per-category budget vs. spend |
| GET | `/categories` | List expense categories |
| GET | `/categories/options` | Categories as selectable options |
| GET | `/categories/:id/breakdown` | Budget breakdown for one category |
| POST | `/categories` | Create a category |
| PATCH | `/categories/:id` | Rename a category |
| GET | `/subcategories` | List expense subcategories |
| GET | `/subcategories/budget-types` | Available budget types |
| GET | `/subcategories/:id` | Fetch one subcategory |
| POST | `/subcategories` | Create a subcategory |
| PUT | `/subcategories/:id` | Update a subcategory |
| GET | `/transactions` | Transactions for a month |
| GET | `/transactions/options` | Selectable income sources and subcategories |
| GET | `/transactions/:id` | Transaction details |
| POST | `/transactions` | Record a transaction |
| GET | `/income-sources` | List income sources |
| GET | `/income-sources/:id` | Fetch one income source |
| POST | `/income-sources` | Create an income source |
| PUT | `/income-sources/:id` | Update an income source |
| GET | `/savings` | Monthly goals and progress for a year |
| POST | `/savings/goals` | Create or update a monthly savings goal |
| GET | `/shared-finances` | Shared finances the user belongs to |
| GET | `/shared-finances/:id` | Shared finance details and members |
| POST | `/shared-finances` | Create a shared finance |
| POST | `/shared-finances/join` | Join using an invitation code |
| DELETE | `/shared-finances/:id/leave` | Leave a shared finance |
| DELETE | `/shared-finances/members/:id` | Remove a member |
| POST | `/invitations/:id` | Create an invitation code for a finance |
| GET | `/ws/finances/:id` | WebSocket stream of finance events |

Endpoints that operate on a finance accept an optional `?finance_id=` query
parameter; without it they fall back to the caller's personal finance.
Month-scoped endpoints take `?month=` and `?year=`.
