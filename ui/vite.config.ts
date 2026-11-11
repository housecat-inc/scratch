import { defineConfig } from "vite-plus";

// Vitest for the kit's TypeScript, run through Vite+ (`vp test run`, or
// `make -C app test-ts`). No build or deploy step runs the tests; CI's ui-test
// job is where they are enforced. Pure helpers run in Node without a browser
// or DOM emulator.
export default defineConfig({
  test: {
    environment: "node",
    exclude: ["**/node_modules/**", "storybook/**"],
    include: ["**/*.test.ts"],
  },
});
