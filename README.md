# TicketDot Backend API 🚌⚙️
> High-performance, concurrent bus ticket reservation and fleet management REST API built with Go, Chi router, and PostgreSQL.

---

## 📑 Table of Contents
1. [Overview](#-overview)
2. [Architecture & Design](#-architecture--design)
3. [Tech Stack](#-tech-stack)
4. [Database & Migrations](#-database--migrations)
5. [Getting Started & Local Setup](#-getting-started--local-setup)
   - [Prerequisites](#prerequisites)
   - [Environment Variables](#environment-variables)
   - [Database Initialization](#database-initialization)
   - [Starting the API Server](#starting-the-api-server)
6. [Pre-Seeded Credentials & Test Data](#-pre-seeded-credentials--test-data)
7. [API Reference](#-api-reference)
   - [Authentication](#1-authentication)
   - [Public Search & City Discovery](#2-public-search--city-discovery)
   - [Seat Management & Bookings](#3-seat-management--bookings)
   - [Operator Management](#4-operator-management)
   - [User Profile](#5-user-profile)
8. [Concurrency & Concurrency Controls](#-concurrency--concurrency-controls)
9. [Project Structure](#-project-structure)
10. [Running Tests](#-running-tests)

---

## 🌟 Overview

The TicketDot backend provides a complete set of RESTful services powering the TicketDot bus booking platform. It handles:
- **City Discovery & Route Planning**: Idempotently seeded Indian cities linked into operator routes.
- **Seat Inventory Lifecycle**: Physical bus layouts mapped to per-schedule seat inventories with dynamic availability computation.
- **Atomic Multi-Seat Locking**: Prevents double-booking via transactional all-or-nothing locking with a 5-minute lock window.
- **Automated Expired-Lock Cleanup**: Background goroutine continuously releasing expired locks back to available inventory.
- **Role-Based Access Control**: Strict role enforcement (`passenger` and `operator_admin`) using HMAC-SHA256 JWTs.
- **Operator Fleet & Manifest Operations**: Real-time fleet metrics, trip scheduling, and enriched passenger manifests.

---

## 🏗 Architecture & Design

The backend follows a clean, layered architectural pattern:

```mermaid
flowchart TD
    Client["Client (Web / Mobile / cURL)"] --> HTTP["HTTP Router (Chi v5)"]
    
    subgraph Middleware
        CORS["CORS Middleware"]
        AuthMW["JWT Auth Middleware"]
        RoleMW["Role Guard Middleware"]
    end
    
    HTTP --> CORS
    CORS --> AuthMW
    AuthMW --> RoleMW
    
    subgraph Handlers ["Handlers Layer"]
        HAuth[AuthHandler]
        HSearch[SearchHandler]
        HSeat[SeatHandler]
        HBooking[BookingHandler]
        HOperator[OperatorHandler]
        HProfile[ProfileHandler]
    end
    
    RoleMW --> HBooking
    RoleMW --> HOperator
    RoleMW --> HProfile
    CORS --> HSearch
    CORS --> HSeat
    CORS --> HAuth
    
    subgraph Services ["Service Layer (Business Logic)"]
        SAuth[AuthService]
        SSearch[SearchService]
        SSeat[SeatService]
        SBooking[BookingService]
        SOperator[OperatorService]
        SProfile[ProfileService]
    end
    
    HAuth --> SAuth
    HSearch --> SSearch
    HSeat --> SSeat
    HBooking --> SBooking
    HOperator --> SOperator
    HProfile --> SProfile
    
    subgraph Repositories ["Repository Layer (PostgreSQL)"]
        RAuth[AuthRepository]
        RSearch[SearchRepository]
        RSeat[SeatRepository]
        RBooking[BookingRepository]
        ROperator[OperatorRepository]
        RProfile[ProfileRepository]
    end
    
    SAuth --> RAuth
    SSearch --> RSearch
    SSeat --> RSeat
    SBooking --> RBooking
    SOperator --> ROperator
    SProfile --> RProfile
    
    subgraph Storage ["PostgreSQL"]
        DB[(BusTicket DB)]
    end
    
    RAuth & RSearch & RSeat & RBooking & ROperator & RProfile --> DB
    
    subgraph BackgroundWorker ["Background Worker"]
        Cleaner[ExpiredLockCleaner] -. Sweeps every 30s .-> DB
    end
```

---

## 🛠 Tech Stack

- **Language**: Go 1.22+
- **Router**: [`github.com/go-chi/chi/v5`](https://github.com/go-chi/chi)
- **Database**: PostgreSQL 14+ with `github.com/lib/pq`
- **Authentication**: JWT (`golang-jwt/jwt/v5`) using HMAC-SHA256 with 24-hour token duration
- **Password Hashing**: `golang.org/x/crypto/bcrypt`
- **Configuration**: Environment variables with `github.com/joho/godotenv`
- **Migrations**: Native Go `embed.FS` runner with `schema_migrations` tracking table

---

## 🗄 Database & Migrations

The database migrations are embedded directly into the Go binary (`migrations/embed.go`). When the server boots (`cmd/api/main.go`), it invokes `migrations.Run(db)`. It automatically checks the `schema_migrations` table and applies unapplied migrations in chronological order inside atomic transactions.

| Migration File | Key Responsibilities |
|---|---|
| `001_initial_schema.sql` | `pgcrypto` extension, `user_role` enum (`passenger`, `operator_admin`, `super_admin`), `users` table. |
| `002_search_schema.sql` | `cities`, `operators`, `routes`, `buses`, and `schedules` tables. Initial seeds for Mumbai, Pune, Shivneri Travels. |
| `003_seat_schema.sql` | `seat_layouts` (bus template) and `seat_inventory` (per-schedule inventory). |
| `004_booking_schema.sql` | `bookings` table storing reference, user, schedule, seat, amount, status, timestamps. |
| `005_fixes.sql` | Adds lock tracking columns (`locked_by`, `locked_at`, `lock_expires`) to `seat_inventory`, adds `operator_id` to `routes`, and creates partial unique index `idx_bookings_schedule_seat_confirmed` on `bookings(schedule_id, seat_number) WHERE status = 'confirmed'`. |
| `006_multi_seat_bookings.sql` | Drops single-seat unique constraint on `booking_ref` in `bookings` to support multi-seat bookings under one reference; adds index on `booking_ref`. |
| `007_seed_cities.sql` | Seeds 15 major Indian cities (`Mumbai`, `Pune`, `Bengaluru`, `Chennai`, `Hyderabad`, `Delhi`, `Goa`, `Kochi`, etc.) idempotently. |

---

## 🚀 Getting Started & Local Setup

### Prerequisites
- **Go**: Version `1.22` or higher ([Download](https://golang.org/dl/))
- **PostgreSQL**: Version `14` or higher ([Download](https://www.postgresql.org/download/))

---

### Environment Variables

Copy `.env.example` to `.env`:

```bash
cp .env.example .env
```

Configure your `.env` variables:

```env
DATABASE_URL=postgres://postgres:your_password@localhost:5432/BusTicket?sslmode=disable
JWT_SECRET=your-secure-random-jwt-secret-key-32-chars-min
PORT=8080
```

> **Note**: The application validates `JWT_SECRET` and `DATABASE_URL` at boot and will intentionally fail startup if either is missing.

---

### Database Initialization

Create a new PostgreSQL database matching the name in your `DATABASE_URL`:

```sql
CREATE DATABASE "BusTicket";
```

No manual SQL file execution is required. When the API starts, all 7 migrations execute automatically!

---

### Starting the API Server

1. Download Go module dependencies:
   ```bash
   go mod tidy
   ```

2. Run the application:
   ```bash
   go run cmd/api/main.go
   ```

3. Successful startup output:
   ```
   2026/10/15 12:20:00 Database connected
   2026/10/15 12:20:00 applying migration: 001_initial_schema.sql
   ...
   2026/10/15 12:20:01 applied all migrations successfully
   2026/10/15 12:20:01 Expired lock cleaner started
   2026/10/15 12:20:01 Server running on :8080
   ```

---

## 🔑 Pre-Seeded Credentials & Test Data

The database includes pre-configured accounts for testing:

| Role | Email | Password | Details |
|---|---|---|---|
| **Passenger** | `bob@example.com` | `password123` | Can search trips, lock seats, confirm bookings, cancel reservations, and view tickets. |
| **Operator Admin** | `shinas@gmail.com` | `password123` | Associated with *Shivneri Travels*. Can manage fleet buses, routes, schedules, and view passenger manifests. |

---

## 📡 API Reference

Base URL: `http://localhost:8080`

### 1. Authentication

#### Register User
- **Method & Path**: `POST /auth/signup`
- **Request**:
  ```json
  {
    "name": "Jane Doe",
    "email": "jane@example.com",
    "password": "password123",
    "phone": "+919876543210",
    "role": "passenger"
  }
  ```
- **Response**: `201 Created`

#### Login
- **Method & Path**: `POST /auth/login`
- **Request**:
  ```json
  {
    "email": "bob@example.com",
    "password": "password123"
  }
  ```
- **Response**: `200 OK`
  ```json
  {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": "e0b5756c-0e27-4c7b-9ebc-2792da0d7c49",
      "name": "Bob",
      "email": "bob@example.com",
      "role": "passenger"
    }
  }
  ```

#### Current User
- **Method & Path**: `GET /auth/me`
- **Headers**: `Authorization: Bearer <token>`
- **Response**: `200 OK`

---

### 2. Public Search & City Discovery

#### List Cities
- **Method & Path**: `GET /cities`
- **Response**: `200 OK`
  ```json
  {
    "data": [
      { "id": "11111111-1111-1111-1111-111111111101", "name": "Mumbai", "state": "Maharashtra" },
      { "id": "11111111-1111-1111-1111-111111111102", "name": "Pune", "state": "Maharashtra" },
      { "id": "11111111-1111-1111-1111-111111111103", "name": "Bengaluru", "state": "Karnataka" }
    ]
  }
  ```

#### Search Available Buses
- **Method & Path**: `GET /buses/search?origin=Mumbai&destination=Pune&travel_date=2026-10-20`
- **Query Parameters**:
  - `origin` (string, required): Origin city name
  - `destination` (string, required): Destination city name
  - `travel_date` (string, optional): Travel date `YYYY-MM-DD`
- **Response**: `200 OK`
  ```json
  {
    "count": 1,
    "data": [
      {
        "schedule_id": "42bfb73e-324d-4eb8-b99f-7a4cbfae6d34",
        "operator_name": "Shivneri Travels",
        "bus_id": "89ec9c0e-43f1-4db8-8422-7776b6b7a111",
        "bus_type": "AC Sleeper",
        "departure_time": "2026-10-20T06:00:00Z",
        "arrival_time": "2026-10-20T10:00:00Z",
        "available_seats": 28,
        "base_fare": 850.00
      }
    ]
  }
  ```
  *(Note: `available_seats` is computed dynamically from real-time available `seat_inventory` rows).*

#### Get Schedule Details
- **Method & Path**: `GET /buses/{scheduleID}`
- **Response**: `200 OK`
  ```json
  {
    "data": {
      "schedule_id": "42bfb73e-...",
      "origin": "Mumbai",
      "destination": "Pune",
      "operator_name": "Shivneri Travels",
      "bus_type": "AC Sleeper",
      "departure_time": "2026-10-20T06:00:00Z",
      "arrival_time": "2026-10-20T10:00:00Z",
      "base_fare": 850.00
    }
  }
  ```

#### Get Real-Time Seat Status
- **Method & Path**: `GET /buses/{scheduleID}/seats`
- **Response**: `200 OK`
  ```json
  {
    "seats": [
      { "seat_number": "A1", "status": "available", "seat_type": "seater", "is_window": true },
      { "seat_number": "A2", "status": "locked", "seat_type": "seater", "is_window": false },
      { "seat_number": "B1", "status": "booked", "seat_type": "sleeper", "is_window": true }
    ]
  }
  ```

---

### 3. Seat Management & Bookings

#### Lock Seats (Atomic, All-or-Nothing)
- **Method & Path**: `POST /bookings/lock`
- **Headers**: `Authorization: Bearer <token>`
- **Request**:
  ```json
  {
    "schedule_id": "42bfb73e-...",
    "seats": ["A1", "A2"]
  }
  ```
- **Rules**:
  - Maximum 6 seats per request.
  - All-or-nothing: if any seat is unavailable/locked by someone else, transaction aborts and names the unavailable seat.
  - Expired locks from previous users are treated as available.
  - Rejects schedules that have already departed.
- **Response**: `200 OK`
  ```json
  {
    "message": "seat locked successfully",
    "data": {
      "schedule_id": "42bfb73e-...",
      "seats": ["A1", "A2"],
      "lock_expires": "2026-10-15T12:40:00Z"
    }
  }
  ```

#### Confirm Booking
- **Method & Path**: `POST /bookings/confirm`
- **Headers**: `Authorization: Bearer <token>`
- **Request**:
  ```json
  {
    "schedule_id": "42bfb73e-...",
    "seats": ["A1", "A2"]
  }
  ```
- **Rules**:
  - Rejects if lock has expired (`lock_expires < NOW()`).
  - Generates a single, cryptographically random 8-character `booking_ref` (e.g. `K9F2Q8B1`).
  - Updates `seat_inventory` status to `'booked'`.
- **Response**: `200 OK`
  ```json
  {
    "message": "booking confirmed",
    "data": {
      "booking_ref": "K9F2Q8B1",
      "schedule_id": "42bfb73e-...",
      "seats": ["A1", "A2"],
      "status": "confirmed",
      "total_amount": 1700.00
    }
  }
  ```

#### Release Held Seats
- **Method & Path**: `POST /bookings/release`
- **Headers**: `Authorization: Bearer <token>`
- **Request**:
  ```json
  {
    "schedule_id": "42bfb73e-...",
    "seats": ["A1", "A2"]
  }
  ```
- **Response**: `200 OK`

#### Get Current User's Bookings
- **Method & Path**: `GET /bookings/my-bookings`
- **Headers**: `Authorization: Bearer <token>`
- **Response**: `200 OK` (Consolidated multi-seat bookings grouped by `booking_ref`).

#### Get Single Booking by Reference
- **Method & Path**: `GET /bookings/{bookingRef}`
- **Headers**: `Authorization: Bearer <token>`
- **Response**: `200 OK`

#### Cancel Booking
- **Method & Path**: `POST /bookings/{bookingRef}/cancel`
- **Headers**: `Authorization: Bearer <token>`
- **Response**: `200 OK` (Sets booking status to `'cancelled'` and frees all seats back to `'available'`).

---

### 4. Operator Management

All operator endpoints require a valid JWT with the `operator_admin` role.

#### Dashboard Statistics
- **Method & Path**: `GET /operator/dashboard`
- **Response**: `200 OK`
  ```json
  {
    "total_buses": 3,
    "total_routes": 4,
    "total_schedules": 6,
    "total_bookings": 12,
    "total_revenue": 10200.00
  }
  ```

#### Bus Fleet Management
- `GET /operator/buses`: List operator's buses.
- `POST /operator/buses`: Register a new bus.
  ```json
  {
    "registration_no": "MH-12-AB-9999",
    "bus_type": "AC Sleeper",
    "total_seats": 30
  }
  ```
  *(Automatically generates 30 physical seat layout rows for the bus).*
- `PUT /operator/buses/{busID}`: Update bus details.
- `DELETE /operator/buses/{busID}`: Remove bus.

#### Route Management
- `GET /operator/routes`: List routes created by or assigned to this operator.
- `POST /operator/routes`: Create route.
  ```json
  {
    "origin_city_id": "11111111-1111-1111-1111-111111111101",
    "destination_city_id": "11111111-1111-1111-1111-111111111102"
  }
  ```

#### Schedule Management
- `GET /operator/schedules`: List trips scheduled by the operator.
- `POST /operator/schedules`: Schedule trip.
  ```json
  {
    "route_id": "33333333-3333-3333-3333-333333333301",
    "bus_id": "89ec9c0e-43f1-4db8-8422-7776b6b7a111",
    "departure_time": "2026-10-25T06:00:00Z",
    "arrival_time": "2026-10-25T11:00:00Z",
    "base_fare": 950.00
  }
  ```
  *(Automatically populates `seat_inventory` rows derived from the bus's seat layout).*

#### Enriched Passenger Manifest
- **Method & Path**: `GET /operator/bookings`
- **Response**: `200 OK`
  ```json
  {
    "data": [
      {
        "booking_ref": "K9F2Q8B1",
        "schedule_id": "42bfb73e-...",
        "passenger_name": "Bob",
        "passenger_email": "bob@example.com",
        "passenger_phone": "+919876543210",
        "origin": "Mumbai",
        "destination": "Pune",
        "bus_number": "MH-12-AB-1234",
        "seat_number": "A1, A2",
        "status": "confirmed",
        "amount": 1700.00,
        "confirmed_at": "2026-10-15T12:30:00Z"
      }
    ]
  }
  ```

---

### 5. User Profile

- `GET /profile`: Get logged-in user profile details.
- `PUT /profile`: Update name, phone, or avatar.

---

## 🔒 Concurrency & Concurrency Controls

1. **Atomic Transactions**: Lock operations execute within `BEGIN ... COMMIT` blocks with `SELECT ... FOR UPDATE` row locks, guaranteeing no race conditions when multiple users attempt locking the same seat simultaneously.
2. **Double-Booking Guard**: Partial unique index on the database level:
   ```sql
   CREATE UNIQUE INDEX idx_bookings_schedule_seat_confirmed
   ON bookings (schedule_id, seat_number)
   WHERE status = 'confirmed';
   ```
3. **Automatic Cleaner Daemon**: Background goroutine running `cleanup.StartExpiredLockCleaner(ctx, db)` sweeps every 30 seconds to reclaim abandoned locks (`lock_expires < NOW() AND status = 'locked'`).

---

## 📁 Project Structure

```
TicketDot/
├── cmd/
│   └── api/
│       └── main.go                 # Bootstrapper, env validation, migration runner
├── internal/
│   ├── auth/                       # Authentication & JWT
│   │   ├── handler/                # Auth HTTP handlers (Signup, Login, Me)
│   │   ├── middleware/             # JWT & Role authorization middlewares
│   │   ├── model/                  # User model & DTOs
│   │   ├── repository/             # User SQL persistence
│   │   └── service/                # Password hashing, JWT creation & validation
│   ├── booking/                    # Seat reservation core
│   │   ├── cleanup/                # Expired lock cleaner daemon
│   │   ├── handler/                # Lock, Confirm, Release, Cancel handlers
│   │   ├── model/                  # Booking structs & JSON contracts
│   │   ├── repository/             # Booking queries & transactional locking
│   │   └── service/                # 8-char crypto booking_ref generator
│   ├── db/                         # PostgreSQL connection pool initializer
│   ├── http/middleware/            # CORS middleware
│   ├── operator/                   # Operator portal backend
│   │   ├── handler/                # Bus, Route, Schedule, Manifest handlers
│   │   ├── model/                  # Operator models & DTOs
│   │   ├── repository/             # Operator SQL queries
│   │   └── service/                # Operator business logic
│   ├── profile/                    # Profile management
│   ├── routes/                     # Central Chi router route registry
│   ├── search/                     # Cities discovery & bus search
│   └── seat/                       # Physical layout & seat inventory
├── migrations/                     # Embedded SQL migrations (001 - 007)
│   ├── 001_initial_schema.sql
│   ├── 002_search_schema.sql
│   ├── 003_seat_schema.sql
│   ├── 004_booking_schema.sql
│   ├── 005_fixes.sql
│   ├── 006_multi_seat_bookings.sql
│   ├── 007_seed_cities.sql
│   ├── embed.go                    # go:embed filesystem
│   └── runner.go                   # Automated migration runner
├── .env.example
├── go.mod
├── go.sum
└── README.md
```

---

## 🧪 Running Tests

Execute all backend unit and repository tests:

```bash
go test -v ./...
```

Run tests with race detection enabled:

```bash
go test -race ./...
```
