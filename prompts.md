# LLM prompts used during implementation

This file is intended to satisfy the assignment requirement that prompts used while building the application are checked into the repository.

## Prompt 1 — architecture and implementation

Build a web application with two flows: registration and user recognition/login during checkout. Registration collects email, first name, and last name, stores the user, generates a random 6-digit numeric code, and displays it. Checkout collects email, phone, and shipping address. When a complete email is entered, recognize a registered user in the background. If registered, show a modal for the 6-digit code with a skip option. Validate the code, create a login session, show the user's name after login, and record checkout data in PostgreSQL. Use distinct React/TypeScript frontend, Go API, and PostgreSQL database layers.

## Prompt 2 — security requirements

Implement the numeric login code as a bcrypt hash in PostgreSQL and use a signed, HttpOnly, Secure session cookie after successful login. Keep the database credentials and session secret in environment variables and never expose them to the frontend.

## Prompt 3 — deployment

Provide a deployment plan using a Vercel-hosted React frontend, Render-hosted Go API, and Supabase-hosted PostgreSQL database. Include environment variables, CORS, HTTPS cookie requirements, database schema deployment, and GitHub collaborator setup.
