## Setting Up Environment Configuration

1. **Create a file named `config.js` inside the `public` folder in root directory.**
2. **Add the following code to `config.js`:**

   ```javascript
   window.config = {
     ASGARDEO_BASE_URL: "", // Asgardeo base URL
     CLIENT_ID: "", // Asgardeo client ID
     SIGN_IN_REDIRECT_URL: "", // Redirect URL after sign in
     SIGN_OUT_REDIRECT_URL: "", // Redirect URL after sign out
     BACKEND_BASE_URL: "", // Backend API base URL
     IS_MICROAPP: true,
   };
   ```

## Tests

`npm test` runs the unit tests (Vitest). They cover the rules that are kept free of React and of any component, such as
what the change request pages read of the customer's part of the process (`src/utils/changeRequestProgress.ts`).
