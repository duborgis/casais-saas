# Code Style Rules for casais-saas

## Backend (`backend/`)
- Use ES Modules (`import`/`export`).
- Use Express 5 features.
- Keep `index.js` as the entry point.
- Prefer `const` over `let`.
- Error handling: use middleware for centralized error management.

## Frontend (`frontend/`)
- Use React 19 functional components with Hooks.
- Use TypeScript for all components and utilities.
- Styling: Use TailwindCSS 4 utility classes.
- Animation: Use Framer Motion for UI transitions.
- Component structure: One component per file in `src/components`.
- Icons: Use `lucide-react`.
