# Approved account flow and student dashboard

The owner approved the interactive prototype on 2026-10-31 and requested implementation through the student dashboard, including its spacing between components and modules.

- `/login` is the public student entry, with illustration left and form right. Public login and landing expose no internal portal link.
- `/portal` is direct staff entry, with form left and illustration right. Preserve role-based routing from authenticated server claims; a URL never grants a role. The old `/login/staff` address redirects to `/portal`.
- Registration keeps name, required email, optional phone and minimum-eight-character password. Success opens a six-digit email verification screen using the real API. Verification remains optional under D01; skip opens `/student`.
- Verification supports API errors, pending, resend, authenticated refresh, and already-verified accounts. Dashboard reminders link back to verification. No demo code or simulated credentials are included.
- Auth pages reuse the project's tokens, fonts, logo, controls and signature ring. Ring, caption and feature cards occupy separate layout regions. Motion is an entrance sequence and respects reduced-motion preferences.
- Apply the approved dashboard's spacious heading, four metrics, welcome panel, separate tryout/results modules and onboarding steps using the existing application shell. Real loaded accounts retain real activity; new accounts show zero counts, no score and their actual balance.
- Student dashboard reads only typed API data. Summary counts and average are server computed; recent results use persisted scores. Distinguish loading, failure/retry, empty and populated states.
- English and Indonesian, both themes, keyboard navigation and 320/390/768/1024/1440px are required checks.

This changes the P03 user interface and the P17 student overview, not scoring, purchase, access or verification policy. Existing authentication/authorization, account recovery and exam behavior stay authoritative. Do not modify the PRD, introduce tenant selection, or implement unrelated dashboard destinations.
