# Browser tests

They run against a stack that is already up, with the local sign-in:

```bash
# 1. Postgres, then the server (from the repo root)
TF_AUTH_DISABLED=true TF_SEED=true TF_INSECURE_COOKIES=true \
  TF_PG_HOST=localhost TF_PG_USER=postgres TF_PG_PASSWORD=... go run ./server/cmd/server

# 2. the UI
npm --prefix web run dev

# 3. the tests (first time: npx playwright install chromium)
npm --prefix web run e2e
```

`TF_E2E_URL` points the tests at another address. `a11y.spec.ts` runs axe-core on
the main screens in both themes and fails on serious or critical findings.
`states.spec.ts` breaks API answers on purpose to check the loading, error and
empty states.
`github.spec.ts` covers the Phase 8 screens with GitHub's answers replaced; it needs no GitHub.
`signup.spec.ts` covers the login page, the waiting page and the admin's approval list.
