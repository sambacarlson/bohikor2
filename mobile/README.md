# mobile/

Expo app for the employee salary-advance flow (login, signup, request an advance, history,
phone/PIN account settings).

## Status: frozen (Epic 9)

`bohikor/`'s `/{company}` routes are now the primary employee client as of the multi-tenant pivot
(see `PLAN.md`). This app is kept for reference and potential future use, but is **not** receiving
new features. Keep it green — `npm run lint`, `npm run typecheck`, and `npm run test` should
continue to pass — but new employee-facing work belongs in `bohikor/`, not here.

## Build & run

```bash
npm install
npx expo run:android   # or npx expo run:ios (requires prebuilt native dirs)
npm run lint
npm run typecheck
npm run test
```

Requires a dev client build (`npx expo run:android/ios`). No Firebase native modules needed.
