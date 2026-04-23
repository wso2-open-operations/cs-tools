# Customer Portal Webapp

The Customer Portal Webapp is a modern React application built with TypeScript and Vite, designed to provide a seamless user experience for managing customer-related tasks and services.

## Tech Stack

- **Core**: [React 19](https://react.dev/)
- **Build Tool**: [Vite](https://vitejs.dev/)
- **Language**: [TypeScript](https://www.typescriptlang.org/)
- **UI Library**: [Oxygen UI](https://github.com/wso2/oxygen-ui)
- **Data Fetching**: [TanStack Query](https://tanstack.com/query/latest) (React Query)
- **Authentication**: [@asgardeo/auth-react](https://github.com/asgardeo/asgardeo-auth-react-sdk)
- **Testing**: [Vitest](https://vitest.dev/) & [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/)

## Getting Started

### Prerequisites

- [Node.js](https://nodejs.org/) (Latest LTS recommended)
- [pnpm](https://pnpm.io/) (Recommended package manager)

### Installation

1. Install dependencies:

   ```bash
   pnpm install
   ```

2. Copy the environment variables template:

   ```bash
   cp .env.example .env
   ```

3. Update the `.env` file with your specific configuration values.

### Development

Start the development server:

```bash
pnpm run dev
```

### Build

Build the application for production:

```bash
pnpm run build
```

Preview the production build:

```bash
pnpm run preview
```

## Configuration

The application uses environment variables for configuration. All variables are prefixed with `CUSTOMER_PORTAL_`.

### Environment Variables

| Variable                                     | Description                                                             | Example                          |
| -------------------------------------------- | ----------------------------------------------------------------------- | -------------------------------- |
| `CUSTOMER_PORTAL_AUTH_BASE_URL`              | Asgardeo/Auth base URL                                                  | `https://api.asgardeo.com/t/org` |
| `CUSTOMER_PORTAL_AUTH_CLIENT_ID`             | OAuth2 Client ID                                                        | `your_client_id`                 |
| `CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL`  | Sign-in callback URL                                                    | `http://localhost:3000`          |
| `CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL` | Sign-out callback URL                                                   | `http://localhost:3000`          |
| `CUSTOMER_PORTAL_BACKEND_BASE_URL`           | Backend API base URL                                                    | `https://api.example.com`        |
| `CUSTOMER_PORTAL_THEME`                      | Application theme (acrylicOrange, acrylicPurple, highContrast, classic) | `acrylicOrange`                  |
| `CUSTOMER_PORTAL_LOG_LEVEL`                  | Logging level (DEBUG, INFO, WARN, ERROR)                                | `INFO`                           |

### Import Aliases

The project uses the `@` alias to refer to the `src` directory, allowing for cleaner imports:

```typescript
import { someUtil } from "@/utils/someUtil";
```

## Logging

The app includes a custom logging service that respects the `CUSTOMER_PORTAL_LOG_LEVEL` environment variable. Access the logger via the `useLogger` hook:

```typescript
const logger = useLogger();
logger.info("Message to log");
```

## Testing

Run tests using Vitest:

```bash
pnpm run test
```
