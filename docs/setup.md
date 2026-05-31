# Getting Started

## Prerequisites

- **Node.js** (LTS version recommended)
- **PostgreSQL** instance
- **npm** or **yarn**

## Local Development Setup

The project is split into `backend` and `frontend`. You will need to run both for a full experience.

### 1. Backend Setup

```bash
cd backend
npm install
cp .env.example .env
# Edit .env with your DATABASE_URL and JWT_SECRET
npm run dev
```

### 2. Frontend Setup

```bash
cd frontend
npm install
npm run dev
```

The frontend will typically be available at `http://localhost:5173`.

## Environment Variables

### Backend
- `DATABASE_URL`: PostgreSQL connection string.
- `JWT_SECRET`: Secret key for token signing.
- `PORT`: Server port (default: 3000).

### Frontend
- `VITE_API_URL`: URL of the backend API.
- `VITE_SOCKET_URL`: URL of the Socket.io server.
