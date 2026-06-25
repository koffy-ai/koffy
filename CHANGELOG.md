# Changelog

All notable changes to Koffy are documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.1] - 2026-06-25

### Fixed

- Improved Alipay payment behavior from the recharge page: non-WeChat browsers now open Alipay in a separate page while keeping the Koffy recharge page available.
- Added a WeChat in-app browser notice for users choosing Alipay, and kept WeChat in-app Alipay fallback on the current page to avoid blank pages in the embedded browser.

## [0.2.0] - 2026-06-24

### Added

- Alipay recharge support for desktop web checkout and mobile H5 checkout, including async notify handling and local test-mode hooks.
- User-center payment method selection with WeChat Pay and Alipay brand icons.
- Independently uploadable and persistent user-center and admin favicons.
- Full production Compose stack for a blank Docker host.

### Upgrade Notes

- Existing v0.1.0 databases must run `migrations/003_add_alipay.sql` before creating Alipay recharge orders. Fresh installs already include the updated provider enum in `migrations/001_init.sql`.

### Changed

- Production Nginx and LiteLLM custom configuration now uses ignored runtime files copied from tracked examples.
- Compose examples use the stable LiteLLM `latest` image tag.

### Fixed

- Alipay recharge wallet ledger labels are now localized in the user center.
- Alipay callback times without timezone information are parsed as `Asia/Shanghai` to avoid UTC container timezone offsets.
- Alipay gateway errors are mapped to clearer user-facing messages for app status, product permission, and signing problems.

## [0.1.0] - 2026-06-18

### Added

- Self-hosted user center and administration console.
- Casdoor-backed authentication with phone/password and optional WeChat login.
- Wallets, points ledger, plans, subscriptions, and monthly entitlements.
- Recharge orders and optional WeChat Pay integration.
- OpenAI-compatible Koffy Gateway with application keys, rate limits, LiteLLM routing, usage pre-authorization, and settlement.
- Optional SMS verification and Tencent CAPTCHA integration.
- Docker Compose examples for local development and production deployment.
