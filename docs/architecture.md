# Architecture

## System Design

`casais-saas` follows a traditional client-server architecture with an emphasis on real-time state synchronization.

### Backend Structure

- **REST API**: Handles CRUD operations for users, couples, and session metadata.
- **Socket.io Server**: Manages live connections, room-based messaging (one room per couple), and event broadcasting.
- **Data Layer**: Direct interactions with PostgreSQL via `pg`.

### Frontend Structure

- **Components**: Functional React components styled with Tailwind 4.
- **State Management**: React Context or dedicated hooks for managing WebSocket state.
- **Animations**: Framer Motion for smooth transitions between mediation steps.

## Communication Flow

1. **Authentication**: User logs in via REST, receives a JWT.
2. **WebSocket Handshake**: Client connects to Socket.io with the JWT for authentication.
3. **Room Join**: Backend automatically places the user in a room unique to their couple.
4. **Interaction**: Messages or state changes (e.g., "Step Completed") are emitted via Sockets and persisted to the DB.
