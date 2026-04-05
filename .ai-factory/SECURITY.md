# Security: Ignored Items

Items below are excluded from security-checklist audits.
Review periodically — ignored risks may become relevant.

| Item | Reason | Date | Author |
|------|--------|------|--------|
| export-no-rate-limit | Export endpoint has auth+role check; rate limiting deferred to future milestone | 2026-04-05 | alexsh220me |
| export-token-in-url | ?token= in URL is intentional trade-off for browser <a download> links without JS; JWT has short TTL; nginx log masking deferred | 2026-04-05 | alexsh220me |
