# Security Policy

## Supported versions

Security fixes are provided for the latest released version. Koffy is currently pre-1.0, so users should review release notes before upgrading.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Use GitHub's **Report a vulnerability** feature in the repository Security tab to submit a private report.

Include the affected version, reproduction steps, impact, and any suggested mitigation. Please avoid accessing data that does not belong to you and allow maintainers reasonable time to investigate before public disclosure.

## Deployment responsibilities

Koffy handles authentication, points, usage records, and optional payment callbacks. Operators are responsible for:

- keeping Casdoor, MySQL, Redis, LiteLLM, and container images patched;
- using unique, randomly generated production secrets;
- terminating TLS at a trusted reverse proxy;
- restricting Billing API, database, Redis, and internal Docker network access;
- protecting WeChat Pay private keys and third-party provider credentials;
- backing up and testing restoration of both Koffy and Casdoor data.
