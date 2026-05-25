# casais-saas

SaaS application for couples mediation and management.

## Agent Behavior Mandate
Before implementing any significant change, you MUST investigate and ask clarifying questions to ensure full alignment with the user's needs. Do not proceed based on intuition or assumptions alone; validate the direction first.

## Build Commands
- **Backend**:
  - Run dev: `cd backend && npm run dev`
  - Start prod: `cd backend && npm start`
- **Frontend**:
  - Run dev: `cd frontend && npm run dev`
  - Build: `cd frontend && npm run build`

## Test Commands
- **Backend**: `cd backend && npm test` (Currently not implemented)
- **Frontend**: None currently implemented.

## Lint/Format Commands
- **Frontend**: `cd frontend && npm run lint`

## Style & Architecture
- **Backend**: Node.js with Express, Socket.io for real-time communication, PostgreSQL via `pg`.
- **Frontend**: React 19 (Vite), TypeScript, TailwindCSS 4, Framer Motion for animations.
- **Communication**: REST API + WebSockets (Socket.io).
- **Security**: JWT-based authentication, `bcryptjs` for password hashing.

## Project Context
- **Mediation**: See `MEDIATION_GUIDE.md` for logic details.
- **Real-time**: Extensive use of Socket.io for chat and shared state between couples.
